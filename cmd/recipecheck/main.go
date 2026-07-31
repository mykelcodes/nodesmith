// Command recipecheck performs an end-to-end verification of one bundled recipe.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"nodesmith/internal/planner"
	"nodesmith/internal/recipe"
	"nodesmith/internal/runner"
	"nodesmith/internal/toolchain"
)

const (
	defaultScaffoldTimeout = 25 * time.Minute
	defaultSmokeTimeout    = 25 * time.Minute
	shutdownTimeout        = 30 * time.Second
)

type config struct {
	manifestPath   string
	expectedID     string
	projectName    string
	packageManager string
	tempRoot       string
	smokeJSON      string
	scaffoldLimit  time.Duration
	smokeLimit     time.Duration
	dryRun         bool
	keep           bool
}

type smokeSpec struct {
	ID    string            `json:"id,omitempty"`
	Label string            `json:"label"`
	Bin   string            `json:"bin"`
	Args  []string          `json:"args"`
	CWD   string            `json:"cwd,omitempty"`
	Env   map[string]string `json:"env,omitempty"`
}

type commandResolver interface {
	ResolveCommand(name string) (string, []string, error)
}

func main() {
	logger := log.New(os.Stdout, "[recipecheck] ", log.LstdFlags|log.Lmicroseconds)
	if err := run(context.Background(), os.Args[1:], logger); err != nil {
		log.New(os.Stderr, "[recipecheck] ERROR: ", 0).Print(err)
		os.Exit(1)
	}
}

func run(parent context.Context, args []string, logger *log.Logger) error {
	options, err := parseFlags(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	manifestBytes, err := os.ReadFile(options.manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest %q: %w", options.manifestPath, err)
	}
	manifest, err := recipe.DecodeAndValidate(bytes.NewReader(manifestBytes))
	if err != nil {
		return fmt.Errorf("decode manifest %q: %w", options.manifestPath, err)
	}
	if options.expectedID != "" && manifest.ID != options.expectedID {
		return fmt.Errorf(
			"manifest id is %q, expected %q",
			manifest.ID,
			options.expectedID,
		)
	}
	if options.projectName == "" {
		options.projectName = "nodesmith-check-" + manifest.ID
	}

	if options.packageManager == "" {
		if len(manifest.Requires.PackageManagers) == 0 {
			return fmt.Errorf("recipe %q declares no package manager", manifest.ID)
		}
		options.packageManager = manifest.Requires.PackageManagers[0]
	}
	if !slices.Contains(manifest.Requires.PackageManagers, options.packageManager) {
		return fmt.Errorf(
			"package manager %q is not supported by recipe %q",
			options.packageManager,
			manifest.ID,
		)
	}

	workspace, err := os.MkdirTemp(options.tempRoot, "nodesmith-recipecheck-"+manifest.ID+"-")
	if err != nil {
		return fmt.Errorf("create temporary workspace: %w", err)
	}
	if options.keep {
		logger.Printf("workspace will be preserved: %s", workspace)
	} else {
		defer func() {
			if removeErr := os.RemoveAll(workspace); removeErr != nil {
				logger.Printf("cleanup warning for %s: %v", workspace, removeErr)
				return
			}
			logger.Printf("removed temporary workspace: %s", workspace)
		}()
	}

	pathResolver := toolchain.NewPathResolver()
	if err := pathResolver.SetOverride(os.Getenv("PATH")); err != nil {
		return fmt.Errorf("use process PATH: %w", err)
	}
	resolver := toolchain.NewResolver(pathResolver)
	if err := verifyRequirements(parent, logger, manifest, options.packageManager, resolver); err != nil {
		return err
	}

	scaffoldPlan, err := planner.Resolve(manifest, planner.ScaffoldRequest{
		RecipeID:       manifest.ID,
		ProjectName:    options.projectName,
		ParentDir:      workspace,
		PackageManager: options.packageManager,
		InstallDeps:    true,
		GitInit:        false,
		Answers:        map[string]any{},
	}, resolver)
	if err != nil {
		return fmt.Errorf("resolve recipe plan: %w", err)
	}
	logPlan(logger, "scaffold/install", scaffoldPlan)

	smokeSpecs, err := parseSmokeSpecs(options.smokeJSON)
	if err != nil {
		return fmt.Errorf("parse smoke commands: %w", err)
	}
	smokePlan, err := buildSmokePlan(manifest.ID, scaffoldPlan.ProjectDir, smokeSpecs, resolver)
	if err != nil {
		return fmt.Errorf("resolve smoke plan: %w", err)
	}
	logPlan(logger, "build smoke", smokePlan)

	if options.dryRun {
		logger.Printf("dry run complete; no process was executed")
		return nil
	}

	logger.Printf("executing recipe %s in %s", manifest.ID, workspace)
	if err := executePlan(
		parent,
		logger,
		"scaffold/install",
		scaffoldPlan,
		options.scaffoldLimit,
		pathResolver,
	); err != nil {
		return err
	}
	projectInfo, err := os.Stat(scaffoldPlan.ProjectDir)
	if err != nil {
		return fmt.Errorf("stat generated project: %w", err)
	}
	if !projectInfo.IsDir() {
		return fmt.Errorf("generated project path is not a directory: %s", scaffoldPlan.ProjectDir)
	}

	if len(smokePlan.Steps) == 0 {
		logger.Printf("no build smoke command was configured")
		return nil
	}
	if err := executePlan(
		parent,
		logger,
		"build smoke",
		smokePlan,
		options.smokeLimit,
		pathResolver,
	); err != nil {
		return err
	}

	logger.Printf("recipe %s passed scaffold, install, and build smoke verification", manifest.ID)
	return nil
}

func parseFlags(args []string) (config, error) {
	var options config
	flags := flag.NewFlagSet("recipecheck", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&options.manifestPath, "manifest", "", "path to one bundled recipe manifest")
	flags.StringVar(&options.expectedID, "recipe", "", "expected recipe id")
	flags.StringVar(&options.projectName, "project-name", "", "generated project name")
	flags.StringVar(&options.packageManager, "package-manager", "", "supported package manager")
	flags.StringVar(&options.tempRoot, "temp-root", "", "directory in which to create a temporary workspace")
	smokeDefault := os.Getenv("RECIPECHECK_SMOKE_JSON")
	if smokeDefault == "" {
		smokeDefault = "[]"
	}
	flags.StringVar(
		&options.smokeJSON,
		"smoke-json",
		smokeDefault,
		"strict JSON array of build smoke commands",
	)
	flags.DurationVar(
		&options.scaffoldLimit,
		"scaffold-timeout",
		defaultScaffoldTimeout,
		"maximum scaffold and install duration",
	)
	flags.DurationVar(
		&options.smokeLimit,
		"smoke-timeout",
		defaultSmokeTimeout,
		"maximum build smoke duration",
	)
	flags.BoolVar(&options.dryRun, "dry-run", false, "resolve and print commands without executing them")
	flags.BoolVar(&options.keep, "keep", false, "preserve the temporary workspace")
	if err := flags.Parse(args); err != nil {
		return config{}, err
	}
	if flags.NArg() != 0 {
		return config{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	if strings.TrimSpace(options.manifestPath) == "" {
		return config{}, errors.New("-manifest is required")
	}
	if options.scaffoldLimit <= 0 {
		return config{}, errors.New("-scaffold-timeout must be positive")
	}
	if options.smokeLimit <= 0 {
		return config{}, errors.New("-smoke-timeout must be positive")
	}
	if options.tempRoot != "" {
		info, err := os.Stat(options.tempRoot)
		if err != nil {
			return config{}, fmt.Errorf("stat -temp-root: %w", err)
		}
		if !info.IsDir() {
			return config{}, errors.New("-temp-root must name a directory")
		}
	}
	return options, nil
}

func verifyRequirements(
	parent context.Context,
	logger *log.Logger,
	manifest recipe.Manifest,
	packageManager string,
	resolver *toolchain.Resolver,
) error {
	detectContext, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	detected, err := toolchain.NewDetector(resolver).Detect(detectContext, true)
	if err != nil {
		return fmt.Errorf("detect toolchain: %w", err)
	}
	result := toolchain.EvaluateRequirements(detected, toolchain.Requirements{
		Node:            manifest.Requires.Node,
		PackageManagers: []string{packageManager},
		Tools:           manifest.Requires.Tools,
	})
	if !result.Available {
		return fmt.Errorf(
			"toolchain does not satisfy recipe %q: %s",
			manifest.ID,
			strings.Join(result.Reasons, "; "),
		)
	}

	names := append([]string{"node", packageManager}, manifest.Requires.Tools...)
	slices.Sort(names)
	names = slices.Compact(names)
	for _, name := range names {
		tool, exists := detected.Lookup(name)
		if !exists {
			continue
		}
		logger.Printf("tool %-6s version=%s path=%s", name, tool.Version, tool.Path)
	}
	return nil
}

func parseSmokeSpecs(raw string) ([]smokeSpec, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var specs []smokeSpec
	if err := decoder.Decode(&specs); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("unexpected data after smoke command array")
		}
		return nil, fmt.Errorf("read trailing smoke command data: %w", err)
	}

	seen := make(map[string]struct{}, len(specs))
	for index := range specs {
		spec := &specs[index]
		if strings.TrimSpace(spec.ID) == "" {
			spec.ID = fmt.Sprintf("smoke-%d", index+1)
		}
		if _, duplicate := seen[spec.ID]; duplicate {
			return nil, fmt.Errorf("duplicate smoke command id %q", spec.ID)
		}
		seen[spec.ID] = struct{}{}
		if strings.TrimSpace(spec.Label) == "" {
			return nil, fmt.Errorf("%s: label is required", spec.ID)
		}
		if strings.TrimSpace(spec.Bin) == "" {
			return nil, fmt.Errorf("%s: bin is required", spec.ID)
		}
		if spec.CWD == "" {
			spec.CWD = "projectDir"
		}
		if spec.CWD != "projectDir" && spec.CWD != "parentDir" {
			return nil, fmt.Errorf("%s: cwd must be projectDir or parentDir", spec.ID)
		}
		for key, value := range spec.Env {
			if key == "" || strings.ContainsAny(key, "=\x00") {
				return nil, fmt.Errorf("%s: invalid environment key %q", spec.ID, key)
			}
			if strings.ContainsRune(value, '\x00') {
				return nil, fmt.Errorf("%s: environment value %q contains a NUL byte", spec.ID, key)
			}
		}
		for argumentIndex, argument := range spec.Args {
			if strings.ContainsRune(argument, '\x00') {
				return nil, fmt.Errorf(
					"%s: argument %d contains a NUL byte",
					spec.ID,
					argumentIndex,
				)
			}
		}
	}
	return specs, nil
}

func buildSmokePlan(
	recipeID string,
	projectDir string,
	specs []smokeSpec,
	resolver commandResolver,
) (planner.Plan, error) {
	steps := make([]planner.PlanStep, 0, len(specs))
	for _, spec := range specs {
		binary, prefixArgs, err := resolver.ResolveCommand(spec.Bin)
		if err != nil {
			return planner.Plan{}, fmt.Errorf("%s: resolve %q: %w", spec.ID, spec.Bin, err)
		}
		arguments := append(append([]string(nil), prefixArgs...), spec.Args...)
		directory := projectDir
		if spec.CWD == "parentDir" {
			directory = filepath.Dir(projectDir)
		}
		environment := map[string]string{
			"ASTRO_TELEMETRY_DISABLED": "1",
			"CI":                       "true",
			"DO_NOT_TRACK":             "1",
			"EXPO_NO_TELEMETRY":        "1",
			"NEXT_TELEMETRY_DISABLED":  "1",
			"NO_COLOR":                 "1",
		}
		for key, value := range spec.Env {
			environment[key] = value
		}
		environment["CI"] = "true"
		steps = append(steps, planner.PlanStep{
			ID:      spec.ID,
			Label:   spec.Label,
			Bin:     binary,
			Args:    arguments,
			Dir:     directory,
			Env:     environment,
			Display: displayArgv(binary, arguments),
		})
	}
	return planner.Plan{
		RecipeID:   recipeID,
		ProjectDir: projectDir,
		Steps:      steps,
		Warnings:   []string{},
	}, nil
}

func displayArgv(binary string, args []string) string {
	argv := append([]string{binary}, args...)
	encoded, err := json.Marshal(argv)
	if err != nil {
		return fmt.Sprintf("%q", argv)
	}
	return string(encoded)
}

func logPlan(logger *log.Logger, phase string, plan planner.Plan) {
	logger.Printf("%s plan: recipe=%s project=%s steps=%d", phase, plan.RecipeID, plan.ProjectDir, len(plan.Steps))
	for index, step := range plan.Steps {
		logger.Printf(
			"%s step %d/%d id=%s cwd=%s argv=%s",
			phase,
			index+1,
			len(plan.Steps),
			step.ID,
			step.Dir,
			displayArgv(step.Bin, step.Args),
		)
	}
}

func executePlan(
	parent context.Context,
	logger *log.Logger,
	phase string,
	plan planner.Plan,
	limit time.Duration,
	pathResolver *toolchain.PathResolver,
) error {
	manager := runner.NewManager(runner.WithPathProvider(func() (string, error) {
		return pathResolver.ResolvedPath(parent)
	}))
	unsubscribe := manager.Subscribe(func(event runner.Event) {
		step, ok := event.Payload.(runner.StepEvent)
		if !ok {
			return
		}
		logger.Printf(
			"%s step id=%s index=%d/%d state=%s",
			phase,
			step.StepID,
			step.Index+1,
			step.Total,
			step.State,
		)
	})
	defer unsubscribe()

	job, err := manager.Start(plan)
	if err != nil {
		return fmt.Errorf("start %s: %w", phase, err)
	}
	runContext, cancel := context.WithTimeout(parent, limit)
	defer cancel()

	nextSequence := 0
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		nextSequence = drainLogs(logger, manager, job.ID, nextSequence)
		status, statusErr := manager.Status(job.ID)
		if statusErr != nil {
			return fmt.Errorf("read %s status: %w", phase, statusErr)
		}
		switch status.State {
		case runner.StateSuccess:
			drainLogs(logger, manager, job.ID, nextSequence)
			logger.Printf("%s completed successfully", phase)
			return nil
		case runner.StateFailed, runner.StateCancelled:
			drainLogs(logger, manager, job.ID, nextSequence)
			return fmt.Errorf(
				"%s ended state=%s exitCode=%d error=%s",
				phase,
				status.State,
				status.ExitCode,
				status.Error,
			)
		}

		select {
		case <-runContext.Done():
			if cancelErr := manager.Cancel(job.ID); cancelErr != nil {
				logger.Printf("%s cancellation warning: %v", phase, cancelErr)
			}
			shutdownContext, shutdownCancel := context.WithTimeout(
				context.Background(),
				shutdownTimeout,
			)
			shutdownErr := manager.Shutdown(shutdownContext)
			shutdownCancel()
			drainLogs(logger, manager, job.ID, nextSequence)
			if shutdownErr != nil {
				logger.Printf("%s shutdown warning: %v", phase, shutdownErr)
			}
			return fmt.Errorf("%s exceeded timeout %s: %w", phase, limit, runContext.Err())
		case <-ticker.C:
		}
	}
}

func drainLogs(
	logger *log.Logger,
	manager *runner.Manager,
	jobID string,
	fromSequence int,
) int {
	lines, err := manager.Logs(jobID, fromSequence)
	if err != nil {
		logger.Printf("log replay warning: %v", err)
		return fromSequence
	}
	for _, line := range lines {
		logger.Printf("[%s][%s] %s", line.StepID, line.Stream, line.Text)
		fromSequence = line.Seq + 1
	}
	return fromSequence
}
