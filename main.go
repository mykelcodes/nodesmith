package main

import (
	"context"
	"embed"
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
)

//go:embed all:frontend/build
var assets embed.FS

//go:embed recipes/*.json
var recipeFiles embed.FS

func main() {
	recipeFS, err := fs.Sub(recipeFiles, "recipes")
	if err != nil {
		log.Fatalf("prepare bundled recipes: %v", err)
	}
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatalf("find the user configuration directory: %v", err)
	}
	nodesmithConfigDir := filepath.Join(userConfigDir, "nodesmith")
	userRecipeDir := filepath.Join(nodesmithConfigDir, "recipes")
	defaultParentDir, err := os.UserHomeDir()
	if err != nil {
		defaultParentDir = ""
	}

	bridge := services.NewBridgeContext()
	storeService, err := services.NewStoreService(nodesmithConfigDir, defaultParentDir)
	if err != nil {
		log.Fatalf("initialise local storage: %v", err)
	}
	pathResolver := toolchain.NewPathResolver()
	binaryResolver := toolchain.NewResolver(pathResolver)
	detector := toolchain.NewDetector(binaryResolver)
	recipeService, err := services.NewRecipeService(bridge, recipeFS, userRecipeDir, detector)
	if err != nil {
		log.Fatalf("initialise recipes: %v", err)
	}
	toolchainService, err := services.NewToolchainService(
		bridge,
		pathResolver,
		detector,
		storeService,
	)
	if err != nil {
		log.Fatalf("initialise toolchain: %v", err)
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
		log.Fatalf("initialise scaffold service: %v", err)
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
		OnStartup: func(ctx context.Context) {
			bridge.Set(ctx)
		},
		OnDomReady: func(_ context.Context) {
			go func() {
				if _, detectErr := toolchainService.Detect(true); detectErr != nil {
					log.Printf("initial toolchain scan: %v", detectErr)
				}
			}()
		},
		OnShutdown: func(_ context.Context) {
			shutdownContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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
