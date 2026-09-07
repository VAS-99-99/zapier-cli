# Windows setup: Claude Code and Cowork

This guide is for Windows x64. Run commands yourself in PowerShell, one at a
time, and stop if a command fails. Install Git first. Claude Code also needs
the `claude` command installed; Cowork needs the Claude Desktop app.

| Installation | CLI and MCP executables | Claude Code skill | Cowork skill and MCP connection |
| --- | --- | --- | --- |
| `install.ps1` | Yes | No | No |
| `install.ps1 -Agent Claude` | Yes | Yes | No |
| Upload the Windows Cowork ZIP in the app | Bundled copies | No | Yes |

Both apps reuse the CLI's saved Zapier login under the same Windows user.
Installing the Claude Code skill does not register its MCP server. Uploading
the Cowork ZIP declares the app's MCP connection; it needs no `claude mcp add`.

## Option A: Claude Desktop Cowork

### 1. Install the CLI

```powershell
cd $env:USERPROFILE
git clone https://github.com/VAS-99-99/zapier-cli.git
cd zapier-cli
powershell -NoProfile -ExecutionPolicy Bypass -File .\install.ps1
$env:Path = "$env:LOCALAPPDATA\Microsoft\WindowsApps;$env:Path"
```

If you already cloned the repository, open that folder and start with the
installer command. The installer downloads a checksummed release; no Go,
Node.js, Python, or source build is required.

### 2. Sign in and verify the account

```powershell
zapier-pp-cli auth browser
```

Sign in personally in the browser. Wait for `Connected to Zapier`; the browser
closes automatically after success. Then run:

```powershell
zapier-pp-cli session --agent --no-learn
```

Check the returned email and account ID before inspecting Zaps. If this CLI
already has the correct saved login, skip browser authentication.

### 3. Install the skill and MCP through the app

1. Open [GitHub Releases](https://github.com/VAS-99-99/zapier-cli/releases).
   Choose the newest release, including release candidates.
2. Download **`zapier-cowork_windows_x86_64.zip`** and leave it zipped.
   This is different from the CLI archive and GitHub's source-code ZIP.
3. In Claude Desktop's **chat side**, open
   **Customize → Plugins → Add → Upload a plugin**.
4. Select the ZIP and enable **Zapier cowork**. Approve the local MCP server
   if the app asks. There may be no separate Connect button; the connector can
   appear as **Runs locally in sessions**.
5. Start a fresh Cowork task. Verify that the Zapier skill and `session_check`
   tool are available. Call only that tool, check the account, and confirm it
   before asking Claude to inspect Zaps.

The ZIP supplies both the Cowork skill and its native MCP server. Do not run
`claude mcp add` for this app path. Do not use **Add marketplace** to upload it.

If only the skill appears, fully quit Claude from the system tray, reopen it,
allow the plugin download to finish, and start a new task. An installed skill
or a **Runs locally in sessions** label does not prove the tools are connected.
Tools may have a `remote-devices` prefix. If still missing, check Claude's MCP
logs; do not repeatedly sign in or assume missing tools prove a cloud-only
limitation.

For scheduled audits, first complete an interactive check, then follow the
[scheduling guide](teammate-setup.md#schedule-the-daily-check). Verify an actual
scheduled run separately; an interactive success does not establish that.

## Option B: Claude Code

### 1. Install the CLI and Claude Code skill

Confirm `claude --version` works, then run:

```powershell
cd $env:USERPROFILE
git clone https://github.com/VAS-99-99/zapier-cli.git
cd zapier-cli
powershell -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 -Agent Claude
$env:Path = "$env:LOCALAPPDATA\Microsoft\WindowsApps;$env:Path"
```

If already cloned, open that folder and start with the installer command.
If you installed the plain CLI earlier, rerunning the installer with
`-Agent Claude` adds the skill plugin and preserves the saved Zapier login.
The plugin is named `zapier-read-only`; its skill is `zapier`.

### 2. Sign in

Use the [sign-in and account check above](#2-sign-in-and-verify-the-account).
Do not sign in again if the same Windows user already has the correct saved
CLI connection from the Cowork setup.

### 3. Register the MCP connection

```powershell
claude mcp add --transport stdio --scope user zapier -- "$env:LOCALAPPDATA\Microsoft\WindowsApps\zapier-pp-mcp.exe" --transport stdio
claude mcp list
```

If `zapier` is already registered, inspect it with `claude mcp get zapier`
instead of adding a duplicate. The executable should be the path above.
User scope makes this connection available across local Claude Code projects
under this Windows user. See [Claude Code's MCP documentation](https://code.claude.com/docs/en/mcp).

### 4. Verify in Claude Code

Restart Claude Code to load the skill and MCP connection. Open `/mcp` and check
that `zapier` is connected. Ask Claude to use the Zapier MCP `session_check`
tool and report the exact account, then confirm it before requesting a Zap
list. The skill can also use the CLI directly; explicitly request MCP when
testing the MCP connection.

## What has been verified

On Windows, release `v0.1.0-rc.7` completed browser login and a live CLI account
check. Claude Desktop subsequently logged a successful connection to the
uploaded Cowork plugin and announced 21 tools; the user reported Cowork working.
The earlier activation delay's exact cause was not established.

The Claude Code instructions match `install.ps1` and Claude Code's documented
stdio registration. The installed Windows MCP executable passed an independent
initialize/tools-list check. A fresh end-to-end Claude Code session using this
entire guide has not been verified here.
