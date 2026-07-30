# Toolchain resolution

Nodesmith resolves and executes generator tools without constructing a command
line for a shell. Recipe steps can name only these logical binaries:

`node`, `npm`, `npx`, `pnpm`, `pnpx`, `yarn`, `bun`, `bunx`, `git`, `go`,
`cargo`, `gh`, and `code`.

`internal/toolchain.Resolver` checks each PATH entry itself and returns an
absolute path. A recipe cannot supply a path, extension, alternate casing, or
unlisted executable. On Windows, resolution also checks native executables and
the standard `.cmd`, `.bat`, and `.ps1` shims. Native `.exe`/`.com` files are
preferred. Resolution does not wrap a shim in `cmd.exe` or PowerShell.

## Login-shell PATH exception

Desktop applications launched from Finder or a Linux application menu commonly
inherit a minimal PATH. That hides tools installed by nvm, fnm, Volta, or shell
startup configuration.

On macOS and Linux, Nodesmith asks the configured interactive login shell for
its environment once. The interactive mode includes tool managers commonly
initialised by `.zshrc`, `.bashrc`, or equivalent shell configuration. If a
Finder-launched macOS app receives no `SHELL`, Nodesmith uses the system
`/bin/zsh`. This is the sole documented exception to the no-shell rule. The
invocation is equivalent to the following explicit argv:

```text
[$SHELL, "-l", "-i", "-c", "env"]
```

The command text is a constant. No project name, recipe value, PATH override, or
other user-controlled value is interpolated into it. The process has a
two-second context deadline. Nodesmith extracts the `PATH=` entry; if the shell
is missing, exits unsuccessfully, times out, or returns no PATH, it uses the
current process `PATH`. Windows never performs login-shell discovery and uses
the process PATH directly.

The discovered value is cached in memory for the application's lifetime and is
exposed by `ResolvedPath`. A settings override takes effect immediately through
`SetPathOverride`; clearing it reveals the previously discovered value without
launching the login shell again.

All actual tool execution, including version probes, uses an absolute binary
path and an explicit argv slice via `exec.CommandContext`. It never uses
`sh -c`, `cmd /c`, PowerShell command text, or a joined command line.

## Detection and recipe gates

Toolchain detection scans `node`, `npm`, `npx`, `pnpm`, `yarn`, `bun`, `git`,
`go`, `cargo`, `gh`, and `code` concurrently. An absent binary is a normal
`Present: false` result. A present binary whose version command fails remains
present and carries the probe error. Results are cached for 60 seconds; a forced
scan, cache expiry, or effective PATH change triggers a fresh scan.

Versions are normalized from each tool's output. Recipe gates support exact
versions and AND-only comparator ranges using `>=`, `>`, `<=`, `<`, `=`, or
`==`, for example:

```text
>=20.0.0
>=20.0.0 <21.0.0
```

Caret, tilde, wildcard, and OR ranges are intentionally unsupported in v1.
`EvaluateRequirements` reports every blocking reason, chooses the first
available package manager in recipe order, and lets the UI disable execution
instead of allowing a late process failure.

## Editor and file-manager integration

Desktop post-actions follow the same explicit-argv policy. Editor selection is
an exact identifier from `code`, `cursor`, or `zed`; paths, flags, alternate
casing, and joined command strings are rejected. The project directory is one
argv element even when it contains whitespace or shell metacharacters.

File-manager reveal uses fixed platform commands: `open -R <dir>` on macOS,
`xdg-open <dir>` on Linux, and `explorer.exe <dir>` on Windows. These commands
are selected by build-tagged helpers and resolved against the same effective
PATH. No desktop integration invokes a shell.
