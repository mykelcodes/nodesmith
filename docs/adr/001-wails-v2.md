# ADR-001 — Wails v2

- Status: accepted
- Date: 2026-07-28

## Context

The original planning pack proposed Wails v3. During implementation, the project owner explicitly
selected Wails v2 and explicitly ruled out Wails v3.

## Decision

Nodesmith uses the exact Wails version pinned in `go.mod`:
`github.com/wailsapp/wails/v2 v2.13.0`.

The domain architecture remains unchanged:

- generator behavior is declarative recipe data;
- planning is pure and separate from execution;
- core packages do not import Wails;
- the Wails layer is an adapter over core packages;
- commands use explicit executable and argument slices, never a shell command string.

The bridge follows Wails v2 conventions:

- bound service structs are registered in `options.App.Bind`;
- the application context is captured in `OnStartup` and passed to services;
- backend events use `runtime.EventsEmit`;
- frontend events use the generated `wailsjs/runtime` module;
- native directory selection uses `runtime.OpenDirectoryDialog`;
- bindings are generated with `wails generate module`.

## Consequences

The Wails-v3-specific service registration and binding paths in the supplied architecture document
do not apply. Public method names, DTO JSON fields, event names, and the recipe/plan contracts remain
the product contract unless another ADR changes them.

