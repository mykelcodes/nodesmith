# Nodesmith

Nodesmith is a Wails v2 desktop app for scaffolding JavaScript and TypeScript projects through a
safe, form-driven workflow. Recipes describe official generator CLIs as data; Nodesmith resolves
them into exact argument arrays, shows every command for review, then streams execution output.

## Stack

- Go 1.25+
- Wails v2.13.0
- Svelte 5 and TypeScript
- Tailwind CSS v4
- pnpm

## Development

```bash
cd frontend && pnpm install
wails dev
```

The browser-only frontend can be run with `cd frontend && pnpm dev`. Wails bindings are generated
with `wails generate module`.

## Verification

```bash
go test ./...
go vet ./...
cd frontend && pnpm check && pnpm test && pnpm build
wails build
```

See `docs/adr/001-wails-v2.md` for the deliberate Wails v2 decision.
