# Frontend bridge API

Nodesmith binds four Go services through Wails v2. Generated modules live under
`frontend/src/lib/wailsjs/go/services`; application code calls the typed wrappers in
`frontend/src/lib/api` rather than importing generated bindings from components.

## RecipeService

| Method | Use |
|---|---|
| `List()` | Load catalogue summaries, availability, and missing-tool reasons. |
| `Get(id)` | Load the full manifest that drives the configure form. |
| `Reload()` | Rescan bundled and user recipes, then refresh the catalogue. |
| `Validate(raw)` | Validate recipe JSON in an authoring workflow without saving it. |
| `OpenRecipeDir()` | Reveal the local recipe override folder in the platform file manager. |

`nodesmith:recipes:reloaded` is emitted after a successful reload.

## ToolchainService

| Method | Use |
|---|---|
| `Detect(force)` | Read the cached tool scan, or force a fresh version probe. |
| `ResolvedPath()` | Show the exact `PATH` used for discovery and generator processes. |
| `SetPathOverride(path)` | Persist and immediately apply a replacement `PATH`; an empty value restores discovery. |

`nodesmith:toolchain:changed` is emitted after a forced scan or a `PATH` change.

## ScaffoldService

| Method | Use |
|---|---|
| `Plan(request)` | Validate the destination and resolve the exact argv shown on the mandatory review screen. |
| `Start(request)` | Start only the same request that was most recently reviewed with `Plan`. |
| `Cancel(jobId)` | Cancel a pending or running job and its child process group. |
| `Status(jobId)` | Reattach to the current job snapshot after navigation. |
| `Logs(jobId, fromSeq)` | Replay retained console lines, inclusive of `fromSeq`. |
| `PickDirectory(startAt)` | Open the Wails v2 native directory picker. |
| `OpenInEditor(dir, editor)` | Open a completed project in VS Code, Cursor, or Zed. |
| `RevealInFileManager(dir)` | Reveal a completed project using the native file manager. |

Job events are `nodesmith:job:started`, `nodesmith:job:step`,
`nodesmith:job:log`, and `nodesmith:job:done`. Log payloads include a monotonically increasing
sequence number so the UI can deduplicate event/backfill overlap.

## StoreService

| Method | Use |
|---|---|
| `GetSettings()` / `SaveSettings(settings)` | Load or atomically persist local preferences. |
| `ListPresets()` / `SavePreset(preset)` / `DeletePreset(id)` | Manage named scaffold configurations. |
| `ListHistory(limit)` | Load newest project attempts first; a non-positive limit returns all retained history. |
| `ClearHistory()` | Atomically replace project history with an empty list. |

All rejected service promises contain user-facing context. Treat the backend as the authority for
paths, project names, recipe validation, and plan review.
