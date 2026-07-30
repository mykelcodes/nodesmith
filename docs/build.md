# Building Nodesmith

Nodesmith is a Wails v2.13.0 desktop application with a Go backend and a Svelte frontend. The
commands below intentionally pin the CLI to the same release as `go.mod`.

## Prerequisites

- Go 1.25 or newer.
- Node.js 24 and pnpm 11.6.0. Node 22.12 or newer also satisfies the frontend toolchain.
- [go-task](https://taskfile.dev/) for the convenience commands in `Taskfile.yml` (optional).
- The native dependencies for your platform, described below.

You do not need a globally installed Wails CLI. Task invokes this exact module:

```text
go run github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
```

Install the frontend once, then use the Taskfile:

```sh
task frontend:install
pnpm --dir frontend exec playwright install chromium
task check
task dev
task build
```

`task bindings` refreshes `frontend/src/lib/wailsjs`. Generated bindings are committed, and
`task bindings:check` fails if regeneration changes them. `task check` also runs the pinned
golangci-lint v2.11.4 rules from `.golangci.yml`. Chromium is required because the frontend Vitest
suite includes Playwright browser-mode component tests. On Linux, use
`pnpm --dir frontend exec playwright install --with-deps chromium` to install Chromium's native
dependencies at the same time.

## Platform dependencies

### macOS

Install the current Xcode Command Line Tools. The macOS SDK must include the
`UniformTypeIdentifiers` framework used by Wails file dialogs. Current macOS SDKs also need an
explicit deployment target when linking the Wails v2 application. The Taskfile sets
`CGO_LDFLAGS="-mmacosx-version-min=10.13 -framework UniformTypeIdentifiers"` for local Wails
bindings, development, and build commands. The universal build combines amd64 and arm64 binaries
with `lipo`:

```sh
task build:macos-universal
```

The result is under `build/bin/`, including `Nodesmith.app`.

### Linux

The CI build uses Ubuntu 22.04 and WebKitGTK 4.0:

```sh
sudo apt-get update
sudo apt-get install build-essential libgtk-3-dev libwebkit2gtk-4.0-dev pkg-config
task build:linux-amd64
```

On distributions that only ship WebKitGTK 4.1, install the equivalent GTK 3, WebKitGTK 4.1,
compiler, and pkg-config packages, then use:

```sh
task build:linux-amd64-webkit41
```

The Linux binary is written to `build/bin/nodesmith`.

### Windows

Install the Go and Node.js toolchains plus the Microsoft WebView2 runtime. A normal supported
Windows 10 or Windows 11 installation generally already has WebView2.

```powershell
task build:windows-amd64
```

The executable is written to `build/bin/nodesmith.exe`.

## CI

`.github/workflows/ci.yml` runs Go tests, vet, golangci-lint v2.11.4, frontend type-checking,
linting and unit tests, binding freshness checks, and native Wails builds on macOS, Windows, and
Linux. macOS produces a universal application; Windows and Linux produce amd64 builds. Each run
installs the Playwright Chromium version selected by the frontend lockfile; Linux also installs its
browser system dependencies. Branch and pull-request runs upload short-lived commit artifacts. A
`v*` tag uploads 90-day workflow artifacts named
`nodesmith-<tag>-darwin-universal`, `nodesmith-<tag>-windows-amd64`, and
`nodesmith-<tag>-linux-amd64`. After lint and all platform builds pass, the tag also publishes a
GitHub Release with generated release notes and these download assets:

- `nodesmith-<tag>-darwin-universal.zip`
- `nodesmith-<tag>-windows-amd64.zip`
- `nodesmith-<tag>-linux-amd64.tar.gz`
- `SHA256SUMS`

Push a version tag such as `v1.0.0` to create the corresponding release. The macOS archive contains
the universal application bundle, the Windows archive contains the amd64 executable, and the Linux
archive contains the amd64 binary.

`.github/workflows/nightly-recipes.yml` has two layers:

1. The offline contract matrix validates the embedded catalogue, parser, and committed golden plans
   on macOS, Windows, and Linux without executing generator code.
2. The execution matrix scaffolds all 14 recipes on every operating system, installs their
   dependencies, and performs a recipe-appropriate build smoke test. It pins Wails to v2.13.0,
   installs Rust for Tauri, and installs the required WebKitGTK development packages on Linux.

The execution matrix uses `cmd/recipecheck`, which accepts smoke commands as strict JSON, resolves
every executable through Nodesmith's native allowlisted resolver, and executes explicit argv through
the production runner. It never passes a recipe or smoke command through a shell. Each scaffold and
smoke phase has a 25-minute process-tree-aware timeout, the job has a 55-minute ceiling, and its
temporary project is removed on success or failure.

If either the offline contracts or execution matrix fails, a least-privilege notification job opens
an issue containing the workflow run. Further failures comment on that open issue instead of
creating duplicates; closing it allows a later regression to open a fresh issue.

You can validate a recipe plan locally without downloading or running its generator:

```sh
task recipes:verify:dry \
  RECIPE=vite \
  'SMOKE_JSON=[{"label":"Build","bin":"npm","args":["run","build"]}]'
```

Remove `:dry` to execute the generator, dependency install, and smoke command. All built-in recipes
support npm, so the scheduled workflow uses npm consistently across operating systems.
