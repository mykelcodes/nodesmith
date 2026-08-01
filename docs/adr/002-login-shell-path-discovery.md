# ADR-002 — Login-shell PATH discovery uses `-l -i`

- Status: accepted
- Date: 2026-08-01

## Context

A macOS application launched from Finder, and a Linux application launched from a desktop
entry, does not inherit the shell's PATH. It receives the minimal system PATH, which
contains none of `/opt/homebrew/bin`, `~/.nvm/versions/node/*/bin`, `~/.volta/bin`, or
`~/.local/share/fnm`. Every tool Nodesmith detects then appears missing and every recipe
that requires one is gated off.

Nodesmith therefore discovers the user's real PATH once at startup by running
`$SHELL -l -i -c env` and reading the `PATH=` line
(`discoverLoginShellPATH` in `internal/toolchain/path.go`).

The adversarial audit (T-24) questioned the `-i` flag. `-i` sources interactive startup
files, which commonly print banners, draw a prompt, or start background processes — the
last of which can hold the output pipe open past the deadline. Discovery is a
non-interactive query, so `-l` alone looks like the correct choice.

## Decision

Keep `-l -i`.

The shells Nodesmith supports split their startup files by *both* login and interactive
status, and the version managers that matter most install into the interactive file:

| Shell | `-l` sources | `-i` additionally sources |
| --- | --- | --- |
| zsh | `.zshenv`, `.zprofile`, `.zlogin` | `.zshrc` |
| bash | `.bash_profile` / `.profile` | `.bashrc` |

- The nvm installer appends its `NVM_DIR` export and `nvm.sh` source line to `~/.zshrc`
  (and `~/.bashrc`), not to a login file.
- fnm's `fnm env` shell hook and Volta's PATH export follow the same convention.
- Homebrew's `shellenv` is the exception: `brew` instructs users to add it to
  `~/.zprofile`, so Homebrew alone would survive dropping `-i`.

Dropping `-i` would therefore silently break Node version detection for the single most
common macOS Node setup, which is the exact failure this discovery exists to prevent.

The hang risk `-i` introduces is addressed directly rather than by removing the flag:

- The discovery context carries a 2-second deadline (`loginShellTimeout`).
- `cmd.WaitDelay` is set to 1 second (`loginShellWaitDelay`), so a background process
  started by an rc file that inherits stdout cannot keep `Output` blocked past the
  deadline. This closed T-01 for this call site.
- Discovery failure is not fatal: `ResolvedPath` falls back to the process PATH.

## Consequences

- Users whose interactive startup files are slow pay up to 2 seconds once per launch, on
  the first call to `ResolvedPath`. The result is cached for the process lifetime.
- Discovery failure is no longer silent. `PathResolver.DiscoveryWarning` retains the
  reason and `ToolchainService.Detect` surfaces it in the toolchain doctor, so a user
  seeing "every tool is missing" is shown the cause rather than only the symptom.
- A user whose setup this still does not cover can set an explicit PATH override in
  Settings, which takes precedence over discovery entirely.
- If a supported shell later moves version-manager initialisation out of the interactive
  file, this decision should be revisited.
