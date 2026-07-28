# Wails v2 integration notes

Nodesmith is pinned to Wails v2.13.0. This file records the integration patterns used in this
repository so future work does not accidentally introduce Wails v3 APIs.

## Application and bindings

- Bootstrap with `wails.Run(&options.App{...})`.
- Register bound structs in `options.App.Bind`.
- Capture `context.Context` through `OnStartup`.
- Generate bindings with `wails generate module`.
- Generated Go bindings live under the configured `wailsjsdir` and are imported from
  `frontend/src/lib/wailsjs/go/...`.

## Runtime operations

- Emit from Go with `runtime.EventsEmit(ctx, name, payload)`.
- Subscribe in the frontend with `EventsOn` from
  `frontend/src/lib/wailsjs/runtime/runtime`; keep and call the returned unsubscribe function.
- Pick a directory with `runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{...})`.
- Open a URL with `runtime.BrowserOpenURL(ctx, url)`.

## Guardrails

- Do not import `github.com/wailsapp/wails/v3`.
- Do not use `application.New`, `application.NewService`, or `app.Event.Emit`.
- Do not hand-edit generated files under `frontend/src/lib/wailsjs`.
- Keep Wails imports out of domain packages.

Official references:

- https://wails.io/docs/howdoesitwork/
- https://wails.io/docs/reference/runtime/events/
- https://wails.io/docs/reference/runtime/dialog/

