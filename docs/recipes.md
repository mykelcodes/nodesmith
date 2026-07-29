# Recipe authoring

Nodesmith recipes are declarative JSON manifests. A recipe describes the form shown to the user
and the exact process arguments Nodesmith may execute; it does not contain JavaScript or Go code.
The canonical machine-readable contract is
[`recipes/recipe.schema.json`](../recipes/recipe.schema.json), and
[`recipes/vite.json`](../recipes/vite.json) is the reference implementation.

## Add or override a recipe

Open the user recipe folder from Nodesmith's settings, add a file ending in `.json`, then reload
recipes. The folder is `<user-config>/nodesmith/recipes`, where `<user-config>` is the
platform-specific user configuration directory.

A valid user recipe with a new `id` is added to the catalogue. A valid user recipe with the same
`id` as a bundled recipe overrides that recipe. Invalid files are skipped and shown as warnings;
they never partially load.

## Minimal valid manifest

This example uses an allowlisted executable, passes one value per argv element, avoids generator
prompts, and leaves dependency installation and Git initialisation to Nodesmith:

```json
{
  "schemaVersion": 1,
  "id": "acme-web",
  "name": "Acme Web",
  "category": "frontend",
  "description": "An Acme-flavoured React starter built with Vite.",
  "docsUrl": "https://vite.dev/guide/",
  "tags": ["react", "vite"],
  "icon": "vite",
  "verifiedAt": "2026-07-28",
  "requires": {
    "node": ">=20.19.0",
    "packageManagers": ["pnpm", "npm", "yarn", "bun"],
    "tools": []
  },
  "fields": [
    {
      "id": "typescript",
      "label": "TypeScript",
      "type": "boolean",
      "default": true
    }
  ],
  "steps": [
    {
      "id": "scaffold",
      "label": "Scaffold project",
      "bin": "npx",
      "cwd": "parentDir",
      "env": {"CI": "1"},
      "args": [
        "--yes",
        "create-vite@latest",
        "${projectName}",
        "--template",
        {
          "if": "typescript",
          "then": ["react-ts"],
          "else": ["react"]
        },
        "--no-interactive"
      ]
    },
    {
      "id": "install",
      "label": "Install dependencies",
      "bin": "${packageManager}",
      "cwd": "projectDir",
      "env": {"CI": "1"},
      "args": ["install"],
      "when": "installDeps"
    },
    {
      "id": "git-init",
      "label": "Initialise repository",
      "bin": "git",
      "cwd": "projectDir",
      "env": {"CI": "1"},
      "args": ["init"],
      "when": "gitInit"
    }
  ]
}
```

## Top-level properties

| Property | Meaning |
|---|---|
| `schemaVersion` | Must be `1`. |
| `id` | Stable, unique, kebab-case identifier used by presets and history. |
| `name` | Human-readable catalogue name. |
| `category` | One of `frontend`, `fullstack`, `backend`, `desktop`, `mobile`, or `tooling`. |
| `description` | A short catalogue description. |
| `docsUrl` | Absolute HTTP(S) documentation URL. |
| `tags` | Unique kebab-case search and filter terms. |
| `icon` | Key understood by the Nodesmith frontend icon map. |
| `verifiedAt` | Date in `YYYY-MM-DD` form when the command was last verified. |
| `requires` | Node range, supported package managers, and required extra tools. |
| `fields` | Options used to build the dynamic form. |
| `steps` | Ordered process invocations; at least one is required. |

`requires.packageManagers` may contain `pnpm`, `npm`, `yarn`, and `bun`. Only list managers the
generator actually supports. `requires.tools` may contain extra allowlisted binaries that must be
present before the recipe is available.

## Fields

Every field needs a kebab-case `id`, a `label`, a `type`, and a type-correct `default`.

| Type | Default | Additional properties |
|---|---|---|
| `select` | A string matching one option value | Non-empty `options` |
| `multiselect` | An array of option values | Non-empty `options` |
| `boolean` | `true` or `false` | — |
| `text` | A string | — |
| `number` | A JSON number | — |

Options use `{ "value": "...", "label": "..." }`. A field may also include `help` and
`visibleIf`. Field ids may not shadow the built-in variables below.

## Built-in variables

Nodesmith always provides:

- `projectName`
- `projectDir`
- `parentDir`
- `packageManager`
- `installDeps`
- `gitInit`

Use `${variable}` substitution inside a literal argv element. Substitution never performs shell
parsing and never splits on whitespace, so `"--template ${template}"` is one incorrect argument.
Write it as two elements: `"--template", "${template}"`.

The only dynamic executable is `"${packageManager}"`. All other `bin` values must be one of:
`bun`, `bunx`, `cargo`, `code`, `gh`, `git`, `go`, `node`, `npm`, `npx`, `pnpm`, `pnpx`,
`wails`, or `yarn`. User recipes cannot expand this executable allowlist. Wails recipes must
target the Wails v2 `wails` CLI; Nodesmith does not support the Wails v3 `wails3` executable.

Set the optional top-level `installPolicy` to `"required"` when the upstream generator always
installs dependencies and cannot honour `installDeps=false`. Nodesmith treats omitted values as
`"optional"` for backwards compatibility and disables the corresponding UI control for required
recipes.

## Argument nodes

Each entry in `args` is one of three closed forms:

1. A string, producing exactly one argv element.
2. A conditional object:

   ```json
   {
     "if": "typescript",
     "then": ["--typescript"],
     "else": ["--javascript"]
   }
   ```

3. A multiselect expansion:

   ```json
   {
     "forEach": "addons",
     "args": ["--add", "${item}"]
   }
   ```

Conditional and iteration nodes may nest to a maximum object depth of three. `forEach` must
reference a `multiselect` field, and `${item}` only exists inside that node.

## Conditions

Conditions deliberately support a small grammar:

```text
field
!field
field == "value"
field != "value"
field includes "value"
```

Literals may be quoted strings, numbers, `true`, or `false`. `includes` is only valid for a
multiselect field. There are no `&&`, `||`, parentheses, or arbitrary expressions. Use nested
conditional argument nodes when a command needs more than two branches.

## Steps and non-interactive execution

Each step requires:

- a unique kebab-case `id`;
- a human-readable `label`;
- an allowlisted `bin`;
- `cwd` set to `parentDir` or `projectDir`;
- an `env` object containing `"CI": "1"`;
- an argv array in `args`.

Environment names must be valid cross-platform names and unique without regard
to case. `PATH` is reserved because Nodesmith supplies the exact detected path
to every process, and `CI` is always enforced as `1`.

Use `when` to gate a whole step. Scaffold steps normally run in `parentDir`; install and Git steps
normally run in `projectDir`. Bundled recipes initialise Git, stage the generated files, and create
an `Initial commit` when `gitInit` is enabled.

For a plain `pnpm install` step, Nodesmith adds
`--config.strict-dep-builds=false` to the reviewed command. pnpm still blocks
unapproved dependency build scripts and records them in `pnpm-workspace.yaml`,
but the pending approvals do not turn an otherwise completed scaffold into a
failed run. Users can review and approve those scripts later with
`pnpm approve-builds`. A recipe that supplies an explicit pnpm build-script
policy keeps that policy unchanged.

Recipes must never rely on a prompt. Verify the current generator help or upstream CLI option
definitions, then provide the project name and every option needed to suppress questions.
Prefer explicit no-install and no-git flags so Nodesmith retains control through `installDeps` and
`gitInit`. If a generator cannot suppress one of those side effects, document the exception and do
not invent a flag.

Never place `sh`, `bash`, `cmd`, or PowerShell in a recipe. Never use `-c`, command substitution,
redirection, pipes, or a string-built command line. Every process and argument must remain visible
as structured data in the dry-run plan.

## Verification checklist

Before changing `verifiedAt`:

1. Read the generator's current official documentation and current `--help` output or upstream
   option definitions.
2. Confirm every prompt has an explicit answer or skip flag.
3. Validate the JSON against the recipe schema and Nodesmith's manifest validator.
4. Resolve a dry-run and inspect every argv element, working directory, and environment value.
5. Scaffold into a disposable temporary directory.
6. If dependencies are supported, install them and run the generated project's build command.
7. Confirm `installDeps: false` and `gitInit: false` avoid those side effects whenever the
   upstream generator exposes the necessary controls.
8. Delete the temporary project and set `verifiedAt` to the verification date.

Generator flags drift. Treat a nightly scaffold failure as a recipe defect, re-check upstream help,
and change only the manifest rather than adding generator-specific logic to the Go backend.
