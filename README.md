# Zapier read-only CLI

Inspect Zaps and failed runs without changing anything in Zapier.

## macOS installation

Run this yourself in Terminal:

```bash
git clone https://github.com/VAS-99-99/zapier-cli.git && cd zapier-cli
./install.sh --agent claude
export PATH="$HOME/.local/bin:$PATH"
zapier-pp-cli auth browser
```

The last command opens Zapier in your browser. Complete login and wait for the
`Connected to Zapier` message.

## Windows installation

On Windows x64, install Git if needed, then run these commands yourself in
PowerShell, one at a time. Stop if a command fails.

```powershell
git clone https://github.com/VAS-99-99/zapier-cli.git
cd zapier-cli
powershell -NoProfile -ExecutionPolicy Bypass -File .\install.ps1
$env:Path = "$env:LOCALAPPDATA\Microsoft\WindowsApps;$env:Path"
zapier-pp-cli auth browser
```

If you already cloned this repository, open that folder in PowerShell and
start with the installer command. Complete the browser login and wait for
`Connected to Zapier`. Both the CLI and MCP executable are installed.

For Claude Code, add `-Agent Claude` to the installer command; for Codex, add
`-Agent Codex`. The selected agent's command must already be installed.
Restart the agent after installation to load its skill.

For Claude Desktop Cowork, follow the ZIP upload steps below and choose
`zapier-cowork_windows_x86_64.zip`.

## Switch Zapier accounts

Run this yourself in Terminal on Mac or PowerShell on Windows:

```bash
zapier-pp-cli auth logout
zapier-pp-cli auth browser
zapier-pp-cli session --agent --no-learn
```

If the browser opens the previous Zapier account, sign out of Zapier there and
sign in to the intended account. Cowork uses the same saved login, so start a
new Cowork task afterward. Update any scheduled task with the new email and
account ID before enabling it.

## Cowork installation

1. Open the [latest release](https://github.com/VAS-99-99/zapier-cli/releases/latest).
2. Download the matching ZIP and leave it zipped:

   | Computer | ZIP |
   | --- | --- |
   | Apple Silicon Mac | `zapier-cowork_darwin_arm64.zip` |
   | Intel Mac | `zapier-cowork_darwin_x86_64.zip` |
   | Windows x64 | `zapier-cowork_windows_x86_64.zip` |

3. In Claude Desktop's **chat side**, click **Customize → Plugins → Add → Upload a plugin**.
4. Choose the ZIP file itself. Do not extract it or choose a folder.
5. Enable the plugin and approve its local MCP server.

Do not use **Add marketplace**. It is not the ZIP uploader and currently does
not work for this repository.

## Daily scheduled check

After Cowork works interactively, follow [the scheduling guide](docs/teammate-setup.md#schedule-the-daily-check).

## More detail

- [Teammate setup](docs/teammate-setup.md)
- [Cowork troubleshooting](docs/cowork.md)
- [Contributor guide](AGENTS.md)
