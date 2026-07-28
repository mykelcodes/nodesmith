package planner

type ScaffoldRequest struct {
	RecipeID       string         `json:"recipeId"`
	ProjectName    string         `json:"projectName"`
	ParentDir      string         `json:"parentDir"`
	PackageManager string         `json:"packageManager"`
	InstallDeps    bool           `json:"installDeps"`
	GitInit        bool           `json:"gitInit"`
	Answers        map[string]any `json:"answers"`
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
	Label   string            `json:"label"`
	Bin     string            `json:"bin"`
	Args    []string          `json:"args"`
	Dir     string            `json:"dir"`
	Env     map[string]string `json:"env"`
	Display string            `json:"display"`
}

type BinaryResolver interface {
	Resolve(name string) (string, error)
}

type commandResolver interface {
	ResolveCommand(name string) (string, []string, error)
}
