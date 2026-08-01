package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"

	"nodesmith/internal/project"
	"nodesmith/internal/recipe"
	"nodesmith/internal/toolchain"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// RecipeService owns the active embedded-plus-user recipe registry.
type RecipeService struct {
	bridge   *BridgeContext
	embedded fs.FS
	userDir  string
	detector *toolchain.Detector

	mu       sync.RWMutex
	registry *recipe.Registry
	report   recipe.LoadReport
}

func NewRecipeService(
	bridge *BridgeContext,
	embedded fs.FS,
	userDir string,
	detector *toolchain.Detector,
) (*RecipeService, error) {
	if bridge == nil || embedded == nil || detector == nil {
		return nil, fmt.Errorf("configure recipe service: dependency is nil")
	}
	registry, report, err := recipe.Load(embedded, userDir)
	if err != nil {
		return nil, fmt.Errorf("load bundled recipes: %w", err)
	}
	service := &RecipeService{
		bridge:   bridge,
		embedded: embedded,
		userDir:  userDir,
		detector: detector,
		registry: registry,
		report:   report,
	}
	// The startup load report is produced before any interface exists. Replay
	// it once the web view can receive events, otherwise a user recipe that
	// failed to validate is skipped in complete silence at launch and only
	// becomes visible if the user happens to press Reload.
	bridge.OnUIReady(service.emitLoadReport)
	return service, nil
}

func (service *RecipeService) emitLoadReport(ctx context.Context) {
	service.mu.RLock()
	report := service.report
	count := service.registry.Len()
	service.mu.RUnlock()

	if len(report.Warnings) == 0 && len(report.Overrides) == 0 {
		return
	}
	runtime.EventsEmit(ctx, "nodesmith:recipes:reloaded", ReloadResult{
		Count:     count,
		Warnings:  cloneSlice(report.Warnings),
		Overrides: cloneSlice(report.Overrides),
	})
}

// List returns deterministic catalogue summaries with current availability.
//
// The error return is reachable: Registry.List clones each manifest and reports
// a clone failure rather than handing out registry-aliased slices. Toolchain
// detection failures are not errors here — they are folded into each summary's
// UnavailableReasons so the catalogue still renders.
func (service *RecipeService) List() ([]RecipeSummary, error) {
	manifests, err := service.listManifests()
	if err != nil {
		return nil, err
	}
	detected, detectionErr := service.detector.Detect(service.bridge.get(), false)
	summaries := make([]RecipeSummary, 0, len(manifests))
	for _, manifest := range manifests {
		available, reasons, manager := recipeAvailability(manifest, detected, detectionErr)
		summaries = append(summaries, RecipeSummary{
			ID:                    manifest.ID,
			Name:                  manifest.Name,
			Category:              manifest.Category,
			Description:           manifest.Description,
			DocsURL:               manifest.DocsURL,
			Tags:                  cloneSlice(manifest.Tags),
			Icon:                  manifest.Icon,
			VerifiedAt:            manifest.VerifiedAt,
			InstallPolicy:         normalizedInstallPolicy(manifest),
			MinimumReleaseAge:     cloneMinutes(manifest.MinimumReleaseAge),
			Available:             available,
			UnavailableReasons:    reasons,
			DefaultPackageManager: manager,
		})
	}
	return summaries, nil
}

// Get returns a full data-driven recipe.
func (service *RecipeService) Get(id string) (Recipe, error) {
	manifest, err := service.getManifest(id)
	if err != nil {
		return Recipe{}, err
	}
	detected, detectionErr := service.detector.Detect(service.bridge.get(), false)
	available, reasons, _ := recipeAvailability(manifest, detected, detectionErr)
	result, err := recipeDTO(manifest)
	if err != nil {
		return Recipe{}, fmt.Errorf("prepare recipe %q for the interface: %w", id, err)
	}
	result.Available = available
	result.UnavailableReasons = reasons
	return result, nil
}

// Reload rescans the user recipe directory and atomically swaps the registry.
func (service *RecipeService) Reload() (ReloadResult, error) {
	registry, report, err := recipe.Load(service.embedded, service.userDir)
	if err != nil {
		return ReloadResult{}, fmt.Errorf("reload recipes: %w", err)
	}
	service.mu.Lock()
	service.registry = registry
	service.report = report
	service.mu.Unlock()
	result := ReloadResult{
		Count:     registry.Len(),
		Warnings:  cloneSlice(report.Warnings),
		Overrides: cloneSlice(report.Overrides),
	}
	if ctx, ready := service.bridge.ready(); ready {
		runtime.EventsEmit(ctx, "nodesmith:recipes:reloaded", result)
	}
	return result, nil
}

// Validate validates a raw user-recipe manifest without saving it.
func (service *RecipeService) Validate(raw string) (ValidationResult, error) {
	if strings.TrimSpace(raw) == "" {
		return ValidationResult{Valid: false, Error: "Recipe JSON is empty."}, nil
	}
	if _, err := recipe.DecodeAndValidate(strings.NewReader(raw)); err != nil {
		return ValidationResult{Valid: false, Error: err.Error()}, nil
	}
	return ValidationResult{Valid: true}, nil
}

// OpenRecipeDir reveals the local recipe override directory.
func (service *RecipeService) OpenRecipeDir() error {
	if err := os.MkdirAll(service.userDir, 0o700); err != nil {
		return fmt.Errorf("create local recipe directory: %w", err)
	}
	if err := project.RevealInFileManager(service.userDir); err != nil {
		return fmt.Errorf("open local recipe directory: %w", err)
	}
	return nil
}

func (service *RecipeService) listManifests() ([]recipe.Manifest, error) {
	service.mu.RLock()
	manifests, err := service.registry.List()
	service.mu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("load recipes: %w", err)
	}
	return manifests, nil
}

func (service *RecipeService) getManifest(id string) (recipe.Manifest, error) {
	service.mu.RLock()
	manifest, err := service.registry.Get(id)
	service.mu.RUnlock()
	if err != nil {
		return recipe.Manifest{}, fmt.Errorf("load recipe %q: %w", id, err)
	}
	return manifest, nil
}

func recipeAvailability(
	manifest recipe.Manifest,
	detected toolchain.Toolchain,
	detectionErr error,
) (bool, []string, string) {
	if detectionErr != nil {
		return false, []string{"Toolchain detection failed: " + detectionErr.Error()}, ""
	}
	gate := toolchain.EvaluateRequirements(detected, toolchain.Requirements{
		Node:            manifest.Requires.Node,
		PackageManagers: manifest.Requires.PackageManagers,
		Tools:           manifest.Requires.Tools,
	})
	return gate.Available, cloneSlice(gate.Reasons), gate.PackageManager
}

func recipeDTO(manifest recipe.Manifest) (Recipe, error) {
	fields := make([]RecipeField, 0, len(manifest.Fields))
	for _, field := range manifest.Fields {
		options := make([]RecipeOption, 0, len(field.Options))
		for _, option := range field.Options {
			options = append(options, RecipeOption{Value: option.Value, Label: option.Label})
		}
		fields = append(fields, RecipeField{
			ID:        field.ID,
			Label:     field.Label,
			Type:      string(field.Type),
			Default:   field.Default,
			Help:      field.Help,
			Options:   options,
			VisibleIf: field.VisibleIf,
			Required:  field.Required,
			Pattern:   field.Pattern,
			MinLength: clonePointer(field.MinLength),
			MaxLength: clonePointer(field.MaxLength),
			Min:       clonePointer(field.Min),
			Max:       clonePointer(field.Max),
		})
	}

	steps := make([]RecipeStep, 0, len(manifest.Steps))
	for _, step := range manifest.Steps {
		data, err := json.Marshal(step.Args)
		if err != nil {
			return Recipe{}, fmt.Errorf("encode step %q arguments: %w", step.ID, err)
		}
		var args []any
		if err := json.Unmarshal(data, &args); err != nil {
			return Recipe{}, fmt.Errorf("decode step %q arguments: %w", step.ID, err)
		}
		steps = append(steps, RecipeStep{
			ID:    step.ID,
			Label: step.Label,
			Bin:   step.Bin,
			CWD:   step.CWD,
			Env:   step.Env,
			Args:  args,
			When:  step.When,
		})
	}

	return Recipe{
		SchemaVersion:     manifest.SchemaVersion,
		ID:                manifest.ID,
		Name:              manifest.Name,
		Category:          manifest.Category,
		Description:       manifest.Description,
		DocsURL:           manifest.DocsURL,
		Tags:              cloneSlice(manifest.Tags),
		Icon:              manifest.Icon,
		VerifiedAt:        manifest.VerifiedAt,
		InstallPolicy:     normalizedInstallPolicy(manifest),
		MinimumReleaseAge: cloneMinutes(manifest.MinimumReleaseAge),
		Requires: RecipeRequirements{
			Node:            manifest.Requires.Node,
			PackageManagers: cloneSlice(manifest.Requires.PackageManagers),
			Tools:           cloneSlice(manifest.Requires.Tools),
		},
		Fields: fields,
		Steps:  steps,
	}, nil
}

func normalizedInstallPolicy(manifest recipe.Manifest) string {
	policy := manifest.InstallPolicy
	if policy == "" {
		policy = recipe.InstallOptional
	}
	return string(policy)
}
