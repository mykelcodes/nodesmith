package recipe

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

type LoadReport struct {
	Overrides []string `json:"overrides"`
	Warnings  []string `json:"warnings"`
}

type Registry struct {
	byID map[string]Manifest
}

// Load reads recipes from an embedded filesystem rooted at the recipes
// directory, then applies valid user recipes in filename order. A user recipe
// replaces an embedded recipe with the same id. Invalid user files are skipped
// and reported; invalid embedded files fail the load.
func Load(embedded fs.FS, userDir string) (*Registry, LoadReport, error) {
	if embedded == nil {
		return nil, LoadReport{}, fmt.Errorf("load embedded recipes: filesystem is nil")
	}

	embeddedRecipes, err := loadRequiredFS(embedded)
	if err != nil {
		return nil, LoadReport{}, fmt.Errorf("load embedded recipes: %w", err)
	}

	registry := &Registry{byID: make(map[string]Manifest, len(embeddedRecipes))}
	embeddedPaths := make(map[string]string, len(embeddedRecipes))
	for _, loaded := range embeddedRecipes {
		if previous, duplicate := embeddedPaths[loaded.manifest.ID]; duplicate {
			return nil, LoadReport{}, fmt.Errorf(
				"load embedded recipes: %s: duplicate id %q (already defined by %s)",
				loaded.path,
				loaded.manifest.ID,
				previous,
			)
		}
		embeddedPaths[loaded.manifest.ID] = loaded.path
		registry.byID[loaded.manifest.ID] = loaded.manifest
	}

	report := LoadReport{}
	if userDir == "" {
		return registry, report, nil
	}
	userRecipes, warnings, err := loadOptionalUserDir(userDir)
	if err != nil {
		// An unreadable override directory must not take the whole catalogue
		// down with it. The embedded recipes are still valid, so report the
		// problem and carry on with them.
		report.Warnings = append(
			report.Warnings,
			fmt.Sprintf("%s: %v; user recipes skipped", userDir, err),
		)
		sort.Strings(report.Warnings)
		return registry, report, nil
	}
	report.Warnings = append(report.Warnings, warnings...)

	userPaths := make(map[string]string, len(userRecipes))
	for _, loaded := range userRecipes {
		if previous, duplicate := userPaths[loaded.manifest.ID]; duplicate {
			report.Warnings = append(report.Warnings, fmt.Sprintf(
				"%s: duplicate id %q (already defined by %s); skipped",
				loaded.path,
				loaded.manifest.ID,
				previous,
			))
			continue
		}
		userPaths[loaded.manifest.ID] = loaded.path
		if _, replaces := registry.byID[loaded.manifest.ID]; replaces {
			report.Overrides = append(report.Overrides, loaded.manifest.ID)
		}
		registry.byID[loaded.manifest.ID] = loaded.manifest
	}
	sort.Strings(report.Overrides)
	sort.Strings(report.Warnings)
	return registry, report, nil
}

func (registry *Registry) Len() int {
	if registry == nil {
		return 0
	}
	return len(registry.byID)
}

func (registry *Registry) List() ([]Manifest, error) {
	if registry == nil {
		return []Manifest{}, nil
	}
	ids := make([]string, 0, len(registry.byID))
	for id := range registry.byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	manifests := make([]Manifest, 0, len(ids))
	for _, id := range ids {
		manifest, err := cloneManifest(registry.byID[id])
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, manifest)
	}
	return manifests, nil
}

func (registry *Registry) Get(id string) (Manifest, error) {
	if registry == nil {
		return Manifest{}, fmt.Errorf("recipe %q not found", id)
	}
	manifest, exists := registry.byID[id]
	if !exists {
		return Manifest{}, fmt.Errorf("recipe %q not found", id)
	}
	return cloneManifest(manifest)
}

type loadedManifest struct {
	path     string
	manifest Manifest
}

func loadRequiredFS(source fs.FS) ([]loadedManifest, error) {
	names, err := fs.Glob(source, "*.json")
	if err != nil {
		return nil, fmt.Errorf("list recipe files: %w", err)
	}
	slices.Sort(names)

	loaded := make([]loadedManifest, 0, len(names))
	for _, name := range names {
		if strings.HasSuffix(name, ".schema.json") {
			continue
		}
		data, err := fs.ReadFile(source, name)
		if err != nil {
			return nil, fmt.Errorf("%s: read: %w", name, err)
		}
		manifest, err := DecodeAndValidate(strings.NewReader(string(data)))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		loaded = append(loaded, loadedManifest{path: name, manifest: manifest})
	}
	return loaded, nil
}

func loadOptionalUserDir(directory string) ([]loadedManifest, []string, error) {
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read directory %s: %w", directory, err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" || strings.HasSuffix(entry.Name(), ".schema.json") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	loaded := make([]loadedManifest, 0, len(names))
	var warnings []string
	for _, name := range names {
		path := filepath.Join(directory, name)
		data, err := os.ReadFile(path)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: read: %v; skipped", path, err))
			continue
		}
		manifest, err := DecodeAndValidate(strings.NewReader(string(data)))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v; skipped", path, err))
			continue
		}
		loaded = append(loaded, loadedManifest{path: path, manifest: manifest})
	}
	return loaded, warnings, nil
}

// cloneManifest returns a deep copy. It must never fall back to returning its
// argument: the registry hands these out to callers, and the returned value
// shares every slice and map with the stored manifest, so an aliased result
// would let one caller corrupt the registry for all later ones.
func cloneManifest(manifest Manifest) (Manifest, error) {
	data, err := json.Marshal(manifest)
	if err != nil {
		return Manifest{}, fmt.Errorf("encode recipe %q: %w", manifest.ID, err)
	}
	clone, err := DecodeBytes(data)
	if err != nil {
		return Manifest{}, fmt.Errorf("decode recipe %q: %w", manifest.ID, err)
	}
	return clone, nil
}
