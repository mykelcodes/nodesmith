package services

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"nodesmith/internal/runner"
	"nodesmith/internal/toolchain"
)

func TestScaffoldServiceProcess(t *testing.T) {
	if os.Getenv("GO_WANT_SCAFFOLD_SERVICE_PROCESS") != "1" {
		return
	}
	_, _ = fmt.Fprintln(os.Stdout, "project created")
}

func TestScaffoldServiceRequiresReviewAndRecordsSuccessfulRun(t *testing.T) {
	service, storeService := newScaffoldServiceTestHarness(t)
	request := ScaffoldRequest{
		RecipeID:       "service-test",
		ProjectName:    "demo",
		ParentDir:      t.TempDir(),
		PackageManager: "npm",
		Answers:        map[string]any{},
	}

	if _, err := service.Start(request); err == nil ||
		!strings.Contains(err.Error(), "review the resolved commands") {
		t.Fatalf("Start() without review error = %v", err)
	}

	plan, err := service.Plan(request)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(plan.Steps) != 1 || plan.Hash == "" {
		t.Fatalf("Plan() = %#v, want one hashed step", plan)
	}

	changed := request
	changed.ProjectName = "different"
	if _, err := service.Start(changed); err == nil ||
		!strings.Contains(err.Error(), "review the resolved commands") {
		t.Fatalf("Start() changed request error = %v", err)
	}

	job, err := service.Start(request)
	if err != nil {
		t.Fatalf("Start() reviewed request error = %v", err)
	}
	final := waitForServiceJob(t, service, job.ID)
	if final.State != string(runner.StateSuccess) {
		t.Fatalf("job = %#v, want success", final)
	}
	logs, err := service.Logs(job.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) == 0 || logs[0].Text != "project created" {
		t.Fatalf("Logs() = %#v, want helper output", logs)
	}

	history := waitForHistory(t, storeService, job.ID)
	if history.State != string(runner.StateSuccess) || history.PlanHash != plan.Hash {
		t.Fatalf("history = %#v, want successful reviewed plan", history)
	}

	if _, err := service.Start(request); err == nil ||
		!strings.Contains(err.Error(), "review the resolved commands") {
		t.Fatalf("reused review Start() error = %v", err)
	}
}

func TestScaffoldServiceKeepsOnlyLatestReviewedRequest(t *testing.T) {
	service, storeService := newScaffoldServiceTestHarness(t)
	first := ScaffoldRequest{
		RecipeID:       "service-test",
		ProjectName:    "first",
		ParentDir:      t.TempDir(),
		PackageManager: "npm",
		Answers:        map[string]any{},
	}
	second := first
	second.ProjectName = "second"

	if _, err := service.Plan(first); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Plan(second); err != nil {
		t.Fatal(err)
	}
	service.mu.Lock()
	reviewCount := len(service.reviewed)
	service.mu.Unlock()
	if reviewCount != 1 {
		t.Fatalf("reviewed request count = %d, want 1", reviewCount)
	}
	if _, err := service.Start(first); err == nil ||
		!strings.Contains(err.Error(), "review the resolved commands") {
		t.Fatalf("Start(first) error = %v, want stale review rejection", err)
	}
	job, err := service.Start(second)
	if err != nil {
		t.Fatalf("Start(second) error = %v", err)
	}
	if final := waitForServiceJob(t, service, job.ID); final.State != string(runner.StateSuccess) {
		t.Fatalf("second job = %#v, want success", final)
	}
	waitForHistory(t, storeService, job.ID)
}

func TestScaffoldServiceDoesNotSilentlyRenameWhitespace(t *testing.T) {
	service, _ := newScaffoldServiceTestHarness(t)
	_, err := service.Plan(ScaffoldRequest{
		RecipeID:    "service-test",
		ProjectName: " demo ",
		ParentDir:   t.TempDir(),
		Answers:     map[string]any{},
	})
	if err == nil || !strings.Contains(err.Error(), "leading or trailing whitespace") {
		t.Fatalf("Plan() whitespace error = %v", err)
	}
}

func TestScaffoldServiceEnforcesRecipeRequirementsForPlanAndStart(t *testing.T) {
	service, _ := newScaffoldServiceTestHarness(t)
	request := ScaffoldRequest{
		RecipeID:       "service-test",
		ProjectName:    "demo",
		ParentDir:      t.TempDir(),
		PackageManager: "npm",
		Answers:        map[string]any{},
	}
	service.detect = func(context.Context, bool) (toolchain.Toolchain, error) {
		return testServiceToolchain("18.20.4"), nil
	}
	if _, err := service.Plan(request); err == nil ||
		!strings.Contains(err.Error(), "does not satisfy required version") {
		t.Fatalf("Plan() unavailable toolchain error = %v", err)
	}

	service.detect = func(context.Context, bool) (toolchain.Toolchain, error) {
		return testServiceToolchain("24.0.0"), nil
	}
	if _, err := service.Plan(request); err != nil {
		t.Fatalf("Plan() with usable toolchain error = %v", err)
	}

	service.detect = func(context.Context, bool) (toolchain.Toolchain, error) {
		return testServiceToolchain("18.20.4"), nil
	}
	if _, err := service.Start(request); err == nil ||
		!strings.Contains(err.Error(), "does not satisfy required version") {
		t.Fatalf("Start() unavailable toolchain error = %v", err)
	}
}

func TestScaffoldServiceEnforcesSelectedPackageManager(t *testing.T) {
	service, _ := newScaffoldServiceTestHarness(t)
	service.detect = func(context.Context, bool) (toolchain.Toolchain, error) {
		detected := testServiceToolchain("24.0.0")
		detected.Tools["npm"] = toolchain.Tool{
			Name:    "npm",
			Present: true,
			Error:   "npm version probe failed",
		}
		return detected, nil
	}
	_, err := service.Plan(ScaffoldRequest{
		RecipeID:       "service-test",
		ProjectName:    "demo",
		ParentDir:      t.TempDir(),
		PackageManager: "npm",
		Answers:        map[string]any{},
	})
	if err == nil ||
		!strings.Contains(err.Error(), `package manager "npm" is unavailable`) ||
		!strings.Contains(err.Error(), "npm version probe failed") {
		t.Fatalf("Plan() selected package-manager error = %v", err)
	}
}

func newScaffoldServiceTestHarness(t *testing.T) (*ScaffoldService, *StoreService) {
	t.Helper()

	binDir := t.TempDir()
	executableName := "node"
	if runtime.GOOS == "windows" {
		executableName += ".exe"
	}
	copyCurrentExecutable(t, filepath.Join(binDir, executableName))

	paths := toolchain.NewPathResolver()
	if err := paths.SetOverride(binDir); err != nil {
		t.Fatal(err)
	}
	resolver := toolchain.NewResolver(paths)
	detector := toolchain.NewDetector(resolver)
	bridge := NewBridgeContext()
	manifest := []byte(`{
		"schemaVersion": 1,
		"id": "service-test",
		"name": "Service test",
		"category": "tooling",
		"description": "Exercises the scaffold service.",
		"docsUrl": "https://example.com/service-test",
		"tags": ["test"],
		"icon": "test",
			"verifiedAt": "2026-07-28",
			"requires": {
				"node": ">=20.0.0",
				"packageManagers": ["npm", "pnpm"],
				"tools": []
			},
		"fields": [],
		"steps": [{
			"id": "create",
			"label": "Create test project",
			"bin": "node",
			"cwd": "parentDir",
			"env": {"CI": "1", "GO_WANT_SCAFFOLD_SERVICE_PROCESS": "1"},
			"args": ["-test.run=TestScaffoldServiceProcess"]
		}]
	}`)
	recipeService, err := NewRecipeService(
		bridge,
		fstest.MapFS{"service-test.json": {Data: manifest}},
		"",
		detector,
	)
	if err != nil {
		t.Fatal(err)
	}
	storeService, err := NewStoreService(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manager := runner.NewManager(runner.WithPathProvider(func() (string, error) {
		return paths.ResolvedPath(context.Background())
	}))
	service, err := NewScaffoldService(
		bridge,
		recipeService,
		resolver,
		manager,
		storeService,
	)
	if err != nil {
		t.Fatal(err)
	}
	service.detect = func(context.Context, bool) (toolchain.Toolchain, error) {
		return testServiceToolchain("24.0.0"), nil
	}
	return service, storeService
}

func testServiceToolchain(nodeVersion string) toolchain.Toolchain {
	return toolchain.Toolchain{
		Tools: map[string]toolchain.Tool{
			"node": {
				Name:    "node",
				Present: true,
				Version: nodeVersion,
			},
			"npm": {
				Name:    "npm",
				Present: true,
				Version: "11.0.0",
			},
			"pnpm": {
				Name:    "pnpm",
				Present: true,
				Version: "10.0.0",
			},
		},
	}
}

func copyCurrentExecutable(t *testing.T, target string) {
	t.Helper()
	sourcePath, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = source.Close()
	}()

	destination, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(destination, source); err != nil {
		_ = destination.Close()
		t.Fatal(err)
	}
	if err := destination.Close(); err != nil {
		t.Fatal(err)
	}
}

func waitForServiceJob(t *testing.T, service *ScaffoldService, id string) Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job, err := service.Status(id)
		if err != nil {
			t.Fatal(err)
		}
		switch job.State {
		case string(runner.StateSuccess), string(runner.StateFailed), string(runner.StateCancelled):
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %q did not finish", id)
	return Job{}
}

func waitForHistory(t *testing.T, service *StoreService, id string) HistoryEntry {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		history, err := service.ListHistory(0)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range history {
			if entry.ID == id && entry.State == string(runner.StateSuccess) {
				return entry
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("history for job %q did not reach success", id)
	return HistoryEntry{}
}
