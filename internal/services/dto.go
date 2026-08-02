package services

import "time"

// cloneSlice copies source into a slice that is never nil, so the JSON bridge
// always encodes an empty array instead of null.
func cloneSlice[T any](source []T) []T {
	result := make([]T, len(source))
	copy(result, source)
	return result
}

// clonePointer copies an optional scalar so a DTO never aliases registry state.
func clonePointer[T any](source *T) *T {
	if source == nil {
		return nil
	}
	value := *source
	return &value
}

// RecipeSummary is the catalogue representation of a recipe.
type RecipeSummary struct {
	ID                    string   `json:"id"`
	Name                  string   `json:"name"`
	Category              string   `json:"category"`
	Description           string   `json:"description"`
	DocsURL               string   `json:"docsUrl"`
	Tags                  []string `json:"tags"`
	Icon                  string   `json:"icon"`
	VerifiedAt            string   `json:"verifiedAt"`
	InstallPolicy         string   `json:"installPolicy"`
	MinimumReleaseAge     *int     `json:"minimumReleaseAge"`
	Available             bool     `json:"available"`
	UnavailableReasons    []string `json:"unavailableReasons"`
	DefaultPackageManager string   `json:"defaultPackageManager"`
}

// RecipeRequirements describes the tools a recipe needs.
type RecipeRequirements struct {
	Node            string   `json:"node"`
	PackageManagers []string `json:"packageManagers"`
	Tools           []string `json:"tools"`
}

// RecipeOption is one selectable manifest value.
type RecipeOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// RecipeField drives one input in the configuration form. The constraint fields
// mirror recipe.Field so the form can reject a bad answer before the bridge
// call; the planner enforces the same rules again and remains authoritative.
type RecipeField struct {
	ID        string         `json:"id"`
	Label     string         `json:"label"`
	Type      string         `json:"type"`
	Default   any            `json:"default"`
	Help      string         `json:"help"`
	Options   []RecipeOption `json:"options"`
	VisibleIf string         `json:"visibleIf"`
	Required  bool           `json:"required"`
	Pattern   string         `json:"pattern"`
	MinLength *int           `json:"minLength"`
	MaxLength *int           `json:"maxLength"`
	Min       *float64       `json:"min"`
	Max       *float64       `json:"max"`
}

// RecipeStep exposes a recipe step for recipe inspection and authoring tools.
type RecipeStep struct {
	ID    string            `json:"id"`
	Label string            `json:"label"`
	Bin   string            `json:"bin"`
	CWD   string            `json:"cwd"`
	Env   map[string]string `json:"env"`
	Args  []any             `json:"args"`
	When  string            `json:"when"`
}

// Recipe is the frontend-safe representation of a manifest.
type Recipe struct {
	SchemaVersion      int                `json:"schemaVersion"`
	ID                 string             `json:"id"`
	Name               string             `json:"name"`
	Category           string             `json:"category"`
	Description        string             `json:"description"`
	DocsURL            string             `json:"docsUrl"`
	Tags               []string           `json:"tags"`
	Icon               string             `json:"icon"`
	VerifiedAt         string             `json:"verifiedAt"`
	InstallPolicy      string             `json:"installPolicy"`
	MinimumReleaseAge  *int               `json:"minimumReleaseAge"`
	Requires           RecipeRequirements `json:"requires"`
	Fields             []RecipeField      `json:"fields"`
	Steps              []RecipeStep       `json:"steps"`
	Available          bool               `json:"available"`
	UnavailableReasons []string           `json:"unavailableReasons"`
}

// ReloadResult reports the outcome of rescanning user recipes.
type ReloadResult struct {
	Count     int      `json:"count"`
	Warnings  []string `json:"warnings"`
	Overrides []string `json:"overrides"`
}

// ValidationResult is returned by the recipe authoring validator.
type ValidationResult struct {
	Valid bool   `json:"valid"`
	Error string `json:"error"`
}

// Tool describes one executable detected on the effective PATH.
type Tool struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"`
	Present bool   `json:"present"`
	Error   string `json:"error"`
}

// Toolchain is a deterministic point-in-time tool scan.
type Toolchain struct {
	Path       string    `json:"path"`
	DetectedAt time.Time `json:"detectedAt"`
	Tools      []Tool    `json:"tools"`
	// PathWarning explains why the PATH above is the process PATH rather than
	// the login shell's. Empty when discovery succeeded or was not attempted.
	PathWarning string `json:"pathWarning"`
}

// ScaffoldRequest contains built-in and recipe-specific answers.
type ScaffoldRequest struct {
	RecipeID          string         `json:"recipeId"`
	ProjectName       string         `json:"projectName"`
	ParentDir         string         `json:"parentDir"`
	PackageManager    string         `json:"packageManager"`
	InstallDeps       bool           `json:"installDeps"`
	GitInit           bool           `json:"gitInit"`
	MinimumReleaseAge *int           `json:"minimumReleaseAge"`
	Answers           map[string]any `json:"answers"`
}

// Plan is the exact, reviewable execution plan.
type Plan struct {
	RecipeID   string     `json:"recipeId"`
	ProjectDir string     `json:"projectDir"`
	Steps      []PlanStep `json:"steps"`
	Warnings   []string   `json:"warnings"`
	Hash       string     `json:"hash"`
}

// PlanStep is one resolved command or structured project operation.
type PlanStep struct {
	ID      string            `json:"id"`
	Kind    string            `json:"kind"`
	Label   string            `json:"label"`
	Bin     string            `json:"bin"`
	Args    []string          `json:"args"`
	Dir     string            `json:"dir"`
	Env     map[string]string `json:"env"`
	Display string            `json:"display"`
	Config  *ProjectConfig    `json:"config"`
	Setup   *ProjectSetup     `json:"setup"`
}

// ProjectConfig is a reviewable structured edit to a package-manager file.
type ProjectConfig struct {
	Path    string `json:"path"`
	Format  string `json:"format"`
	Section string `json:"section"`
	Key     string `json:"key"`
	Value   string `json:"value"`
}

// ProjectSetup is a reviewable shared tooling configuration.
type ProjectSetup struct {
	RecipeID   string `json:"recipeId"`
	Template   string `json:"template"`
	Linting    string `json:"linting"`
	Formatting string `json:"formatting"`
	Styling    string `json:"styling"`
}

// Job is a stable scaffold-job snapshot.
type Job struct {
	ID         string    `json:"id"`
	State      string    `json:"state"`
	StepIndex  int       `json:"stepIndex"`
	StepCount  int       `json:"stepCount"`
	ExitCode   int       `json:"exitCode"`
	ProjectDir string    `json:"projectDir"`
	StartedAt  time.Time `json:"startedAt"`
	EndedAt    time.Time `json:"endedAt"`
	Error      string    `json:"error"`
}

// LogLine is one replayable process output line.
type LogLine struct {
	Seq    int    `json:"seq"`
	Stream string `json:"stream"`
	Text   string `json:"text"`
	StepID string `json:"stepId"`
}

// Settings are persisted locally as versioned JSON.
type Settings struct {
	DefaultParentDir string `json:"defaultParentDir"`
	PathOverride     string `json:"pathOverride"`
	Editor           string `json:"editor"`
	Theme            string `json:"theme"`
	OpenAfterCreate  bool   `json:"openAfterCreate"`
	// MinimumReleaseAge is the global package cooldown in minutes, or nil when
	// Nodesmith should leave every package manager on its own configuration.
	MinimumReleaseAge *int `json:"minimumReleaseAge"`
	// RecipeMinimumReleaseAge is retained only so version-one settings files
	// written by older builds can still be decoded. New builds clear it on save;
	// per-catalogue choices now live on ScaffoldRequest and in presets.
	RecipeMinimumReleaseAge map[string]int `json:"recipeMinimumReleaseAge,omitempty"`
}

// Sources of a resolved minimum release age, most specific first.
const (
	ReleaseAgeSourceRequest = "request"
	ReleaseAgeSourceRecipe  = "recipe"
	ReleaseAgeSourceGlobal  = "global"
	ReleaseAgeSourceUnset   = "unset"
)

// MinimumReleaseAgeResolution explains which layer supplied a recipe's cooldown
// so the interface can show the effective value and where it came from.
type MinimumReleaseAgeResolution struct {
	Minutes *int   `json:"minutes"`
	Source  string `json:"source"`
}

// Preset is a named, reusable scaffold request.
type Preset struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Request   ScaffoldRequest `json:"request"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

// HistoryEntry records one attempted project creation.
type HistoryEntry struct {
	ID             string    `json:"id"`
	RecipeID       string    `json:"recipeId"`
	RecipeName     string    `json:"recipeName"`
	ProjectName    string    `json:"projectName"`
	ProjectDir     string    `json:"projectDir"`
	PackageManager string    `json:"packageManager"`
	State          string    `json:"state"`
	PlanHash       string    `json:"planHash"`
	DurationMS     int64     `json:"durationMs"`
	CreatedAt      time.Time `json:"createdAt"`
	Error          string    `json:"error"`
}
