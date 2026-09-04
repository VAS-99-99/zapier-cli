# Zapier read-only CLI

`zapier-pp-cli` lets Claude, Codex, and people inspect a connected Zapier
account. It checks the account session, lists and searches Zaps, opens historical
runs, and pinpoints failed steps. It cannot change anything in Zapier.

Normal users do not need Go, Node.js, npm, Playwright, `agent-browser`, a browser
extension, or a Zapier developer token. The release installers provide the CLI
and MCP binaries. `auth browser` manages its pinned browser helper and browser
inside the CLI.

## Give this to your agent

Copy this whole block into Claude or Codex:

```text
Install the private Zapier read-only CLI from
https://github.com/VAS-99-99/zapier-cli and connect my Zapier account.

Complete these five stages in order:

1. ACCESS. Make sure the official GitHub CLI is installed. Check
   `gh auth status` and confirm my GitHub account can read
   `VAS-99-99/zapier-cli`. Guide me through `gh auth login` in the browser if
   needed. Stop if my account has not been invited. Clone the repository with
   `gh repo clone VAS-99-99/zapier-cli`, or safely update an existing clone.

2. INSTALL. Detect my OS and CPU, then run the repository installer. On macOS
   or Linux run `./install.sh`. On Windows PowerShell run
   `powershell -ExecutionPolicy Bypass -File .\install.ps1`. The installer must
   download the matching private GitHub Release, verify SHA-256, and install
   `zapier-pp-cli` and `zapier-pp-mcp`. Normal setup must not install Go,
   Node.js, npm, Playwright, agent-browser, or a browser extension. Verify
   `zapier-pp-cli --version`, then run the exact MCP registration command
   printed by the installer for this host.

3. CONNECT. Run `zapier-pp-cli auth browser`. Let the CLI manage its pinned
   browser helper and browser. Have me sign in to Zapier in the visible window,
   then let the command save the Zapier-scoped session in protected local
   storage and close the window. Never ask me to find, copy, paste, reveal, or
   transmit a cookie or token. Never put one in chat, command arguments, logs,
   screenshots, source control, or an MCP configuration, and never manually
   copy or save one in an arbitrary file. Protected storage managed by the CLI
   is the only supported destination.

4. VERIFY, THEN STOP. The first live Zapier call after `auth browser` must be
   only `zapier-pp-cli session --agent --no-learn`. Show me the exact connected
   account identity and stop. Do not run `doctor`, list Zaps, or read any other
   account data until I confirm that this is the correct account.

5. USE AFTER CONFIRMATION. After I confirm the account, run
   `zapier-pp-cli doctor --json` and the bounded read-only smoke test
   `zapier-pp-cli zaps list --limit 3 --agent --no-learn`. Then use only the
   documented read commands.

Read and obey AGENTS.md. This product is remotely read only. Never create,
edit, enable, disable, rename, publish, replay, cancel, or delete anything in
Zapier. Never add a generic outbound POST, webhook delivery, remote feedback,
or a general-purpose browser MCP. Local installation, protected credential
storage, local files, and the local learning store are allowed.
```

## Supported systems

The private GitHub Release contains these checksummed archives:

| System | Release asset |
| --- | --- |
| Windows x64 | `zapier-cli_windows_x86_64.zip` |
| macOS Apple Silicon | `zapier-cli_darwin_arm64.tar.gz` |
| macOS Intel | `zapier-cli_darwin_x86_64.tar.gz` |
| Linux x64 | `zapier-cli_linux_x86_64.tar.gz` |

Each archive contains `zapier-pp-cli`, `zapier-pp-mcp`, this README, the agent
skill, and the license. `SHA256SUMS` covers every archive. Because the repository
is private, the installer requires a GitHub account that has access and an
authenticated `gh` session. It never embeds or saves a GitHub token itself.

## Install manually

Clone the private repository after authenticating GitHub CLI:

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
needed and opens a dedicated visible window. Sign in to Zapier there. The CLI
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
