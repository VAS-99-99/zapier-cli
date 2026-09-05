# Zapier read-only CLI

`zapier-pp-cli` lets Claude, Codex, and people inspect a connected Zapier
account. It checks the account session, lists and searches Zaps, opens historical
runs, and pinpoints failed steps. It cannot change anything in Zapier.

Normal users do not need Go, Node.js, npm, Playwright, `agent-browser`, a browser
extension, or a Zapier developer token. The release installers provide the CLI
and MCP binaries. `auth browser` manages its pinned browser helper and browser
inside the CLI.

## Give this to Claude or Codex

Copy the block below. It keeps authentication outside the agent, which avoids
asking Claude to capture or handle a live browser session. The same text is in
`CLAUDE_INSTALL_PROMPT.txt`.

```text
Quick install our team's unofficial, read-only Zapier CLI from https://github.com/VAS-99-99/zapier-cli. I authorize downloading and installing this tool. Clone it into the current directory; if that destination exists, inspect only that directory and preserve its changes. Do not search the computer for a clone. Read CLAUDE.md, then run the repository installer for this operating system. Use only the prebuilt GitHub Release. For this private repository, reuse my authenticated GitHub CLI session; if access is missing, guide me through GitHub sign-in, never ask me to paste a token. Keep any necessary trust review focused on concrete concerns. The installer performs the checksum and version checks. Do not install Go or build from source.

After installation, confirm `zapier-pp-cli version` works in the current terminal. Fix command resolution yourself if needed. Do not ask me to open another terminal or edit PATH.

You are not being asked to authenticate, capture a session, inspect browser storage, or handle a cookie or token. After installation, stop and tell me to run `zapier-pp-cli auth browser` myself in my own terminal. I will complete the visible Zapier login outside Claude.

Only after I explicitly reply `connected`, run `zapier-pp-cli session --agent --no-learn`. Show me the exact connected account and stop for confirmation. Before that confirmation, do not run doctor, list Zaps, inspect runs, or make any other Zapier request. Never perform a remote Zapier write.
```

## Supported systems

The GitHub Release contains these checksummed archives:

| System | Release asset |
| --- | --- |
| Windows x64 | `zapier-cli_windows_x86_64.zip` |
| macOS Apple Silicon | `zapier-cli_darwin_arm64.tar.gz` |
| macOS Intel | `zapier-cli_darwin_x86_64.tar.gz` |
| Linux x64 | `zapier-cli_linux_x86_64.tar.gz` |

Each archive contains `zapier-pp-cli`, `zapier-pp-mcp`, this README, the agent
skill, and the license. `SHA256SUMS` covers every archive. The installers fetch
public releases directly from GitHub. For private releases they fall back to
an already authenticated GitHub CLI (`gh`) with access to this repository.
They never request or print a GitHub token. Repository permissions are managed
by your team; installation does not change them.

## Install manually

For this private repository, sign into GitHub CLI with a teammate account that
has repository access, then clone it. Public repositories do not require this
GitHub sign-in step.

```bash
git clone https://github.com/VAS-99-99/zapier-cli.git
cd zapier-cli
```

On macOS or Linux:

```bash
./install.sh
```

On Windows PowerShell:

```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

The installers select the current system archive, download it from the
GitHub Release, verify its checksum before extraction, and install both binaries
without administrator rights. The default directories are `$HOME/.local/bin`
on macOS/Linux and `%LOCALAPPDATA%\Microsoft\WindowsApps` on Windows. The
Windows directory is already on the normal user PATH, so the command works in
the current terminal. The macOS/Linux installer adds its directory to the
user's PATH when needed.

Useful installer options:

| macOS/Linux | Windows PowerShell | Purpose |
| --- | --- | --- |
| `--tag TAG` | `-Tag TAG` | Install a named release instead of the latest stable release |
| `--install-dir DIR` | `-InstallDir DIR` | Use another user-writable directory |
| `--verify-only` | `-VerifyOnly` | Download and verify without installing |
| `--no-path-update` | `-NoPathUpdate` | Leave the user's PATH unchanged |

## Connect the account

Run:

```bash
zapier-pp-cli auth browser
```

The CLI downloads its pinned browser components into user-local storage when
needed and opens a dedicated visible window. Run this command yourself rather
than asking an agent to handle authentication. Sign in to Zapier there. The CLI
validates the candidate session using only Zapier's session GET before saving
it. Cookies alone, an unfinished login, or an expired session are not success.
The browser closes automatically after a valid session is saved. The separate
account check below still requires your confirmation.

Credentials stay in the CLI's permission-checked local credential file; they
are not printed or exported for the agent. This file is not an encrypted
password vault. The CLI is read-only, but the underlying browser session can
carry the account's full permissions. Treat the credential file as a password,
keep it out of shared folders/backups, and use `auth logout` to remove the local
connection. Local logout is not a promise to revoke that session at Zapier.

Immediately after connection, run only:

```bash
zapier-pp-cli session --agent --no-learn
```

Show the exact returned account identity and wait for the user to confirm it.
Only then run `doctor` or another live read.

## Connect Claude or Codex

The installer prints commands with the absolute path to `zapier-pp-mcp`. Use
the command for the current host. These PATH-based forms also work when the host
inherits the updated PATH:

```bash
# Claude Code
claude mcp add --scope user zapier -- zapier-pp-mcp

# Codex
codex mcp add zapier -- zapier-pp-mcp
```

The MCP server reads the same protected local credential store as the same OS
user. Do not add a Zapier cookie or token to the MCP configuration. Restart or
reconnect the host after changing its MCP registration.

## What it can do

| Need | Command | Remote effect |
| --- | --- | --- |
| Check the connected account | `session`, `doctor` | Read only |
| List or search Zaps | `zaps list --name <text>` | Read only |
| List historical runs | `runs list --status error` | Read only |
| Inspect one run and its steps | `runs get <run-id>` | Read only |
| Find the failed step and error | `diagnose <zap-name-or-id>` | Read only |

The remote boundary is strict. The CLI has no command to create, edit, enable,
disable, rename, publish, replay, cancel, or delete Zapier data. Webhook delivery
and remote feedback are disabled. Output goes to stdout or an explicitly chosen
local file. Feedback and learning data remain on the current machine.

Use runtime discovery instead of relying on a copied command list:

```bash
zapier-pp-cli which "<capability>" --json
zapier-pp-cli <command> --help
```

Use `--agent` for compact JSON, non-interactive defaults, and no color. The
inspection commands return a stable envelope under `--agent`; read the payload
from `.results` and confirm `.meta.source == "live"`.

```bash
zapier-pp-cli zaps list --name "billing" --limit 5 --agent
zapier-pp-cli runs list --zap <zap-id> --status error --agent
zapier-pp-cli runs get <run-id> --agent
zapier-pp-cli diagnose <zap-id> --limit 5 --agent
```

### History coverage and step data

`runs list` returns 25 runs by default; `--limit` accepts a page size of 1–100.
For the next page use the returned
`meta.pagination.next_offset` with `--offset`. Use `--all` to read all retained
matching runs after the selected offset:

```bash
zapier-pp-cli runs list --limit 25 --offset 25 --agent --no-learn
zapier-pp-cli runs list --zap <zap-id> --status error --all --agent --no-learn
```

Under `--agent`, `results` remains an array. `meta.pagination` reports the
offset, returned count, total count, and whether more results exist. These
counts describe the API's retained history, not every run ever executed.
History can change while pages are being fetched. Overlapping pages fail with
a retry message rather than silently duplicating runs. A missing or malformed
reporting response is an error, not an empty history.

`diagnose --limit` limits failed runs inspected. “No failed runs found” means
none were found in that scope; it does not prove the Zap works. Step inputs,
outputs and errors are the fields supplied by Zapier. An absent body, header,
or output field does not prove no HTTP request or response existed. Run data
can include personal information, tokens, and internal URLs. Review and redact
it before sharing transcripts or exporting reports.

## Release acceptance

Automated release gates cover Go tests, read-only enforcement, fixture-based
login validation, native Windows installer behavior and credential permission
checks. They do not replace a real user login or agent-host test. Use the
[manual acceptance checklist](docs/production-acceptance.md) before treating a
new release as approved for your team.

## Reconnect or uninstall

To reconnect, run `zapier-pp-cli auth browser` again. Treat it as a new
connection: run only `session --agent --no-learn`, show the account identity,
and wait for confirmation before any other live read.

To remove the connection and MCP registration:

```bash
zapier-pp-cli auth logout
claude mcp remove --scope user zapier
codex mcp remove zapier
```

Run only the MCP removal command for each installed host. Then remove
`zapier-pp-cli` and `zapier-pp-mcp` from `$HOME/.local/bin` on macOS/Linux, or
remove `zapier-pp-cli.exe` and `zapier-pp-mcp.exe` from
`%LOCALAPPDATA%\Microsoft\WindowsApps` on Windows. If installation used a custom
directory, remove the two binaries from that directory instead. Remove an empty
installer-added PATH entry if desired.

## Troubleshooting

- Release download fails: confirm GitHub is reachable and the requested
  release tag exists. For this private repository, sign into GitHub CLI with
  an account that has access; a browser login alone does not authenticate the
  installer.
- The command is missing after install: use the absolute binary path printed by
  the installer and report the PATH problem as an installer bug.
- Claude or Codex cannot find the MCP server: register the absolute
  `zapier-pp-mcp` path printed by the installer, then restart the host.
- The Zapier session expired: rerun `auth browser`. Never debug authentication by
  printing, exporting, or manually copying a credential.

## Contributor source build

This section is for people changing the source. Normal installation uses the
prebuilt release and does not require Go.

```bash
go build -o ./bin/zapier-pp-cli ./cmd/zapier-pp-cli
go build -o ./bin/zapier-pp-mcp ./cmd/zapier-pp-mcp
go test ./...
```

Generated-tree changes need a matching durable record under
`.printing-press-patches/`. Do not hand-edit the Printing Press release ledger.
