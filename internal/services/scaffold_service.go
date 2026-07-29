package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"nodesmith/internal/planner"
	"nodesmith/internal/project"
	"nodesmith/internal/runner"
	"nodesmith/internal/toolchain"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type reviewedPlan struct {
	hash string
}

// ScaffoldService plans and executes scaffold jobs through the Wails bridge.
type ScaffoldService struct {
	bridge   *BridgeContext
	recipes  *RecipeService
	resolver *toolchain.Resolver
	jobs     *runner.Manager
	store    *StoreService
	detect   func(context.Context, bool) (toolchain.Toolchain, error)

	mu       sync.Mutex
	reviewed map[string]reviewedPlan
}

func NewScaffoldService(
	bridge *BridgeContext,
	recipes *RecipeService,
	resolver *toolchain.Resolver,
	jobs *runner.Manager,
	storeService *StoreService,
) (*ScaffoldService, error) {
	if bridge == nil || recipes == nil || resolver == nil || jobs == nil || storeService == nil {
		return nil, fmt.Errorf("configure scaffold service: dependency is nil")
	}
	service := &ScaffoldService{
		bridge:   bridge,
		recipes:  recipes,
		resolver: resolver,
		jobs:     jobs,
		store:    storeService,
		detect:   recipes.detector.Detect,
		reviewed: make(map[string]reviewedPlan),
	}
	jobs.Subscribe(service.forwardRunnerEvent)
	return service, nil
}

// Plan validates a request and returns the exact commands the runner would use.
func (service *ScaffoldService) Plan(request ScaffoldRequest) (Plan, error) {
	resolved, normalized, err := service.resolvePlan(request, false)
	if err != nil {
		return Plan{}, err
	}
	key, err := requestKey(normalized)
	if err != nil {
		return Plan{}, fmt.Errorf("remember reviewed plan: %w", err)
	}
	service.mu.Lock()
	clear(service.reviewed)
	service.reviewed[key] = reviewedPlan{hash: resolved.Hash}
	service.mu.Unlock()
	return planDTO(resolved), nil
}

// Start re-resolves a previously reviewed request and begins execution.
func (service *ScaffoldService) Start(request ScaffoldRequest) (Job, error) {
	resolved, normalized, err := service.resolvePlan(request, true)
	if err != nil {
		return Job{}, err
	}
	key, err := requestKey(normalized)
	if err != nil {
		return Job{}, fmt.Errorf("verify reviewed commands: %w", err)
	}
	service.mu.Lock()
	reviewed, exists := service.reviewed[key]
	if exists {
		delete(service.reviewed, key)
	}
	service.mu.Unlock()
	if !exists {
		return Job{}, errors.New("review the resolved commands before starting this project")
	}
	if reviewed.hash != resolved.Hash {
		return Job{}, errors.New("the resolved commands changed after review; review them again before running")
	}

	job, err := service.jobs.Start(resolved)
	if err != nil {
		return Job{}, fmt.Errorf("start project creation: %w", err)
	}
	manifest, manifestErr := service.recipes.getManifest(normalized.RecipeID)
	recipeName := normalized.RecipeID
	if manifestErr == nil {
		recipeName = manifest.Name
	}
	entry := HistoryEntry{
		ID:             job.ID,
		RecipeID:       normalized.RecipeID,
		RecipeName:     recipeName,
		ProjectName:    normalized.ProjectName,
		ProjectDir:     resolved.ProjectDir,
		PackageManager: normalized.PackageManager,
		State:          string(job.State),
		PlanHash:       resolved.Hash,
		CreatedAt:      job.StartedAt,
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = service.store.now().UTC()
	}
	if err := service.store.recordHistory(entry); err != nil {
		_ = service.jobs.Cancel(job.ID)
		return Job{}, fmt.Errorf("record project history before running: %w", err)
	}
	if current, statusErr := service.jobs.Status(job.ID); statusErr == nil && isTerminalJob(current) {
		_ = service.store.updateHistory(
			current.ID,
			string(current.State),
			current.EndedAt.Sub(current.StartedAt).Milliseconds(),
			current.Error,
		)
	}
	return jobDTO(job), nil
}

// Cancel requests cancellation of a running job.
func (service *ScaffoldService) Cancel(jobID string) error {
	if strings.TrimSpace(jobID) == "" {
		return errors.New("cancel project creation: job id is required")
	}
	if err := service.jobs.Cancel(jobID); err != nil {
		return fmt.Errorf("cancel project creation: %w", err)
	}
	return nil
}

// Status returns the latest job snapshot.
func (service *ScaffoldService) Status(jobID string) (Job, error) {
	job, err := service.jobs.Status(jobID)
	if err != nil {
		return Job{}, fmt.Errorf("load project status: %w", err)
	}
	return jobDTO(job), nil
}

// Logs returns replayable output from a sequence number.
func (service *ScaffoldService) Logs(jobID string, fromSeq int) ([]LogLine, error) {
	lines, err := service.jobs.Logs(jobID, fromSeq)
	if err != nil {
		return nil, fmt.Errorf("load project output: %w", err)
	}
	result := make([]LogLine, 0, len(lines))
	for _, line := range lines {
		result = append(result, LogLine{
			Seq:    line.Seq,
			Stream: line.Stream,
			Text:   line.Text,
			StepID: line.StepID,
		})
	}
	return result, nil
}

// PickDirectory opens the native Wails v2 directory chooser.
func (service *ScaffoldService) PickDirectory(startAt string) (string, error) {
	ctx, ready := service.bridge.ready()
	if !ready {
		return "", errors.New("open directory picker: desktop runtime is not ready")
	}
	selected, err := runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		DefaultDirectory:     startAt,
		Title:                "Choose a parent folder",
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", fmt.Errorf("open directory picker: %w", err)
	}
	return selected, nil
}

// OpenInEditor opens a completed project in a supported editor.
func (service *ScaffoldService) OpenInEditor(dir string, editor string) error {
	if err := project.OpenInEditor(dir, editor); err != nil {
		return fmt.Errorf("open project in %s: %w", editor, err)
	}
	return nil
}

// RevealInFileManager reveals a project using the native file manager.
func (service *ScaffoldService) RevealInFileManager(dir string) error {
	if err := project.RevealInFileManager(dir); err != nil {
		return fmt.Errorf("reveal project in the file manager: %w", err)
	}
	return nil
}

func (service *ScaffoldService) resolvePlan(
	request ScaffoldRequest,
	requireWritable bool,
) (planner.Plan, ScaffoldRequest, error) {
	request.RecipeID = strings.TrimSpace(request.RecipeID)
	request.PackageManager = strings.TrimSpace(request.PackageManager)
	if err := validateMinimumReleaseAgeRequest(request); err != nil {
		return planner.Plan{}, ScaffoldRequest{}, fmt.Errorf("configure project: %w", err)
	}
	if request.RecipeID == "" {
		return planner.Plan{}, ScaffoldRequest{}, errors.New("choose a recipe before continuing")
	}
	if request.Answers == nil {
		request.Answers = map[string]any{}
	}

	var (
		target project.Target
		err    error
	)
	if requireWritable {
		target, err = project.ValidateTarget(request.ParentDir, request.ProjectName, project.NameOptions{})
	} else {
		target, err = project.ResolveTarget(request.ParentDir, request.ProjectName, project.NameOptions{})
	}
	if err != nil {
		return planner.Plan{}, ScaffoldRequest{}, fmt.Errorf("check project destination: %w", err)
	}
	request.ParentDir = target.ParentDir

	manifest, err := service.recipes.getManifest(request.RecipeID)
	if err != nil {
		return planner.Plan{}, ScaffoldRequest{}, err
	}
	if len(manifest.Requires.PackageManagers) > 0 &&
		!slices.Contains(manifest.Requires.PackageManagers, request.PackageManager) {
		return planner.Plan{}, ScaffoldRequest{}, fmt.Errorf(
			"package manager %q is not supported by recipe %q",
			request.PackageManager,
			manifest.ID,
		)
	}
	detected, err := service.detect(service.bridge.get(), false)
	if err != nil {
		return planner.Plan{}, ScaffoldRequest{}, fmt.Errorf(
			"check local tools for %s: %w",
			manifest.Name,
			err,
		)
	}
	gate := toolchain.EvaluateRequirements(detected, toolchain.Requirements{
		Node:            manifest.Requires.Node,
		PackageManagers: manifest.Requires.PackageManagers,
		Tools:           manifest.Requires.Tools,
	})
	if !gate.Available {
		return planner.Plan{}, ScaffoldRequest{}, fmt.Errorf(
			"%s is unavailable: %s",
			manifest.Name,
			strings.Join(gate.Reasons, "; "),
		)
	}
	if len(manifest.Requires.PackageManagers) > 0 {
		selectedManager := toolchain.EvaluateRequirements(detected, toolchain.Requirements{
			PackageManagers: []string{request.PackageManager},
		})
		if !selectedManager.Available {
			return planner.Plan{}, ScaffoldRequest{}, fmt.Errorf(
				"package manager %q is unavailable: %s",
				request.PackageManager,
				strings.Join(selectedManager.Reasons, "; "),
			)
		}
	}
	settings, err := service.store.GetSettings()
	if err != nil {
		return planner.Plan{}, ScaffoldRequest{}, fmt.Errorf(
			"read the minimum release age for %s: %w",
			manifest.Name,
			err,
		)
	}
	releaseAge := resolveMinimumReleaseAge(request.MinimumReleaseAge, settings, manifest)
	resolved, err := planner.Resolve(manifest, planner.ScaffoldRequest{
		RecipeID:          request.RecipeID,
		ProjectName:       request.ProjectName,
		ParentDir:         request.ParentDir,
		PackageManager:    request.PackageManager,
		InstallDeps:       request.InstallDeps,
		GitInit:           request.GitInit,
		MinimumReleaseAge: releaseAge.Minutes,
		Answers:           request.Answers,
	}, service.resolver)
	if err != nil {
		return planner.Plan{}, ScaffoldRequest{}, fmt.Errorf("resolve commands for %s: %w", manifest.Name, err)
	}
	if filepath.Clean(resolved.ProjectDir) != filepath.Clean(target.ProjectDir) {
		return planner.Plan{}, ScaffoldRequest{}, errors.New("resolved project path does not match the validated destination")
	}
	return resolved, request, nil
}

func (service *ScaffoldService) forwardRunnerEvent(event runner.Event) {
	if ctx, ready := service.bridge.ready(); ready {
		runtime.EventsEmit(ctx, event.Name, event.Payload)
	}
	done, ok := event.Payload.(runner.DoneEvent)
	if !ok {
		return
	}
	if err := service.store.updateHistory(
		done.JobID,
		string(done.State),
		done.DurationMS,
		done.Error,
	); err != nil {
		if ctx, ready := service.bridge.ready(); ready {
			runtime.LogErrorf(ctx, "update Nodesmith history: %v", err)
		}
	}
}

func requestKey(request ScaffoldRequest) (string, error) {
	encoded, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:]), nil
}

func planDTO(source planner.Plan) Plan {
	steps := make([]PlanStep, 0, len(source.Steps))
	for _, step := range source.Steps {
		kind := step.Kind
		if kind == "" {
			kind = planner.StepKindCommand
		}
		var config *ProjectConfig
		if step.Config != nil {
			config = &ProjectConfig{
				Path:    step.Config.Path,
				Format:  step.Config.Format,
				Section: step.Config.Section,
				Key:     step.Config.Key,
				Value:   step.Config.Value,
			}
		}
		steps = append(steps, PlanStep{
			ID:      step.ID,
			Kind:    kind,
			Label:   step.Label,
			Bin:     step.Bin,
			Args:    cloneSlice(step.Args),
			Dir:     step.Dir,
			Env:     step.Env,
			Display: step.Display,
			Config:  config,
		})
	}
	return Plan{
		RecipeID:   source.RecipeID,
		ProjectDir: source.ProjectDir,
		Steps:      steps,
		Warnings:   cloneSlice(source.Warnings),
		Hash:       source.Hash,
	}
}

func jobDTO(source runner.Job) Job {
	return Job{
		ID:         source.ID,
		State:      string(source.State),
		StepIndex:  source.StepIndex,
		StepCount:  source.StepCount,
		ExitCode:   source.ExitCode,
		ProjectDir: source.ProjectDir,
		StartedAt:  source.StartedAt,
		EndedAt:    source.EndedAt,
		Error:      source.Error,
	}
}

func isTerminalJob(job runner.Job) bool {
	return job.State == runner.StateSuccess ||
		job.State == runner.StateFailed ||
		job.State == runner.StateCancelled
}
