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
Clone https://github.com/VAS-99-99/zapier-cli and follow the repository instructions. Read CLAUDE.md, AGENTS.md, and README.md before taking action. This is my public source repository for an unofficial, remotely read-only Zapier inspection tool.

Audit the source, the read-only network boundary, the release workflow, and the published SHA256SUMS before installing anything. Install only the matching prebuilt GitHub Release. The installer only writes local CLI and MCP files; it does not authenticate or contact Zapier. Do not install Go or build from source.

You are not being asked to authenticate, capture a session, inspect browser storage, or handle a cookie or token. After installation, stop and tell me to run `zapier-pp-cli auth browser` myself in my own terminal. I will complete the visible Zapier login outside Claude.

Only after I explicitly reply `connected`, run `zapier-pp-cli session --agent --no-learn`. Show me the exact connected account and stop for confirmation. Before that confirmation, do not run doctor, list Zaps, inspect runs, or make any other Zapier request. Never perform a remote Zapier write.
```

## Supported systems

The public GitHub Release contains these checksummed archives:

| System | Release asset |
| --- | --- |
| Windows x64 | `zapier-cli_windows_x86_64.zip` |
| macOS Apple Silicon | `zapier-cli_darwin_arm64.tar.gz` |
| macOS Intel | `zapier-cli_darwin_x86_64.tar.gz` |
| Linux x64 | `zapier-cli_linux_x86_64.tar.gz` |

Each archive contains `zapier-pp-cli`, `zapier-pp-mcp`, this README, the agent
skill, and the license. `SHA256SUMS` covers every archive. The current installer
uses an authenticated `gh` session to download the release. It never embeds or
saves a GitHub token itself.

## Install manually

Clone the public repository after authenticating GitHub CLI:

```bash
gh auth login
gh repo clone VAS-99-99/zapier-cli
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

The installers select the current system archive, download it with `gh`, verify
its checksum before extraction, and install both binaries without administrator
rights. The default directories are `$HOME/.local/bin` on macOS/Linux and
`%LOCALAPPDATA%\Programs\ZapierCLI` on Windows. They add the directory to the
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
saves only the Zapier-scoped session to protected local credential storage and
does not print it or create an export file.

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
`%LOCALAPPDATA%\Programs\ZapierCLI` on Windows. If installation used a custom
directory, remove the two binaries from that directory instead. Remove an empty
installer-added PATH entry if desired.

## Troubleshooting

- Private release download fails: run `gh auth status`, then confirm the signed-in
  GitHub account has been invited to `VAS-99-99/zapier-cli`.
- The command is missing after install: open a new terminal, or use the absolute
  binary path printed by the installer.
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
