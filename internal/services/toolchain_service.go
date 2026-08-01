package services

import (
	"fmt"
	"sort"

	"nodesmith/internal/toolchain"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ToolchainService exposes executable discovery and PATH configuration.
type ToolchainService struct {
	bridge   *BridgeContext
	paths    *toolchain.PathResolver
	detector *toolchain.Detector
	store    *StoreService
}

func NewToolchainService(
	bridge *BridgeContext,
	paths *toolchain.PathResolver,
	detector *toolchain.Detector,
	storeService *StoreService,
) (*ToolchainService, error) {
	if bridge == nil || paths == nil || detector == nil || storeService == nil {
		return nil, fmt.Errorf("configure toolchain service: dependency is nil")
	}
	settings, err := storeService.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("configure toolchain service: %w", err)
	}
	if err := paths.SetOverride(settings.PathOverride); err != nil {
		return nil, fmt.Errorf("configure toolchain PATH override: %w", err)
	}
	if err := toolchain.SetPathOverride(settings.PathOverride); err != nil {
		return nil, fmt.Errorf("configure integration PATH override: %w", err)
	}
	return &ToolchainService{
		bridge:   bridge,
		paths:    paths,
		detector: detector,
		store:    storeService,
	}, nil
}

// Detect scans the allowlisted toolchain. A forced scan emits a change event.
func (service *ToolchainService) Detect(force bool) (Toolchain, error) {
	detected, err := service.detector.Detect(service.bridge.get(), force)
	if err != nil {
		return Toolchain{}, fmt.Errorf("detect local toolchain: %w", err)
	}
	result := toolchainDTO(detected, service.paths.DiscoveryWarning())
	if force {
		service.emitChanged(result)
	}
	return result, nil
}

// ResolvedPath returns the exact PATH used to find and execute tools.
func (service *ToolchainService) ResolvedPath() (string, error) {
	path, err := service.paths.ResolvedPath(service.bridge.get())
	if err != nil {
		return "", fmt.Errorf("resolve executable PATH: %w", err)
	}
	return path, nil
}

// SetPathOverride applies and persists a PATH override without an app restart.
func (service *ToolchainService) SetPathOverride(path string) error {
	if err := service.paths.SetOverride(path); err != nil {
		return fmt.Errorf("set PATH override: %w", err)
	}
	if err := toolchain.SetPathOverride(path); err != nil {
		return fmt.Errorf("set integration PATH override: %w", err)
	}
	settings, err := service.store.GetSettings()
	if err != nil {
		return fmt.Errorf("set PATH override: %w", err)
	}
	settings.PathOverride = path
	if err := service.store.SaveSettings(settings); err != nil {
		return fmt.Errorf("set PATH override: %w", err)
	}
	detected, err := service.detector.Detect(service.bridge.get(), true)
	if err != nil {
		return fmt.Errorf("rescan tools after PATH change: %w", err)
	}
	service.emitChanged(toolchainDTO(detected, service.paths.DiscoveryWarning()))
	return nil
}

func (service *ToolchainService) emitChanged(toolchain Toolchain) {
	if ctx, ready := service.bridge.ready(); ready {
		runtime.EventsEmit(ctx, "nodesmith:toolchain:changed", toolchain)
	}
}

func toolchainDTO(source toolchain.Toolchain, pathWarning string) Toolchain {
	names := make([]string, 0, len(source.Tools))
	for name := range source.Tools {
		names = append(names, name)
	}
	sort.Strings(names)
	tools := make([]Tool, 0, len(names))
	for _, name := range names {
		item := source.Tools[name]
		tools = append(tools, Tool{
			Name:    item.Name,
			Path:    item.Path,
			Version: item.Version,
			Present: item.Present,
			Error:   item.Error,
		})
	}
	return Toolchain{
		Path:        source.Path,
		DetectedAt:  source.DetectedAt,
		Tools:       tools,
		PathWarning: pathWarning,
	}
}
