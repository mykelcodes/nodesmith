package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	"nodesmith/internal/runner"
	"nodesmith/internal/services"
	"nodesmith/internal/toolchain"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/build
var assets embed.FS

//go:embed recipes/*.json
var recipeFiles embed.FS

// configDir locates Nodesmith's local state. It never fails: a user
// configuration directory that cannot be determined degrades to the home
// directory and then to the system temporary directory, because losing
// persistence is far better than refusing to start.
func configDir() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "nodesmith")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".nodesmith")
	}
	return filepath.Join(os.TempDir(), "nodesmith")
}

// fatal reports a startup failure that leaves nothing to run. No window exists
// yet, and an application launched from Finder or the Start menu has no console
// the user can read, so the message is also appended to a log file beside the
// application's other local state.
func fatal(stateDir string, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	log.Print(message)
	if stateDir != "" {
		if err := os.MkdirAll(stateDir, 0o700); err == nil {
			path := filepath.Join(stateDir, "startup-error.log")
			file, openErr := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
			if openErr == nil {
				_, _ = fmt.Fprintf(file, "%s %s\n", time.Now().UTC().Format(time.RFC3339), message)
				_ = file.Close()
			}
		}
	}
	os.Exit(1)
}

func main() {
	nodesmithConfigDir := configDir()
	recipeFS, err := fs.Sub(recipeFiles, "recipes")
	if err != nil {
		fatal(nodesmithConfigDir, "prepare bundled recipes: %v", err)
	}
	userRecipeDir := filepath.Join(nodesmithConfigDir, "recipes")
	defaultParentDir, err := os.UserHomeDir()
	if err != nil {
		defaultParentDir = ""
	}

	bridge := services.NewBridgeContext()
	storeService, err := services.NewStoreService(bridge, nodesmithConfigDir, defaultParentDir)
	if err != nil {
		fatal(nodesmithConfigDir, "initialise local storage: %v", err)
	}
	pathResolver := toolchain.NewPathResolver()
	binaryResolver := toolchain.NewResolver(pathResolver)
	detector := toolchain.NewDetector(binaryResolver)
	recipeService, err := services.NewRecipeService(bridge, recipeFS, userRecipeDir, detector)
	if err != nil {
		fatal(nodesmithConfigDir, "initialise recipes: %v", err)
	}
	toolchainService, err := services.NewToolchainService(
		bridge,
		pathResolver,
		detector,
		storeService,
	)
	if err != nil {
		fatal(nodesmithConfigDir, "initialise toolchain: %v", err)
	}
	jobManager := runner.NewManager(runner.WithPathProvider(func() (string, error) {
		return pathResolver.ResolvedPath(context.Background())
	}))
	scaffoldService, err := services.NewScaffoldService(
		bridge,
		recipeService,
		binaryResolver,
		jobManager,
		storeService,
	)
	if err != nil {
		fatal(nodesmithConfigDir, "initialise scaffold service: %v", err)
	}

	err = wails.Run(&options.App{
		Title:     "Nodesmith",
		Width:     1220,
		Height:    800,
		MinWidth:  960,
		MinHeight: 680,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 9, G: 11, B: 16, A: 1},
		Mac: &mac.Options{
			DisableZoom: false,
		},
		OnStartup: func(ctx context.Context) {
			bridge.Set(ctx)
		},
		OnDomReady: func(_ context.Context) {
			// The interface can receive events from here, so anything produced
			// before it existed — the recipe load report — is replayed now.
			bridge.NotifyUIReady()
			go func() {
				if _, detectErr := toolchainService.Detect(true); detectErr != nil {
					log.Printf("initial toolchain scan: %v", detectErr)
				}
			}()
		},
		OnShutdown: func(_ context.Context) {
			// Generous enough to cover a step waiting out the orphaned-output
			// grace period before it can report a terminal state. Timing out
			// here orphans the generator's process tree, which is exactly what
			// this hook exists to prevent.
			shutdownContext, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			if shutdownErr := jobManager.Shutdown(shutdownContext); shutdownErr != nil {
				log.Printf("stop active scaffold job: %v", shutdownErr)
			}
		},
		Bind: []interface{}{
			recipeService,
			toolchainService,
			scaffoldService,
			storeService,
		},
	})
	if err != nil {
		log.Fatalf("run Nodesmith: %v", err)
	}
}
