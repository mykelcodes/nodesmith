package planner

const (
	StepKindCommand       = "command"
	StepKindProjectConfig = "project-config"

	ConfigFormatProperties = "properties"
	ConfigFormatTOML       = "toml"
	ConfigFormatYAML       = "yaml"
)

type ScaffoldRequest struct {
	RecipeID       string `json:"recipeId"`
	ProjectName    string `json:"projectName"`
	ParentDir      string `json:"parentDir"`
	PackageManager string `json:"packageManager"`
	InstallDeps    bool   `json:"installDeps"`
	GitInit        bool   `json:"gitInit"`
	// MinimumReleaseAge is the already-resolved package cooldown in minutes.
	// A nil pointer leaves every package manager on its own configuration;
	// zero explicitly disables a cooldown the package manager applies by
	// default.
	MinimumReleaseAge *int           `json:"minimumReleaseAge,omitempty"`
	Answers           map[string]any `json:"answers"`
}

type Plan struct {
	RecipeID   string     `json:"recipeId"`
	ProjectDir string     `json:"projectDir"`
	Steps      []PlanStep `json:"steps"`
	Warnings   []string   `json:"warnings"`
	Hash       string     `json:"hash"`
}

type PlanStep struct {
	ID      string            `json:"id"`
	Kind    string            `json:"kind,omitempty"`
	Label   string            `json:"label"`
	Bin     string            `json:"bin"`
	Args    []string          `json:"args"`
	Dir     string            `json:"dir"`
	Env     map[string]string `json:"env"`
	Display string            `json:"display"`
	Config  *ProjectConfig    `json:"config,omitempty"`
}

// ProjectConfig describes one safe, structured edit to a package-manager
// configuration file inside the generated project.
type ProjectConfig struct {
	Path    string `json:"path"`
	Format  string `json:"format"`
	Section string `json:"section,omitempty"`
	Key     string `json:"key"`
	Value   string `json:"value"`
}

type BinaryResolver interface {
	Resolve(name string) (string, error)
}

type commandResolver interface {
	ResolveCommand(name string) (string, []string, error)
}
