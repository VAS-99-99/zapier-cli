# Zapier read-only CLI

`zapier-pp-cli` lets Claude, Codex, and people inspect a connected Zapier
account. It checks the account session, lists and searches Zaps, opens historical
runs, and pinpoints failed steps. It cannot change anything in Zapier.

Normal users do not need Go, Node.js, npm, Playwright, `agent-browser`, a browser
extension, or a Zapier developer token. The release installers provide the CLI
and MCP binaries. `auth browser` manages its pinned browser helper and browser
inside the CLI.

## Start here

Choose a route and copy the linked prompt into **Claude Code or Codex running
on the teammate's computer**. These prompts install the CLI itself, not just
a skill. Cowork's Linux shell cannot install a Mac or Windows host executable.

| I want to… | Use this prompt | Paste it into |
| --- | --- | --- |
| Install the CLI and agent skill | [Install CLI](#give-this-to-claude-or-codex) | Claude Code or Codex on your computer |
| Set up Cowork through a repository | [Install Cowork from repository](#cowork-setup-prompts) | Claude Code or Codex on your computer |
| Set up Cowork using a ZIP | [Install Cowork from ZIP](#cowork-setup-prompts) | Claude Code or Codex on your computer |
| Update an existing CLI installation | [Update CLI](#updates-for-existing-users) | Claude Code or Codex on your computer |
| Run a daily Cowork check | [Scheduled check](#set-up-a-cowork-scheduled-check) | Cowork's scheduled-task Instructions field |

Expand the matching prompt below and copy only its code block. The standalone
copies are grouped in [prompts/](prompts/README.md).

The agent installs a checksummed release and reuses saved credentials.
The teammate personally completes any GitHub/Zapier sign-in and app approval.
See [teammate setup](docs/teammate-setup.md) for repository installation,
the first working-check prompt, and daily-task instructions.

This repository is private. Teammates need repository access and their own
GitHub sign-in to download it and its release assets.

**After setup:** [try it in a new project](#what-a-new-project-receives).
**Reference:** [manual installation](#install-manually),
[commands](#what-it-can-do), [troubleshooting](#troubleshooting).

## Give this to Claude or Codex

Installs the CLI, MCP executable, and user-level skill for your terminal agent.
Standalone copy: [prompts/install-cli.txt](prompts/install-cli.txt).

<details>
<summary>Install CLI and agent skill — expand to copy</summary>

```text
Quick install our team's unofficial, read-only Zapier CLI from
https://github.com/VAS-99-99/zapier-cli. I authorize downloading and installing it. From a parent
directory, use only `./zapier-cli`: if it exists, verify that it is this repository and reuse it,
preserving local changes. If it does not exist, clone to `./zapier-cli`. If the current directory is
already this clone, use it. Do not search elsewhere for a clone or blindly clone over an existing
directory. Safely update an existing clone without discarding edits. Preserve saved Zapier
credentials. Read CLAUDE.md and run the installer from the repository directory.

Install the latest prebuilt GitHub Release for this computer. Do not install Go or build from
source. The installers select the right archive automatically: macOS Apple Silicon uses
`zapier-cli_darwin_arm64.tar.gz`, macOS Intel uses `zapier-cli_darwin_x86_64.tar.gz`, and Windows
x64 uses `zapier-cli_windows_x86_64.zip`. These are platform builds of one release, not separate
source repositories. On macOS/Linux, run `./install.sh --agent claude` for Claude or `./install.sh
--agent codex` for Codex. On Windows, run `powershell -ExecutionPolicy Bypass -File .\install.ps1
-Agent Claude` for Claude or `powershell -ExecutionPolicy Bypass -File .\install.ps1 -Agent Codex`
for Codex. The installer performs the checksum and version checks, installs or updates the selected
host's native user-scope plugin, and leaves MCP optional. If repository access needs authentication,
reuse my authenticated GitHub CLI session or guide me through GitHub sign-in. Never ask me to paste
a token.

After installation, confirm `zapier-pp-cli version` works in the current terminal and confirm
`zapier-read-only@vas-zapier-cli` is installed and enabled for the selected host. Fix PATH or
current-command resolution yourself. Do not ask me to open another terminal or edit PATH. Tell me to
start a fresh chat to load the installed skill.

Run only `zapier-pp-cli session --agent --no-learn` to check the existing connection. Show me the
exact connected account and stop for confirmation. If authentication is missing or expired, instead
stop and tell me to run `zapier-pp-cli auth browser` myself in my own terminal. I will complete the
visible Zapier login outside Claude or Codex. Do not authenticate, inspect browser storage, or
handle a cookie or token.

Only after I explicitly reply `connected`, run `zapier-pp-cli session --agent --no-learn`, show the
exact connected account, and stop for confirmation. Before I confirm that account, do not run
doctor, list Zaps, inspect runs, or make any other Zapier request. Never perform a remote Zapier
write.
```

</details>

## Cowork setup prompts

Copy the matching block into a host terminal agent. Each installs the CLI
before preparing Cowork. Complete personal login and desktop approvals when
asked. Use only one Cowork plugin variant at a time.

<details>
<summary>Install Cowork from repository — expand to copy</summary>

Standalone copy: [prompts/install-cowork-repository.txt](prompts/install-cowork-repository.txt).

```text
Set up our team's unofficial, read-only Zapier CLI and prepare Claude Desktop Cowork's Add from
repository installation from https://github.com/VAS-99-99/zapier-cli. I authorize downloading and
installing the prebuilt CLI. This prompt is for Claude Code or Codex running on my Mac or Windows
host. If you are inside Cowork's Linux shell or a remote container, stop and tell me to paste it
into a host terminal agent instead.

Use the current directory if it is already this repository. Otherwise use only ./zapier-cli beneath
the current directory. Verify and reuse an existing clone, preserving local changes and saved Zapier
credentials; clone there if absent. Fetch and fast-forward only when safe. If the directory belongs
to another project or cannot be updated safely, stop and explain. Read CLAUDE.md and
docs/teammate-setup.md. For private GitHub access, reuse my authenticated GitHub CLI session. If
access fails, tell me to sign in to GitHub CLI with an account that has repository access, then
retry after I do so. Never request or print a token.

Detect the actual host OS and CPU and use the repository's supported prebuilt release for it. On
macOS run ./install.sh; on Windows run powershell -ExecutionPolicy Bypass -File .\install.ps1. Use
the default install directory because the repository Cowork plugins refer to that location:
$HOME/.local/bin on macOS or %LOCALAPPDATA%\Microsoft\WindowsApps on Windows. Preserve existing
credentials and unrelated settings. The installer verifies the release archive. Do not install Go,
Node.js, Python, or build from source. If no supported release is available, stop and report the
missing release asset. Verify zapier-pp-cli version in this terminal and confirm zapier-pp-mcp
exists in the default install directory. Report the installed version and whether it is a
prerelease.

Verify that the GitHub repository revision Cowork will fetch actually publishes the
zapier-cowork-macos and zapier-cowork-windows marketplace entries and their referenced plugin files.
Local uncommitted files do not prove teammates can install them. If the entries have not been
published, stop and tell me the maintainer needs to publish the marketplace changes, or offer the
separate checksummed ZIP route from docs/teammate-setup.md. Do not publish changes yourself.

Give me these Claude Desktop steps: open Customize > Plugins, choose Add marketplace > Add from a
repository, and enter https://github.com/VAS-99-99/zapier-cli.git. Install and enable
zapier-cowork-macos on macOS or zapier-cowork-windows on Windows. I will approve the app's
installation and local MCP server. Explain that GitHub CLI authentication does not prove Claude
Desktop can access a private repository; if the app cannot fetch it, report its exact error and
offer the verified ZIP route. This project remains unofficial; this is the app's supported
repository install flow, not an Anthropic endorsement.

Run only zapier-pp-cli session --agent --no-learn to check the saved connection. Show the exact
returned account identity and stop for my confirmation before any other Zapier read. If logged out
or expired, stop and tell me to personally run zapier-pp-cli auth browser on this host. Do not
authenticate, open a login browser, inspect browser storage, or handle cookies. After I report
connected, run only the session check again, show the exact account, and wait for confirmation.

Give me the first Cowork test prompt from docs/teammate-setup.md to paste into a fresh local Cowork
task after installation. Summarize what you actually verified and any remaining user steps. Do not
claim Cowork is connected until its own session_check succeeds. Never change anything in Zapier. Do
not create a scheduled task as part of installation.
```

</details>

<details>
<summary>Install Cowork from ZIP — expand to copy</summary>

Standalone copy: [prompts/install-cowork-zip.txt](prompts/install-cowork-zip.txt).

```text
Set up our team's unofficial, read-only Zapier Cowork plugin from
https://github.com/VAS-99-99/zapier-cli on this computer. I authorize downloading and installing the
prebuilt CLI and downloading the matching Cowork plugin ZIP. This prompt is for Claude Code or Codex
running on my Mac or Windows host. If you are inside Cowork's Linux shell or a remote container,
stop and tell me to paste it into a host terminal agent instead.

Use the current directory if it is already this repository. Otherwise use only ./zapier-cli beneath
the current directory. Verify and reuse an existing clone, preserving local changes and saved Zapier
credentials; clone there if absent. Fetch and fast-forward only when safe. If the directory belongs
to another project or cannot be updated safely, stop and explain. Read CLAUDE.md and
docs/teammate-setup.md. For private GitHub access, reuse my authenticated GitHub CLI session. If
access fails, tell me to sign in to GitHub CLI with an account that has repository access, then
retry after I do so. Never request or print a token.

Detect the actual host OS and CPU. Select zapier-cowork_darwin_arm64.zip for Apple Silicon Mac,
zapier-cowork_darwin_x86_64.zip for Intel Mac, or zapier-cowork_windows_x86_64.zip for Windows x64.
Stop on an unsupported host. Inspect GitHub Releases and select the newest non-draft release
containing that ZIP, COWORK_SHA256SUMS, the host's CLI archive, and SHA256SUMS. State its tag and
whether it is a prerelease. If no release has these assets, stop and list the missing assets and
release URL for the maintainer. Do not build or package from source as a fallback.

Install that exact release's prebuilt CLI using ./install.sh --tag TAG on macOS, or powershell
-ExecutionPolicy Bypass -File .\install.ps1 -Tag TAG on Windows, replacing TAG with the selected
tag. The installer verifies the CLI archive. Do not install Go, Node.js, Python, or a separate
browser helper. This is a Cowork setup; an additional terminal-agent plugin is not required. Verify
zapier-pp-cli version in this terminal, fixing current-command resolution if needed.

Download the matching Cowork ZIP and COWORK_SHA256SUMS from the same release to a clearly named
subdirectory of my Downloads folder. Use gh release download for private assets with my existing
GitHub authentication. Verify the ZIP's exact filename entry against its SHA-256 using the host's
built-in tools, such as shasum -a 256 on macOS or Get-FileHash -Algorithm SHA256 on Windows. Stop on
a missing entry or mismatch. Leave the ZIP zipped. Report the release tag, checksum result, and
exact clickable local ZIP path. GitHub's source-code ZIP and the CLI archive are not Cowork upload
files.

Guide me through uploading that ZIP in Claude Desktop's Customize > Plugins upload flow, enabling
zapier-cowork, and approving its local MCP server. The Add marketplace repository dialog does not
accept ZIP uploads. I will handle the app's install approval. Do not claim Cowork is connected until
its own session_check succeeds.

For the saved connection, run only zapier-pp-cli session --agent --no-learn. Show the exact returned
account identity and stop for my confirmation before any other Zapier read. If logged out or
expired, stop and tell me to personally run zapier-pp-cli auth browser on this host. Do not
authenticate, open a login browser, inspect browser storage, or handle cookies. After I report
connected, run only the session check again, show the exact account, and wait for confirmation.

Give me the first Cowork test prompt from docs/teammate-setup.md to paste into a fresh local Cowork
task after upload. Summarize what you actually verified and any remaining user steps. Never change
anything in Zapier. Do not create a scheduled task as part of installation.
```

</details>

## Updates for existing users

Updates the CLI and terminal skill while preserving your saved connection.
For Cowork plugin updates, see [update or remove](docs/cowork.md#update-or-remove).
Standalone copy: [prompts/update-cli.txt](prompts/update-cli.txt).

<details>
<summary>Update CLI and agent skill — expand to copy</summary>

```text
Update our unofficial read-only Zapier CLI and its agent plugin from
https://github.com/VAS-99-99/zapier-cli. I authorize downloading and installing the update. Use the
clone in the current directory, preserving local changes; fetch and fast-forward only if safe,
otherwise use a separate clean clone. Do not search my computer. Read CLAUDE.md. Run the current
installer for your host: Windows .\install.ps1 -Agent Claude or -Agent Codex; macOS/Linux
./install.sh --agent claude or --agent codex. Use the latest checksummed prebuilt release, never Go
or a source build. Preserve existing credentials and unrelated agent settings. Verify the CLI
version in this terminal and confirm zapier-read-only@vas-zapier-cli is installed and enabled. Tell
me to start a fresh chat to load the updated skill. Do not open a browser or reconnect
automatically. Run only zapier-pp-cli session --agent --no-learn, show the exact connected account,
and stop for confirmation. If authentication is missing or expired, instead give me the exact auth
browser command to run personally; never read or print credentials. Never change anything in Zapier.
```

</details>

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

If the repository is private, sign into GitHub CLI with an account that has
repository access. Public access does not require GitHub sign-in.

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
| `--agent claude` or `--agent codex` | `-Agent Claude` or `-Agent Codex` | Also install or update that host's user-level Zapier skill plugin |

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
The CLI closes the browser after validating the session, then saves the
credential. Its success message says "Connected to Zapier". The separate
account check below still requires your confirmation; this is not a failed
login and is not a reason to open another login window.

Leave the sign-in command running while completing SSO or MFA. It allows five
minutes for sign-in; use `zapier-pp-cli auth browser --timeout 10m` if needed.
Run only one login command at a time. If the window closes before success,
read the terminal error before retrying. The CLI checks for a live browser
before polling, and stops when it detects a closed or unavailable window.

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

For discovery in new chats, install the companion skill plugin for your host:

```bash
./install.sh --agent claude
# Or: ./install.sh --agent codex
```

On Windows, use `powershell -ExecutionPolicy Bypass -File .\install.ps1 -Agent Claude`
or replace `Claude` with `Codex`. The selected host command must already be on
PATH and support plugins. Without this option, installation remains binary-only.
The plugin uses the host's native user-level plugin manager and shares one
runtime skill across Claude Code and Codex. It does not register MCP, install
hooks, start background processes, or copy your credentials into agent settings.

Start a fresh chat outside the clone and ask "Check my Zapier runs." The agent
should find the skill, reuse the local connection, show the exact account, and
wait for confirmation. In Claude Code you can explicitly select
`/zapier-read-only:zapier`; in Codex select the installed `zapier` skill if it
isn't picked automatically. Skill discovery is model-dependent, not a guarantee
that every prompt invokes it. Installing the plugin alone does not install the
CLI binary; the combined installer above installs both.

### What a new project receives

The executables live outside your project. On macOS/Linux, installation uses
`$HOME/.local/bin` unless `XDG_BIN_HOME` or `--install-dir` selects another
directory. On Windows it uses `%LOCALAPPDATA%\\Microsoft\\WindowsApps`.
`PATH` makes the command available from different folders. The terminal skill
also knows the default paths when a new host has an older PATH.

The repository clone contains source and setup files; it is not needed for
every inspection. Agent plugins are registered separately through each host's
user-level plugin manager and cached by that host. The Cowork repository
plugin uses the installed MCP executable; a Cowork ZIP carries its own
executables and needs its own update when they change.

The installer registers the terminal plugin for your user, not just this
repository. Its `zapier` skill contains the CLI location, account checkpoint,
runtime command discovery, Zap lookup, run inspection, and coverage rules.
You do not need to copy this repository's `CLAUDE.md` into each project.

| Instructions | Purpose |
| --- | --- |
| [CLAUDE.md](CLAUDE.md) and [setup skill](SKILL.md) | Guide an agent installing from this repository |
| [Terminal runtime skill](plugins/zapier-read-only/skills/zapier/SKILL.md) | Installed by `--agent claude` or `--agent codex`; available in fresh projects |
| [Cowork runtime skill](cowork/zapier-cowork/skills/zapier/SKILL.md) | Included in the Cowork plugins; uses host MCP, not shell commands |

After setup, start a new chat in an unrelated project and paste this, replacing
the bracketed text with the Zap's name or URL:

```text
Go check if this Zap has any issues: [Zap name or URL]. Inspect the previous
24 hours using the installed Zapier inspection skill and its CLI or local MCP.
Reuse my saved login. Check the connected account first. Do not modify Zapier.
```

The expected first result is a real session check showing the account, not a
request to install again. Confirm the account, then the agent should resolve
the Zap, inspect history, and explain failures with coverage limits. Automatic
selection depends on the model. If it misses the skill, explicitly select
`/zapier-read-only:zapier` in Claude Code, the installed `zapier` skill in Codex,
or the enabled Zapier Cowork plugin's skill in Cowork. Do not claim onboarding
passed until this fresh-project check succeeds in the intended host.

### Optional MCP connection

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

### Local Claude Cowork

Cowork needs its own plugin. Installing the Claude Code skill does not make a
macOS or Windows executable available in Cowork's Linux shell.

For **Add from a repository**, use the [repository setup prompt](prompts/install-cowork-repository.txt).
It installs the prebuilt CLI first, then guides you to add
`https://github.com/VAS-99-99/zapier-cli.git` and choose `zapier-cowork-macos`
or `zapier-cowork-windows`. This is Claude's supported marketplace flow for
our unofficial plugin. The entries must be published to GitHub before remote
installation works. Use one Cowork route at a time to avoid duplicate tools.

For the self-contained **ZIP route**:

1. Get the matching **`zapier-cowork_*.zip`** package from your maintainer or a
   release that includes Cowork assets: `darwin_arm64` for Apple Silicon,
   `darwin_x86_64` for Intel Mac, or `windows_x86_64` for Windows x64.
   Use `COWORK_SHA256SUMS` to verify the download. The CLI archives and their
   `SHA256SUMS` are separate assets.
2. In Claude Desktop, open **Cowork → Customize → Plugins** and upload the
   custom plugin ZIP. Enable it and approve its local MCP server when asked.
3. Start a fresh **local** Cowork task and paste:

   ```text
   Use the zapier-cowork plugin's Zapier skill and local MCP tools. Call only session_check, show my connected email and account ID, and stop for my confirmation. Reuse my saved login. Do not run shell commands, open a login browser, or change anything in Zapier.
   ```

The package includes the native CLI and MCP companion. No Go, Node.js, Python,
terminal PATH change, or credential copying is needed to run it. It uses the
same OS user's saved login; if missing or expired, personally run
`zapier-pp-cli auth browser` using the normal installed CLI, then retry the
account check. A new Cowork task is not a reason to log in again.

This requires **local MCP access**. The plugin does not create schedules;
unattended reads require explicit authorization for an exact account.
See [Cowork setup and
acceptance](docs/cowork.md) for troubleshooting, updates, and packaging.
The [verified Mac setup and scheduling notes](docs/cowork.md#verified-mac-setup-2026-09-06)
record successful plugin discovery and login checks, the desktop controls,
and the remaining acceptance gaps.

## Set up a Cowork scheduled check

First complete the Cowork plugin installation and its session test. In Claude
Desktop, open **Scheduled → New task → Set up manually**:

1. Enter a name such as `Daily Zapier failed-run check`.
2. Choose **Daily**, **09:00**, and **Europe/Sofia** if offered. Verify the saved
   next-run time; DST handling has not been verified.
3. Turn **Require this computer ON**. Keep the computer awake for the run.
4. For an unattended task, choose **Skip all approvals** and explicitly
   authorize the exact account in the instructions below. App permissions
   alone do not supply account authorization.
5. Paste the completed prompt, save, and complete any device approval shown.
   Keep an older cloud-only task paused to avoid duplicates.
6. Use **Run now** to test local MCP and a completed report. To test the timer,
   set a near-future time and wait without clicking Run now.

Replace both placeholders with the Zapier identity you personally confirmed.
Do not use your Claude login email unless it is also your Zapier email.
Standalone copy: [prompts/scheduled-check.txt](prompts/scheduled-check.txt).

<details>
<summary>Scheduled check — expand to copy, then fill in your account</summary>

```text
Use the installed Zapier Cowork skill and local MCP tools to check for failed Zapier runs
in the previous 24 hours.

For this scheduled task only, I explicitly authorize unattended read-only
checks of my personally confirmed account:
Email: YOUR_CONFIRMED_EMAIL
Account ID: YOUR_CONFIRMED_ACCOUNT_ID

First verify session_check is available. If unavailable, report "Local Zapier
MCP unavailable" and stop. Call session_check before any other Zapier read.
Continue automatically only if logged in and the returned email and account
ID both exactly match the authorized values above. If either is missing,
login has expired, a placeholder remains, or either value differs, report the
problem and stop. Do not authenticate or switch accounts.

After a successful match, inspect run history across my Zaps for the previous
24 hours. Summarize failures with Zap name, time in Europe/Sofia, failed step,
error, suggested action, and run link when returned. State the Zaps and history
pages inspected and any coverage limits. If no failures are found, say
"No failures found in the inspected scope." Omit sensitive step inputs and
outputs.

Reuse saved credentials. Do not run shell commands, authenticate, open a
browser, retry runs, edit Zaps, send webhooks, or change anything in Zapier.
```

</details>

A new run proves the timer fired; a completed report proves inspection worked.
If the task still asks for confirmation after an exact match, check that its
saved prompt replaced the old confirmation gate and update the plugin to load
the current skill. Account mismatch or expired login must still stop the task.
See [the full teammate guide](docs/teammate-setup.md) for first-session prompts
and [verification status](docs/cowork.md#verified-mac-setup-2026-09-06).

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

## Repository layout

| Location | Purpose |
| --- | --- |
| [prompts/](prompts/README.md) | Copy-paste setup, update, and scheduled-task prompts |
| [docs/](docs/README.md) | User guides, verification notes, and contributor plans |
| `plugins/` | Installable terminal and Cowork repository plugins |
| `cowork/` | Source template for the bundled Cowork ZIP plugin |
| `cmd/`, `internal/` | Go executables and implementation |
| `scripts/` | Packaging, plugin generation, and verification |
| `AGENTS.md`, `CLAUDE.md`, `SKILL.md` | Agent rules and repository setup instructions |

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
