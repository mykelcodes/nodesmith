# Nodesmith

Nodesmith is a desktop app for scaffolding JavaScript and TypeScript projects through a safe,
form-driven workflow. Recipes describe official generator CLIs as data; Nodesmith checks the local
toolchain, resolves an exact execution plan for review, and streams the resulting output in the app.

> [!IMPORTANT]
> Nodesmith deliberately uses **Wails v2.13.0**. Its Go dependency, CLI, generated bindings, runtime
> APIs, build configuration, CI, and bundled Wails recipe all target Wails v2. Do not introduce or
> migrate this repository to Wails v3.

## Features

- Data-driven catalogue and configuration forms with toolchain availability checks.
- Portable project-name, destination, collision, symlink, and write-access validation.
- Mandatory review of each resolved executable, working directory, and argv before execution.
- Native process execution with explicit argv; recipe values are never interpolated into shell text.
- One active job at a time, process-tree cancellation, ordered stdout/stderr, bounded live delivery,
  and replayable server-side logs.
- Result actions for opening an editor, revealing the project, copying its path, and retrying.
- Local presets, project history, theme settings, PATH override, and toolchain doctor.
- Native directory picker plus VS Code, Cursor, Zed, or absolute custom-editor executable support.

The review is an execution boundary, not a preview assembled by the frontend. Starting a job
revalidates the target and toolchain, resolves the plan again, and requires its hash to match the
reviewed plan. Allowlisted executables are resolved to absolute paths; Windows Node CLI shims are
invoked through `node.exe`, without `cmd.exe` or PowerShell.

## Bundled recipes

Nodesmith ships 14 validated recipes:

- Frontend: React, Solid, Svelte, and Vite.
- Full stack: Astro, Next.js, and SvelteKit.
- Backend: Express, Hono, and NestJS.
- Desktop: Electron, Tauri, and Wails v2.
- Mobile: Expo.

Recipe manifests live in [`recipes/`](recipes/). User recipes can override bundled recipes from the
Nodesmith configuration directory; invalid user recipes are skipped with a surfaced warning. See
the [recipe authoring guide](docs/recipes.md) for the schema and safety rules.

## Stack

- Go 1.25.12+
- Wails v2.13.0
- Svelte 5 and TypeScript
- Tailwind CSS v4
- Node.js 24 and pnpm 11.6.0

## Development

Install [go-task](https://taskfile.dev/), Go, Node.js, pnpm, and the native dependencies for your
platform. The Taskfile runs the pinned Wails v2 CLI through `go run`, so a global Wails installation
is not required.

```sh
task frontend:install
pnpm --dir frontend exec playwright install chromium
task dev
```

Useful commands:

```sh
task check                    # backend, frontend, recipes, lint, and binding freshness
task go:test:race             # Go tests with the race detector
task recipes:validate         # manifests and resolved golden plans, without generators
task bindings                 # regenerate Wails v2 JS/TS bindings
task build                    # native build for the current platform
task doctor                   # Wails v2 platform dependency report
```

To run only the browser frontend, use `pnpm --dir frontend dev`. Backend-dependent actions require
the Wails desktop runtime.

## Platform builds

- macOS: install the current Xcode Command Line Tools; `task build:macos-universal` creates the
  universal application bundle.
- Windows: install the Microsoft WebView2 runtime; `task build:windows-amd64` creates the executable.
- Linux: install `build-essential`, GTK 3, `pkg-config`, and WebKitGTK. Use
  `task build:linux-amd64` for WebKitGTK 4.0 or `task build:linux-amd64-webkit41` for 4.1.

Build artifacts are written under `build/bin/`. CI runs tests and native builds on macOS, Windows,
and Linux. Pushing a `v*` tag publishes the three packaged builds and their checksums on
[GitHub Releases](https://github.com/mykelcodes/nodesmith/releases); the nightly workflow scaffolds
and smoke-builds every bundled recipe on all three.

## Documentation

- [Build, CI, and platform dependencies](docs/build.md)
- [Wails v2 architecture decision](docs/adr/001-wails-v2.md)
- [Wails v2 integration notes](docs/wails-v2-cheatsheet.md)
- [Frontend service and event API](docs/frontend-api.md)
- [Recipe authoring and validation](docs/recipes.md)
- [Toolchain detection and PATH resolution](docs/toolchain.md)
