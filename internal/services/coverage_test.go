package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"nodesmith/internal/recipe"
	"nodesmith/internal/runner"
	"nodesmith/internal/toolchain"
)

// swapRecipeManifest replaces the service's registry with one built from raw.
//
// This models the situation the review boundary exists to catch: the resolved
// commands change between the moment the user reviewed them and the moment
// Start re-resolves. A user recipe edited on disk and reloaded, or a bundled
// recipe replaced by a user override, both produce it.
func swapRecipeManifest(t *testing.T, service *RecipeService, id string, raw []byte) {
	t.Helper()
	registry, _, err := recipe.Load(fstest.MapFS{id + ".json": {Data: raw}}, "")
	if err != nil {
		t.Fatalf("load replacement manifest: %v", err)
	}
	service.mu.Lock()
	service.registry = registry
	service.mu.Unlock()
}

func serviceTestManifest(args string) []byte {
	return []byte(`{
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
			"args": ` + args + `
		}]
	}`)
}

// The central safety property: what the user approved is what runs. An
// identical request whose resolved commands changed must be refused, and the
// refusal must be distinguishable from "never reviewed".
func TestScaffoldServiceRejectsAPlanThatChangedAfterReview(t *testing.T) {
	service, _ := newScaffoldServiceTestHarness(t)
	request := ScaffoldRequest{
		RecipeID:       "service-test",
		ProjectName:    "demo",
		ParentDir:      t.TempDir(),
		PackageManager: "npm",
		Answers:        map[string]any{},
	}

	reviewed, err := service.Plan(request)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	swapRecipeManifest(t, service.recipes, "service-test",
		serviceTestManifest(`["-test.run=TestScaffoldServiceProcess", "--extra-argument"]`))

	mutated, err := service.Plan(request)
	if err != nil {
		t.Fatalf("Plan() after swap error = %v", err)
	}
	if mutated.Hash == reviewed.Hash {
		t.Fatal("plan hash did not change after the recipe changed; the test proves nothing")
	}

	// Restore the review entry recorded against the original hash, so the
	// request key matches but the re-resolved hash does not.
	key, err := requestKey(normalizeForKey(t, service, request))
	if err != nil {
		t.Fatal(err)
	}
	service.mu.Lock()
	service.reviewed = map[string]reviewedPlan{key: {hash: reviewed.Hash}}
	service.mu.Unlock()

	_, err = service.Start(request)
	if err == nil || !strings.Contains(err.Error(), "changed after review") {
		t.Fatalf("Start() error = %v, want a hash-mismatch refusal", err)
	}
	// A mismatch must not be reported as a missing review: the user needs to
	// know the commands changed, not that they forgot a step.
	if strings.Contains(err.Error(), "review the resolved commands before starting") {
		t.Fatalf("Start() error = %v, want it distinguished from an unreviewed request", err)
	}
}

// normalizeForKey reproduces the normalisation Start applies before hashing the
// request, so a test can address the same review entry Start will look up.
func normalizeForKey(t *testing.T, service *ScaffoldService, request ScaffoldRequest) ScaffoldRequest {
	t.Helper()
	_, normalized, err := service.resolvePlan(request, false)
	if err != nil {
		t.Fatalf("resolvePlan() error = %v", err)
	}
	return normalized
}

func TestScaffoldServiceCancel(t *testing.T) {
	service, _ := newScaffoldServiceTestHarness(t)

	if err := service.Cancel("   "); err == nil || !strings.Contains(err.Error(), "job id is required") {
		t.Fatalf("Cancel(blank) error = %v, want a validation error", err)
	}
	if err := service.Cancel("does-not-exist"); err == nil {
		t.Fatal("Cancel(unknown) error = nil, want a not-found error")
	}
}

func TestScaffoldServiceStatusAndLogsRejectUnknownJobs(t *testing.T) {
	service, _ := newScaffoldServiceTestHarness(t)

	if _, err := service.Status("missing"); err == nil ||
		!strings.Contains(err.Error(), "load project status") {
		t.Fatalf("Status(unknown) error = %v", err)
	}
	if _, err := service.Logs("missing", 0); err == nil ||
		!strings.Contains(err.Error(), "load project output") {
		t.Fatalf("Logs(unknown) error = %v", err)
	}
}

// The bridge is not ready in tests, so these exercise the not-ready guard rather
// than the native dialog.
func TestScaffoldServicePickDirectoryRequiresTheDesktopRuntime(t *testing.T) {
	service, _ := newScaffoldServiceTestHarness(t)

	if _, err := service.PickDirectory(t.TempDir()); err == nil ||
		!strings.Contains(err.Error(), "desktop runtime is not ready") {
		t.Fatalf("PickDirectory() error = %v", err)
	}
}

func TestScaffoldServiceIntegrationsRejectInvalidInput(t *testing.T) {
	service, _ := newScaffoldServiceTestHarness(t)

	if err := service.OpenInEditor(t.TempDir(), "not-an-editor"); err == nil ||
		!strings.Contains(err.Error(), "open project in") {
		t.Fatalf("OpenInEditor() error = %v", err)
	}
	if err := service.RevealInFileManager(filepath.Join(t.TempDir(), "missing")); err == nil ||
		!strings.Contains(err.Error(), "reveal project") {
		t.Fatalf("RevealInFileManager() error = %v", err)
	}
}

// forwardRunnerEvent is the only writer of terminal history state. A payload
// that is not a DoneEvent must pass through untouched, and a history-write
// failure must not panic or block delivery.
func TestForwardRunnerEventUpdatesHistoryOnlyForDoneEvents(t *testing.T) {
	service, storeService := newScaffoldServiceTestHarness(t)

	entry := HistoryEntry{
		ID:          "job-1",
		RecipeID:    "service-test",
		RecipeName:  "Service test",
		ProjectName: "demo",
		ProjectDir:  t.TempDir(),
		State:       string(runner.StateRunning),
		CreatedAt:   time.Now().UTC(),
	}
	if err := storeService.recordHistory(entry); err != nil {
		t.Fatal(err)
	}

	// A log event carries no terminal state and must leave history alone.
	service.forwardRunnerEvent(runner.Event{
		Name:    "nodesmith:job:log",
		Payload: runner.LogEvent{JobID: "job-1", Seq: 0, Text: "hello"},
	})
	current := historyEntryByID(t, storeService, "job-1")
	if current.State != string(runner.StateRunning) {
		t.Fatalf("state = %q, want it unchanged by a log event", current.State)
	}

	service.forwardRunnerEvent(runner.Event{
		Name: "nodesmith:job:done",
		Payload: runner.DoneEvent{
			JobID:      "job-1",
			State:      runner.StateFailed,
			ExitCode:   7,
			DurationMS: 1234,
			Error:      "deliberate failure",
		},
	})
	current = historyEntryByID(t, storeService, "job-1")
	if current.State != string(runner.StateFailed) {
		t.Fatalf("state = %q, want failed", current.State)
	}
	if current.DurationMS != 1234 || current.Error != "deliberate failure" {
		t.Fatalf("entry = %#v, want the done event's duration and error recorded", current)
	}
}

// A done event for a job with no history row must be survivable: recordHistory
// is allowed to fail without cancelling the run, so the row may genuinely be
// absent.
func TestForwardRunnerEventToleratesAMissingHistoryRow(t *testing.T) {
	service, _ := newScaffoldServiceTestHarness(t)

	service.forwardRunnerEvent(runner.Event{
		Name: "nodesmith:job:done",
		Payload: runner.DoneEvent{
			JobID: "never-recorded",
			State: runner.StateSuccess,
		},
	})
}

func historyEntryByID(t *testing.T, service *StoreService, id string) HistoryEntry {
	t.Helper()
	entries, err := service.ListHistory(0)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.ID == id {
			return entry
		}
	}
	t.Fatalf("history entry %q not found", id)
	return HistoryEntry{}
}

func TestRecipeServiceListGetAndReload(t *testing.T) {
	service, _ := newScaffoldServiceTestHarness(t)
	recipes := service.recipes

	summaries, err := recipes.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(summaries) != 1 || summaries[0].ID != "service-test" {
		t.Fatalf("List() = %#v, want the single bundled test recipe", summaries)
	}

	full, err := recipes.Get("service-test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if full.ID != "service-test" || len(full.Steps) != 1 {
		t.Fatalf("Get() = %#v, want one step", full)
	}
	if _, err := recipes.Get("no-such-recipe"); err == nil {
		t.Fatal("Get(unknown) error = nil, want a not-found error")
	}

	result, err := recipes.Reload()
	if err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	if result.Count != 1 {
		t.Fatalf("Reload() = %#v, want one recipe", result)
	}
}

// Get returns copies. A caller mutating one must not be able to observe the
// change through a later Get.
func TestRecipeServiceGetReturnsIndependentCopies(t *testing.T) {
	service, _ := newScaffoldServiceTestHarness(t)

	first, err := service.recipes.Get("service-test")
	if err != nil {
		t.Fatal(err)
	}
	first.Steps[0].Label = "mutated"
	first.Tags[0] = "mutated"

	second, err := service.recipes.Get("service-test")
	if err != nil {
		t.Fatal(err)
	}
	if second.Steps[0].Label == "mutated" || second.Tags[0] == "mutated" {
		t.Fatalf("Get() = %#v, want a copy independent of an earlier caller", second)
	}
}

func TestRecipeServiceValidate(t *testing.T) {
	service, _ := newScaffoldServiceTestHarness(t)

	blank, err := service.recipes.Validate("   ")
	if err != nil {
		t.Fatal(err)
	}
	if blank.Valid || !strings.Contains(blank.Error, "empty") {
		t.Fatalf("Validate(blank) = %#v", blank)
	}

	broken, err := service.recipes.Validate(`{"schemaVersion": 1}`)
	if err != nil {
		t.Fatal(err)
	}
	if broken.Valid || broken.Error == "" {
		t.Fatalf("Validate(invalid) = %#v, want an explained rejection", broken)
	}

	good, err := service.recipes.Validate(string(serviceTestManifest(`["--yes"]`)))
	if err != nil {
		t.Fatal(err)
	}
	if !good.Valid || good.Error != "" {
		t.Fatalf("Validate(valid) = %#v", good)
	}
}

// A user recipe that fails to validate is skipped, and the reason must survive
// startup so the catalogue can show it without the user pressing Reload.
func TestRecipeServiceRetainsStartupWarningsForInvalidUserRecipes(t *testing.T) {
	userDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(userDir, "broken.json"),
		[]byte(`{"schemaVersion": 1, "id": "broken"}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	detector := toolchain.NewDetector(toolchain.NewResolver(toolchain.NewPathResolver()))
	service, err := NewRecipeService(
		NewBridgeContext(),
		fstest.MapFS{"service-test.json": {Data: serviceTestManifest(`["--yes"]`)}},
		userDir,
		detector,
	)
	if err != nil {
		t.Fatalf("NewRecipeService() error = %v", err)
	}

	service.mu.RLock()
	warnings := service.report.Warnings
	service.mu.RUnlock()
	if len(warnings) == 0 {
		t.Fatal("startup report has no warnings, want the invalid user recipe reported")
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "broken") {
		t.Fatalf("warnings = %#v, want the failing file named", warnings)
	}
}

func TestToolchainService(t *testing.T) {
	binDir := t.TempDir()
	paths := toolchain.NewPathResolver()
	if err := paths.SetOverride(binDir); err != nil {
		t.Fatal(err)
	}
	detector := toolchain.NewDetector(toolchain.NewResolver(paths))
	storeService, err := NewStoreService(nil, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewToolchainService(NewBridgeContext(), paths, detector, storeService)
	if err != nil {
		t.Fatalf("NewToolchainService() error = %v", err)
	}

	if _, err := NewToolchainService(nil, paths, detector, storeService); err == nil {
		t.Fatal("NewToolchainService(nil bridge) error = nil, want a dependency error")
	}

	detected, err := service.Detect(false)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if len(detected.Tools) == 0 {
		t.Fatal("Detect() returned no tools, want one entry per detected binary")
	}
	// Tools is JSON-marshalled across the bridge; a nil slice would arrive as
	// null rather than an empty array.
	if detected.Tools == nil {
		t.Fatal("Detect().Tools = nil, want a non-nil slice")
	}

	// The constructor applies the persisted override, which is empty here, so
	// the resolver is deliberately reset before this point.
	if err := service.SetPathOverride(binDir); err != nil {
		t.Fatalf("SetPathOverride() error = %v", err)
	}
	resolved, err := service.ResolvedPath()
	if err != nil {
		t.Fatalf("ResolvedPath() error = %v", err)
	}
	if resolved != binDir {
		t.Fatalf("ResolvedPath() = %q, want the override %q", resolved, binDir)
	}

	// An override is persisted so it survives a restart.
	settings, err := storeService.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.PathOverride != binDir {
		t.Fatalf("settings.PathOverride = %q, want %q", settings.PathOverride, binDir)
	}
	if err := service.SetPathOverride(string([]byte{0})); err == nil {
		t.Fatal("SetPathOverride(NUL) error = nil, want a rejection")
	}
}

func TestStoreServiceDeleteHistoryEntry(t *testing.T) {
	service, err := NewStoreService(nil, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b", "c"} {
		if err := service.recordHistory(HistoryEntry{
			ID:          id,
			RecipeID:    "service-test",
			ProjectName: id,
			ProjectDir:  t.TempDir(),
			State:       string(runner.StateSuccess),
			CreatedAt:   time.Now().UTC(),
		}); err != nil {
			t.Fatal(err)
		}
	}

	if err := service.DeleteHistoryEntry("  "); err == nil {
		t.Fatal("DeleteHistoryEntry(blank) error = nil, want a validation error")
	}
	if err := service.DeleteHistoryEntry("b"); err != nil {
		t.Fatalf("DeleteHistoryEntry() error = %v", err)
	}

	entries, err := service.ListHistory(0)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.ID == "b" {
			t.Fatalf("history = %#v, want %q removed", entries, "b")
		}
	}
	if len(entries) != 2 {
		t.Fatalf("len(history) = %d, want 2", len(entries))
	}
}

// History is trimmed at write time, not only on read, so the file itself cannot
// grow without bound.
func TestStoreServiceTrimsHistoryAtWriteTime(t *testing.T) {
	service, err := NewStoreService(nil, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC()
	total := maxHistoryEntries + 25
	for index := range total {
		if err := service.recordHistory(HistoryEntry{
			ID:          "job-" + strings.Repeat("x", index%3) + itoa(index),
			RecipeID:    "service-test",
			ProjectName: "demo",
			ProjectDir:  t.TempDir(),
			State:       string(runner.StateSuccess),
			CreatedAt:   base.Add(time.Duration(index) * time.Second),
		}); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := service.ListHistory(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != maxHistoryEntries {
		t.Fatalf("len(history) = %d, want the cap of %d", len(entries), maxHistoryEntries)
	}
	// The newest entries are the ones worth keeping.
	if entries[0].CreatedAt.Before(base.Add(time.Duration(total-maxHistoryEntries) * time.Second)) {
		t.Fatalf("history[0].CreatedAt = %v, want the newest entries retained", entries[0].CreatedAt)
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}

func TestBridgeContextReadiness(t *testing.T) {
	bridge := NewBridgeContext()

	if _, ready := bridge.ready(); ready {
		t.Fatal("ready() = true before Set, want false")
	}

	var calls int
	bridge.OnUIReady(func(context.Context) { calls++ })

	bridge.Set(context.Background())
	if _, ready := bridge.ready(); !ready {
		t.Fatal("ready() = false after Set, want true")
	}
	if calls != 0 {
		t.Fatalf("callback ran %d times before NotifyUIReady, want 0", calls)
	}

	bridge.NotifyUIReady()
	if calls != 1 {
		t.Fatalf("callback ran %d times, want exactly 1", calls)
	}

	// A callback registered after the UI is already up must still run, or a
	// service constructed late would never replay its startup report.
	var late int
	bridge.OnUIReady(func(context.Context) { late++ })
	if late != 1 {
		t.Fatalf("late callback ran %d times, want 1", late)
	}

	// Notifying twice must not replay callbacks.
	bridge.NotifyUIReady()
	if calls != 1 || late != 1 {
		t.Fatalf("callbacks ran again on a second notify: calls=%d late=%d", calls, late)
	}
}

func TestClonePointer(t *testing.T) {
	t.Parallel()

	if got := clonePointer[int](nil); got != nil {
		t.Fatalf("clonePointer(nil) = %v, want nil", got)
	}
	value := 42
	cloned := clonePointer(&value)
	if cloned == &value {
		t.Fatal("clonePointer returned the original pointer")
	}
	*cloned = 7
	if value != 42 {
		t.Fatalf("value = %d, want the original left untouched", value)
	}
}

func TestIsTerminalJob(t *testing.T) {
	t.Parallel()

	for _, state := range []runner.State{
		runner.StateSuccess,
		runner.StateFailed,
		runner.StateCancelled,
	} {
		if !isTerminalJob(runner.Job{State: state}) {
			t.Fatalf("isTerminalJob(%q) = false, want true", state)
		}
	}
	for _, state := range []runner.State{runner.StatePending, runner.StateRunning} {
		if isTerminalJob(runner.Job{State: state}) {
			t.Fatalf("isTerminalJob(%q) = true, want false", state)
		}
	}
}

// A clean startup has nothing to replay, so the bridge must not be touched.
// The emit path itself needs a live Wails runtime and is exercised manually.
func TestEmitLoadReportIsSilentWhenNothingWentWrong(t *testing.T) {
	service, _ := newScaffoldServiceTestHarness(t)

	service.recipes.mu.RLock()
	report := service.recipes.report
	service.recipes.mu.RUnlock()
	if len(report.Warnings) != 0 || len(report.Overrides) != 0 {
		t.Fatalf("report = %#v, want a clean startup for the test fixture", report)
	}

	// Returns before reaching runtime.EventsEmit, which would need a real
	// desktop context.
	service.recipes.emitLoadReport(context.Background())
}

func TestStoreServiceListPresetsOrdersByRecencyThenName(t *testing.T) {
	service, err := NewStoreService(nil, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	base := time.Now().UTC()
	for _, preset := range []struct {
		name    string
		updated time.Time
	}{
		{name: "beta", updated: base},
		{name: "Alpha", updated: base},
		{name: "newest", updated: base.Add(time.Hour)},
	} {
		if err := service.SavePreset(Preset{
			Name: preset.name,
			Request: ScaffoldRequest{
				RecipeID:       "service-test",
				ProjectName:    "demo",
				ParentDir:      t.TempDir(),
				PackageManager: "npm",
				Answers:        map[string]any{},
			},
		}); err != nil {
			t.Fatalf("SavePreset(%q) error = %v", preset.name, err)
		}
	}

	presets, err := service.ListPresets()
	if err != nil {
		t.Fatalf("ListPresets() error = %v", err)
	}
	if len(presets) != 3 {
		t.Fatalf("len(presets) = %d, want 3", len(presets))
	}
	// Most recently updated first; ties broken case-insensitively by name.
	if presets[0].Name != "newest" {
		t.Fatalf("presets[0].Name = %q, want the most recently saved", presets[0].Name)
	}
	if presets[1].Name != "Alpha" || presets[2].Name != "beta" {
		t.Fatalf("presets = %#v, want ties broken case-insensitively by name", presets)
	}

	// The returned slice is a copy of stored state.
	presets[0].Name = "mutated"
	again, err := service.ListPresets()
	if err != nil {
		t.Fatal(err)
	}
	if again[0].Name == "mutated" {
		t.Fatal("ListPresets returned aliased store state")
	}
}
