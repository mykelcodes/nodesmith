package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"nodesmith/internal/planner"
)

var toolingConfigFiles = []string{
	"eslint.config.js", "eslint.config.mjs", "eslint.config.cjs", "eslint.config.ts", "eslint.config.mts",
	".eslintrc", ".eslintrc.json", ".eslintrc.js", ".eslintrc.cjs", ".eslintrc.yaml", ".eslintrc.yml",
	".oxlintrc.json", ".oxlintrc.jsonc", "oxlint.config.ts", "oxlint.config.mts",
	"biome.json", "biome.jsonc",
	".prettierrc", ".prettierrc.json", ".prettierrc.json5", ".prettierrc.yaml", ".prettierrc.yml",
	".prettierrc.js", ".prettierrc.cjs", ".prettierrc.mjs", ".prettierrc.toml",
	"prettier.config.js", "prettier.config.cjs", "prettier.config.mjs",
	".oxfmtrc.json", ".oxfmtrc.jsonc", "oxfmt.config.ts", "oxfmt.config.mts",
}

var toolingPackages = []string{
	"eslint", "@eslint/js", "typescript-eslint", "@typescript-eslint/parser", "globals", "eslint-plugin-astro", "eslint-plugin-svelte",
	"oxlint", "@biomejs/biome", "prettier", "prettier-plugin-astro", "prettier-plugin-svelte", "oxfmt",
}

var stylingPackages = []string{
	"uniwind", "nativewind", "react-native-css", "react-native-unistyles", "react-native-nitro-modules",
	"tailwindcss", "@tailwindcss/postcss", "postcss",
}

const eslintConfig = `import eslint from '@eslint/js';
import globals from 'globals';
import tseslint from 'typescript-eslint';

export default [
  { ignores: ['node_modules/**', 'dist/**', 'build/**', 'coverage/**', '.expo/**', '.next/**'] },
  eslint.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ['**/*.{js,mjs,cjs,jsx,ts,mts,cts,tsx}'],
    languageOptions: {
      globals: { ...globals.browser, ...globals.node, ...globals.jest, ...globals.vitest, ...globals.mocha },
      parserOptions: { ecmaFeatures: { jsx: true } },
    },
  },
];
`

const astroESLintConfig = `import eslint from '@eslint/js';
import globals from 'globals';
import tseslint from 'typescript-eslint';
import astro from 'eslint-plugin-astro';

export default [
  { ignores: ['node_modules/**', 'dist/**', 'coverage/**', '.astro/**'] },
  eslint.configs.recommended,
  ...tseslint.configs.recommended,
  ...astro.configs['flat/recommended'],
  {
    files: ['**/*.{js,mjs,cjs,jsx,ts,mts,cts,tsx,astro}'],
    languageOptions: { globals: { ...globals.browser, ...globals.node, ...globals.jest, ...globals.vitest, ...globals.mocha } },
  },
];
`

const svelteESLintConfig = `import { defineConfig } from 'eslint/config';
import eslint from '@eslint/js';
import globals from 'globals';
import tseslint from 'typescript-eslint';
import svelte from 'eslint-plugin-svelte';

export default defineConfig([
  { ignores: ['node_modules/**', 'dist/**', 'build/**', 'coverage/**', '.svelte-kit/**'] },
  eslint.configs.recommended,
  tseslint.configs.recommended,
  svelte.configs.recommended,
  {
    languageOptions: { globals: { ...globals.browser, ...globals.node, ...globals.jest, ...globals.vitest, ...globals.mocha } },
  },
  {
    files: ['**/*.svelte', '**/*.svelte.{js,ts}'],
    languageOptions: {
      parserOptions: { parser: tseslint.parser, extraFileExtensions: ['.svelte'] },
    },
  },
]);
`

const oxlintConfig = `{
  "$schema": "./node_modules/oxlint/configuration_schema.json",
  "categories": {
    "correctness": "warn"
  }
}
`

const biomeConfig = `{
  "$schema": "https://biomejs.dev/schemas/2.5.6/schema.json",
  "formatter": {
    "enabled": false
  },
  "linter": {
    "enabled": true,
    "rules": {
      "recommended": true
    }
  }
}
`

const oxfmtConfig = `{
  "$schema": "./node_modules/oxfmt/configuration_schema.json"
}
`

const uniwindCSS = `@import 'tailwindcss';
@import 'uniwind';
`

const uniwindMetroConfig = `const { getDefaultConfig } = require('expo/metro-config');
const { withUniwindConfig } = require('uniwind/metro');

const config = getDefaultConfig(__dirname);

module.exports = withUniwindConfig(config, {
  cssEntryFile: './global.css',
});
`

const nativewindCSS = `@import "tailwindcss/theme.css" layer(theme);
@import "tailwindcss/preflight.css" layer(base);
@import "tailwindcss/utilities.css";
@import "nativewind/theme";
`

const nativewindMetroConfig = `const { getDefaultConfig } = require('expo/metro-config');
const { withNativewind } = require('nativewind/metro');

const config = getDefaultConfig(__dirname);

module.exports = withNativewind(config);
`

const nativewindPostCSSConfig = `export default {
  plugins: {
    '@tailwindcss/postcss': {},
  },
};
`

const unistylesBabelConfig = `module.exports = function (api) {
  api.cache(true);
  return {
    presets: ['babel-preset-expo'],
    plugins: [['react-native-unistyles/plugin', { root: '.' }]],
  };
};
`

func writeProjectSetup(step planner.PlanStep) error {
	if step.Setup == nil {
		return fmt.Errorf("project setup payload is missing")
	}
	root, err := validateSetupRoot(step.Dir)
	if err != nil {
		return err
	}
	packagePath := filepath.Join(root, "package.json")
	packageInfo, err := regularFileInfo(packagePath)
	if err != nil {
		return fmt.Errorf("open generated package.json: %w", err)
	}
	content, err := os.ReadFile(packagePath)
	if err != nil {
		return fmt.Errorf("read generated package.json: %w", err)
	}
	packageJSON, err := decodePackageJSON(content)
	if err != nil {
		return err
	}

	if step.Setup.Linting != "" || step.Setup.Formatting != "" {
		if err := configureNodeTooling(packageJSON, step.Setup); err != nil {
			return err
		}
		for _, name := range toolingConfigFiles {
			if preserveGeneratedToolingConfig(name, step.Setup) {
				continue
			}
			if err := removeSetupFile(root, name); err != nil {
				return err
			}
		}
		if err := writeToolingConfigs(root, step.Setup); err != nil {
			return err
		}
	}
	if step.Setup.Styling != "" {
		if err := configureExpoStyling(packageJSON, step.Setup); err != nil {
			return err
		}
		if err := writeExpoStylingFiles(root, step.Setup); err != nil {
			return err
		}
	}

	rendered, err := json.MarshalIndent(packageJSON, "", "  ")
	if err != nil {
		return fmt.Errorf("encode generated package.json: %w", err)
	}
	rendered = append(rendered, '\n')
	if err := writeSetupFile(root, "package.json", rendered, packageInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("write generated package.json: %w", err)
	}
	return nil
}

func validateSetupRoot(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("project setup directory is empty")
	}
	root := filepath.Clean(value)
	info, err := os.Lstat(root)
	if err != nil {
		return "", fmt.Errorf("open generated project directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("generated project directory must be a real directory")
	}
	return root, nil
}

func regularFileInfo(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s must be a regular file", filepath.Base(path))
	}
	return info, nil
}

func decodePackageJSON(content []byte) (map[string]any, error) {
	var value map[string]any
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode generated package.json: %w", err)
	}
	if value == nil {
		return nil, fmt.Errorf("decode generated package.json: root must be an object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decode generated package.json: multiple JSON values")
		}
		return nil, fmt.Errorf("decode generated package.json: %w", err)
	}
	return value, nil
}

func configureNodeTooling(packageJSON map[string]any, setup *planner.ProjectSetup) error {
	dependencies, err := packageObject(packageJSON, "dependencies")
	if err != nil {
		return err
	}
	devDependencies, err := packageObject(packageJSON, "devDependencies")
	if err != nil {
		return err
	}
	scripts, err := packageObject(packageJSON, "scripts")
	if err != nil {
		return err
	}
	for _, name := range toolingPackages {
		delete(dependencies, name)
		delete(devDependencies, name)
	}

	switch setup.Linting {
	case "eslint":
		devDependencies["eslint"] = "^9.39.4"
		devDependencies["@eslint/js"] = "^9.39.4"
		devDependencies["typescript-eslint"] = "^8.65.0"
		devDependencies["globals"] = "^17.8.0"
		scripts["lint"] = "eslint ."
		scripts["lint:fix"] = "eslint . --fix"
		if setup.RecipeID == "astro" {
			devDependencies["eslint-plugin-astro"] = "^1.3.1"
			devDependencies["@typescript-eslint/parser"] = "^8.65.0"
			scripts["lint"] = "eslint . --ext .astro"
			scripts["lint:fix"] = "eslint . --ext .astro --fix"
		}
		if isSvelteProject(setup) {
			devDependencies["eslint-plugin-svelte"] = "^3.22.0"
			scripts["lint"] = "eslint . --ext .svelte"
			scripts["lint:fix"] = "eslint . --ext .svelte --fix"
		}
	case "oxlint":
		devDependencies["oxlint"] = "^1.76.0"
		scripts["lint"] = "oxlint"
		scripts["lint:fix"] = "oxlint --fix"
	case "biome":
		devDependencies["@biomejs/biome"] = "2.5.6"
		scripts["lint"] = "biome lint ."
		scripts["lint:fix"] = "biome lint --write ."
	default:
		return fmt.Errorf("unsupported linting setup %q", setup.Linting)
	}

	switch setup.Formatting {
	case "prettier":
		devDependencies["prettier"] = "^3.9.6"
		if setup.RecipeID == "astro" {
			devDependencies["prettier-plugin-astro"] = "^0.14.1"
		}
		if isSvelteProject(setup) {
			version := "^4.1.1"
			if setup.RecipeID == "wails" {
				version = "^3.4.2"
			}
			devDependencies["prettier-plugin-svelte"] = version
		}
		scripts["format"] = "prettier . --write"
		scripts["format:check"] = "prettier . --check"
	case "oxfmt":
		devDependencies["oxfmt"] = "^0.61.0"
		scripts["format"] = "oxfmt"
		scripts["format:check"] = "oxfmt --check"
	default:
		return fmt.Errorf("unsupported formatting setup %q", setup.Formatting)
	}
	return nil
}

func packageObject(packageJSON map[string]any, key string) (map[string]any, error) {
	value, exists := packageJSON[key]
	if !exists {
		object := map[string]any{}
		packageJSON[key] = object
		return object, nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("generated package.json property %q must be an object", key)
	}
	return object, nil
}

func writeToolingConfigs(root string, setup *planner.ProjectSetup) error {
	var lintPath, lintContent string
	switch setup.Linting {
	case "eslint":
		lintPath, lintContent = "eslint.config.mjs", eslintConfigFor(setup)
	case "oxlint":
		lintPath, lintContent = ".oxlintrc.json", oxlintConfig
	case "biome":
		lintPath, lintContent = "biome.json", biomeConfig
	default:
		return fmt.Errorf("unsupported linting setup %q", setup.Linting)
	}
	preserveLint, err := matchingSetupFileExists(root, func(name string) bool {
		return setup.Linting == "eslint" && isFlatESLintConfig(name)
	})
	if err != nil {
		return err
	}
	if !preserveLint {
		if err := writeSetupFile(root, lintPath, []byte(lintContent), 0o644); err != nil {
			return err
		}
	}

	var formatPath, formatContent string
	switch setup.Formatting {
	case "prettier":
		formatPath, formatContent = ".prettierrc", prettierConfigFor(setup)
	case "oxfmt":
		formatPath, formatContent = ".oxfmtrc.json", oxfmtConfig
	default:
		return fmt.Errorf("unsupported formatting setup %q", setup.Formatting)
	}
	preserveFormat, err := matchingSetupFileExists(root, func(name string) bool {
		return setup.Formatting == "prettier" && isPrettierConfig(name)
	})
	if err != nil {
		return err
	}
	if preserveFormat {
		return nil
	}
	return writeSetupFile(root, formatPath, []byte(formatContent), 0o644)
}

func preserveGeneratedToolingConfig(name string, setup *planner.ProjectSetup) bool {
	return setup.Linting == "eslint" && isFlatESLintConfig(name) ||
		setup.Formatting == "prettier" && isPrettierConfig(name)
}

func isFlatESLintConfig(name string) bool {
	return strings.HasPrefix(name, "eslint.config.")
}

func isPrettierConfig(name string) bool {
	return strings.HasPrefix(name, ".prettierrc") || strings.HasPrefix(name, "prettier.config.")
}

func matchingSetupFileExists(root string, matches func(string) bool) (bool, error) {
	for _, name := range toolingConfigFiles {
		if !matches(name) {
			continue
		}
		path, err := setupPath(root, name)
		if err != nil {
			return false, err
		}
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return false, fmt.Errorf("inspect %s: %w", name, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return false, fmt.Errorf("refuse to use non-regular setup file %s", name)
		}
		return true, nil
	}
	return false, nil
}

func eslintConfigFor(setup *planner.ProjectSetup) string {
	if setup.RecipeID == "astro" {
		return astroESLintConfig
	}
	if isSvelteProject(setup) {
		return svelteESLintConfig
	}
	return eslintConfig
}

func prettierConfigFor(setup *planner.ProjectSetup) string {
	if setup.RecipeID == "astro" {
		return `{
  "plugins": ["prettier-plugin-astro"],
  "overrides": [
    {
      "files": "*.astro",
      "options": { "parser": "astro" }
    }
  ]
}
`
	}
	if isSvelteProject(setup) {
		return "{\n  \"plugins\": [\"prettier-plugin-svelte\"]\n}\n"
	}
	return "{}\n"
}

func isSvelteProject(setup *planner.ProjectSetup) bool {
	if setup.RecipeID == "svelte" || setup.RecipeID == "sveltekit" {
		return true
	}
	return strings.HasPrefix(setup.Template, "svelte")
}

func configureExpoStyling(packageJSON map[string]any, setup *planner.ProjectSetup) error {
	dependencies, err := packageObject(packageJSON, "dependencies")
	if err != nil {
		return err
	}
	devDependencies, err := packageObject(packageJSON, "devDependencies")
	if err != nil {
		return err
	}
	for _, name := range stylingPackages {
		delete(dependencies, name)
		delete(devDependencies, name)
	}

	switch setup.Styling {
	case "uniwind":
		dependencies["uniwind"] = "^1.9.0"
		devDependencies["tailwindcss"] = "^4.3.3"
	case "nativewind":
		dependencies["nativewind"] = "5.0.0-preview.4"
		dependencies["react-native-css"] = "^3.0.7"
		dependencies["react-native-reanimated"] = "~4.5.1"
		dependencies["react-native-safe-area-context"] = "~5.7.0"
		dependencies["react-native-worklets"] = "0.10.1"
		devDependencies["tailwindcss"] = "^4.3.3"
		devDependencies["@tailwindcss/postcss"] = "^4.3.3"
		devDependencies["postcss"] = "^8.5.6"
		overrides, err := packageObject(packageJSON, "overrides")
		if err != nil {
			return err
		}
		overrides["lightningcss"] = "1.30.1"
	case "unistyles":
		dependencies["react-native-unistyles"] = "^3.3.0"
		dependencies["react-native-nitro-modules"] = "0.36.5"
		devDependencies["babel-preset-expo"] = "~57.0.0"
	default:
		return fmt.Errorf("unsupported Expo styling setup %q", setup.Styling)
	}
	return nil
}

func writeExpoStylingFiles(root string, setup *planner.ProjectSetup) error {
	switch setup.Styling {
	case "uniwind":
		if err := removeSetupFile(root, "src/global.css"); err != nil {
			return err
		}
		if err := writeSetupFile(root, "global.css", []byte(uniwindCSS), 0o644); err != nil {
			return err
		}
		if err := writeSetupFile(root, "metro.config.js", []byte(uniwindMetroConfig), 0o644); err != nil {
			return err
		}
		return prependExpoCSSImport(root, setup.Template)
	case "nativewind":
		if err := removeSetupFile(root, "src/global.css"); err != nil {
			return err
		}
		if err := writeSetupFile(root, "global.css", []byte(nativewindCSS), 0o644); err != nil {
			return err
		}
		if err := writeSetupFile(root, "metro.config.js", []byte(nativewindMetroConfig), 0o644); err != nil {
			return err
		}
		if err := writeSetupFile(root, "postcss.config.mjs", []byte(nativewindPostCSSConfig), 0o644); err != nil {
			return err
		}
		return prependExpoCSSImport(root, setup.Template)
	case "unistyles":
		return writeSetupFile(root, "babel.config.js", []byte(unistylesBabelConfig), 0o644)
	default:
		return fmt.Errorf("unsupported Expo styling setup %q", setup.Styling)
	}
}

func prependExpoCSSImport(root, template string) error {
	var path, importLine string
	switch template {
	case "default":
		path, importLine = "src/app/_layout.tsx", "import '../../global.css';\n"
	case "tabs":
		path, importLine = "app/_layout.tsx", "import '../global.css';\n"
	case "blank":
		path, importLine = "App.js", "import './global.css';\n"
	case "blank-typescript":
		path, importLine = "App.tsx", "import './global.css';\n"
	case "bare-minimum":
		path, importLine = "App.js", "import './global.css';\n"
	default:
		return fmt.Errorf("unsupported Expo template %q", template)
	}
	fullPath := filepath.Join(root, filepath.FromSlash(path))
	if err := checkSetupParents(root, filepath.Dir(fullPath), false); err != nil {
		return err
	}
	info, err := regularFileInfo(fullPath)
	if err != nil {
		return fmt.Errorf("open Expo entry file %s: %w", path, err)
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("read Expo entry file %s: %w", path, err)
	}
	if bytes.Contains(content, []byte(strings.TrimSpace(importLine))) {
		return nil
	}
	updated := append([]byte(importLine), content...)
	return writeSetupFile(root, path, updated, info.Mode().Perm())
}

func removeSetupFile(root, relative string) error {
	path, err := setupPath(root, relative)
	if err != nil {
		return err
	}
	if err := checkSetupParents(root, filepath.Dir(path), false); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect %s: %w", relative, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("refuse to replace non-regular setup file %s", relative)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove replaced setup file %s: %w", relative, err)
	}
	return nil
}

func writeSetupFile(root, relative string, content []byte, defaultMode os.FileMode) error {
	path, err := setupPath(root, relative)
	if err != nil {
		return err
	}
	if err := ensureSetupParents(root, filepath.Dir(path)); err != nil {
		return err
	}
	mode := defaultMode
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("refuse to replace non-regular setup file %s", relative)
		}
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect setup file %s: %w", relative, err)
	}
	if mode == 0 {
		mode = 0o644
	}
	if runtime.GOOS == "windows" {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
		if err != nil {
			return fmt.Errorf("open setup file %s: %w", relative, err)
		}
		if _, err := file.Write(content); err != nil {
			_ = file.Close() // Preserve the more actionable write error.
			return fmt.Errorf("write setup file %s: %w", relative, err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close setup file %s: %w", relative, err)
		}
		return nil
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".nodesmith-setup-*")
	if err != nil {
		return fmt.Errorf("create temporary setup file %s: %w", relative, err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close() // Preserve the chmod error.
		return fmt.Errorf("set setup file mode %s: %w", relative, err)
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close() // Preserve the write error.
		return fmt.Errorf("write setup file %s: %w", relative, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close setup file %s: %w", relative, err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace setup file %s: %w", relative, err)
	}
	return nil
}

func setupPath(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", fmt.Errorf("setup file path %q must be relative", relative)
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("setup file path %q escapes the project", relative)
	}
	return filepath.Join(root, clean), nil
}

func ensureSetupParents(root, parent string) error {
	return checkSetupParents(root, parent, true)
}

func checkSetupParents(root, parent string, create bool) error {
	relative, err := filepath.Rel(root, parent)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("setup file parent escapes the project")
	}
	current := root
	if relative == "." {
		return nil
	}
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if !create {
				return nil
			}
			if err := os.Mkdir(current, 0o755); err != nil {
				return fmt.Errorf("create setup directory: %w", err)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect setup directory: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("setup directory must not contain symlinks")
		}
	}
	return nil
}
