package services

import (
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
	return &RecipeService{
		bridge:   bridge,
		embedded: embedded,
		userDir:  userDir,
		detector: detector,
		registry: registry,
		report:   report,
	}, nil
}

// List returns deterministic catalogue summaries with current availability.
func (service *RecipeService) List() ([]RecipeSummary, error) {
	manifests := service.listManifests()
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

func (service *RecipeService) listManifests() []recipe.Manifest {
	service.mu.RLock()
	manifests := service.registry.List()
	service.mu.RUnlock()
	return manifests
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
		SchemaVersion: manifest.SchemaVersion,
		ID:            manifest.ID,
		Name:          manifest.Name,
		Category:      manifest.Category,
		Description:   manifest.Description,
		DocsURL:       manifest.DocsURL,
		Tags:          cloneSlice(manifest.Tags),
		Icon:          manifest.Icon,
		VerifiedAt:    manifest.VerifiedAt,
		InstallPolicy: normalizedInstallPolicy(manifest),
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
