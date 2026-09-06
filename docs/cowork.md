# Local Cowork connection

The `zapier-cowork` plugin bundles the native release CLI and MCP executable
with a Cowork-specific skill. Its MCP process runs on the host, not in the
Linux shell. Both binaries live together under `server/`, so subprocess lookup
does not depend on PATH. Launch uses stdio explicitly, with no listening port,
tunnel, public server, startup login, or Zapier request during tool discovery.
The plugin makes the skill available; automatic skill selection still depends
on the model. Use the README's explicit prompt for the first check.

The ZIP package is separate from `zapier-read-only`, the terminal-based Claude
Code/Codex plugin. It does not alter either host's existing settings or
marketplace. Upload it in local Cowork rather than editing
`claude_desktop_config.json` or adding a web connector.

The repository marketplace also offers `zapier-cowork-macos` and
`zapier-cowork-windows`. These contain the same skill but start the MCP
executable installed by the normal release installer at its default location:
`$HOME/.local/bin/zapier-pp-mcp` on Mac and
`%LOCALAPPDATA%/Microsoft/WindowsApps/zapier-pp-mcp.exe` on Windows.
They do not bundle binaries, download executables at startup, or depend on
the desktop app inheriting a terminal PATH. Custom installation directories
are not supported by these entries; use the ZIP instead.
Follow [teammate setup](teammate-setup.md) for the repository route. Enable
only one Cowork variant at a time; disabling a duplicate does not remove login.

## Install and accept

Follow [the three-step README setup](../README.md#local-claude-cowork).
The matching prebuilt CLI must already be connected under the same OS user.
For a first installation, use the README's normal binary installation without
an agent plugin flag, then personally run `zapier-pp-cli auth browser`.

The Cowork ZIP contains executable code from this unofficial project. Review
and approve it as such. A checksum checks download integrity, not publisher
trust. Do not disable host safeguards if your organization blocks local MCP.

Acceptance in your actual Cowork installation:

1. Upload and enable the correct platform package. Verify its Zapier skill and
   `session_check` MCP tool appear. Merely seeing the skill is not enough.
2. Ask only for the connected account. Verify the email and account ID before
   approving another read. No browser should open if the saved login is valid.
3. After confirmation, inspect existing Zaps and one historical run. Compare
   the returned step details to Zapier. Never send a webhook to create test data.
4. Start a fresh local task outside the repository. Repeat the account check.
   It should reuse the saved session and stop for confirmation again.

Until those UI and live checks pass, automated package/MCP tests establish
plumbing only, not end-to-end acceptance in your Cowork build.

## Verified Mac setup, 2026-09-06

Evidence below comes from the user's Claude Desktop screenshots and pasted
Cowork results, plus local archive checks. It is not a claim that every
platform or scheduled execution path has passed acceptance.

| Check | Observed result |
| --- | --- |
| Mac packages | Apple Silicon and Intel ZIP integrity checks passed; source macOS and Windows release archive checksums matched. Local Mac plugin manifests report `0.1.0-rc.6`. |
| Cowork installation on Mac | The user uploaded the plugin and Cowork reported both `zapier-cowork:zapier` and its `session_check` MCP tool available. |
| MCP response before login | `session_check` returned successfully with `is_logged_in: false` and a null account ID. Transport worked; authentication did not yet. |
| Login recovery | After being directed to personally run `zapier-pp-cli auth browser`, the user reported success. A later pasted result showed a logged-in session with email, user ID, account ID, and owner role. Identifiers are omitted here. |
| Account checkpoint | Cowork stopped after session identity and requested confirmation, without proceeding to run history. |
| Scheduled-task form | Screenshots show Daily, 09:00, permission choices, and a `Require this computer (Claude Desktop (macOS))` toggle. |

Still unverified: completed Zap/run-history inspection, failed-step diagnosis
against live runs, a confirmed device binding and successful scheduled test,
an unattended daily report, DST behavior, and runtime installation on Intel
Mac or Windows. The later successful session report did not include evidence
establishing that it came from a device-bound scheduled execution.

### Finding the upload file

`Customize` appeared in the left sidebar below `Dispatch`. The `Add
marketplace` dialog offered repository sources, not a ZIP uploader; return to
the Plugins upload flow instead. Exact menu labels can vary by app version.

For Apple Silicon, select `zapier-cowork_darwin_arm64.zip`; for Intel Mac,
select `zapier-cowork_darwin_x86_64.zip`. `darwin` means macOS. Upload the ZIP
without extracting it. A folder with the same base name is not the upload
file. Finder may truncate the middle of the filename; check the `.zip` suffix.
Do not use a CLI `.tar.gz` archive or GitHub's source-code ZIP.

### Scheduling from the Mac desktop form

Keep the earlier cloud-only task paused while creating a local replacement.
In `Scheduled`, open `New task` and `Set up manually`:

1. Enter a name and instructions to inspect failures over the previous 24
   hours, reporting coverage and stopping if local MCP or login is unavailable.
2. Choose `Daily` at `09:00`. The observed form did not display a timezone;
   verify the saved next-run time against Sofia time. DST handling has not
   been demonstrated. A five-field cron expression alone does not establish
   the scheduler's timezone capabilities.
3. Turn **Require this computer ON** before saving. It was off in the user's
   draft screenshot. The form states that this requires the computer to be
   awake. A saved task name or a successful interactive session does not prove
   that the scheduled task can reach local MCP.
4. Choose app permissions deliberately. `Skip all approvals` allows automatic
   tool execution; `Manually approve` adds app approval prompts. Neither
   supplies account authorization. For unattended checks, use the explicit
   exact-account authorization prompt in [teammate setup](teammate-setup.md).
5. Save, complete any device approval the app actually presents, then use the
   task's run-now action to test `session_check`. Confirm the returned account
   before testing the historical reads.

The updated source skill supports an explicitly authorized scheduled check
only when the live session matches both the user's specified email and account
ID. Default interactive checks still stop for confirmation. This is an agent
instruction, not an account restriction enforced by the MCP binary. Refresh
the repository plugin or replace an older ZIP to load the updated skill.
A successful unattended scheduled report remains unverified.

The previous tool-created task reportedly returned
`not bound: no_signed_approval` and was subsequently paused. Its available
tools did not expose a binding operation. The desktop screenshot then revealed
the computer toggle; do not infer that the app lacks a feature merely because
the current MCP tools do not expose it.

## If it cannot connect

- **Skill present, MCP tools absent:** check that the task is local, the plugin
  is enabled, and local MCP is permitted by the app and organization. Restart
  the task after approval. Cloud Cowork cannot run this local connection.
- **Wrong executable format:** install the package for the host OS and CPU,
  not a Linux package selected from Cowork's shell `uname` output.
- **Permission denied starting the executable:** report the exact host error.
  Do not turn off macOS Gatekeeper or run a broad `chmod`/quarantine removal.
- **Expired or missing session:** personally run the installed CLI's
  `auth browser` on the host. No cookies belong in the plugin, chat, connected
  folder, VM, or agent settings. Recheck and confirm the account afterward.
- **Only the terminal CLI is installed:** its location in your home directory
  does not make it reachable from the Cowork shell. Install this plugin.

Local reads reuse the CLI's normal credentials/configuration and may write
its normal local cache/audit state. The learning loop is disabled by the
package. Zapier reads and selected results are still processed by Claude;
"local" does not mean run data stays out of the conversation service.

## Update or remove

Download the new platform-matching Cowork ZIP and verify it with that release's
`COWORK_SHA256SUMS`. Update or replace `zapier-cowork` through Cowork's plugin
UI, then start a fresh task and repeat the account checkpoint. Updating only
the terminal CLI does not update the copies inside the plugin.

Disable or uninstall this plugin in Cowork to remove its tools. This does not
log out the terminal CLI or remove shared credentials. Leave the saved login
alone unless the user explicitly requests logout. No background service or
Claude Code registration needs removing.

## Maintainer packaging and tests

`scripts/sync-cowork-marketplace.py` generates the two repository plugins from
the ZIP plugin's canonical skill and manifest. Run it after editing that source;
`--check` detects drift without writing. Do not edit the generated skill copies.
`python3 scripts/tests/cowork_marketplace_test.py` checks catalog paths and
cache-contained files. To verify actual Mac marketplace installation and MCP
loading against isolated credentials and a synthetic model endpoint:

```bash
python3 scripts/tests/claude_plugin_discovery_test.py plugins/zapier-cowork-macos dist/cowork/zapier-cowork_darwin_arm64/server
```

This passed locally on 2026-09-06. It installs the repository plugin in a
temporary Claude profile and copies the provided native release binaries to a
temporary default install directory. It does not verify Cowork's repository
UI, Windows execution, or GitHub access. Refresh versions deliberately when
publishing plugin updates; native plugin caches use manifest versions.

`scripts/package-cowork.py` consumes already-built release archives. It verifies
their SHA-256 entries before reading them, rejects unsafe archives, and copies
only the two binaries plus maintained plugin files and the license. It never
reads user credentials, runs a binary, or builds Go. Python is a maintainer
dependency only. For example, after downloading the three macOS/Windows CLI
archives and `SHA256SUMS` into `dist/cli-release`:

```bash
python3 scripts/package-cowork.py --release-dir dist/cli-release --tag v0.1.0-rc.6 --output-dir dist/cowork
python3 scripts/tests/cowork_package_test.py
```

The tag must match the downloaded release. The output contains three plugin
ZIPs and `COWORK_SHA256SUMS`. Normal CLI checksums remain unchanged. Releases
created by the updated workflow include these additional assets. The rc.6 CLI
was originally published without Cowork assets; its companion ZIPs are packaged
later from those unchanged, checksummed release binaries and the current
Cowork skill. The CLI archives and their checksums are not replaced.

On the matching host, extract a generated ZIP preserving executable modes and
run `python3 scripts/tests/cowork_mcp_smoke.py /path/to/extracted/plugin`.
It uses an isolated temporary home, synthetic credentials, and a loopback API.
It tests startup, schemas, session identity, sibling CLI discovery without
PATH, run inspection, diagnosis, and expired-session errors. It neither uses
an actual Zapier account nor certifies the Cowork upload UI.

With Claude Code available, run
`python3 scripts/tests/claude_plugin_discovery_test.py /path/to/extracted/plugin`
to check that the native plugin loader advertises both the skill and MCP tools.
This uses a temporary Claude profile and a synthetic model endpoint. It checks
skill advertisement and explicitly invokes Skill to verify that the full
instructions load. The fixture responds from request history so an API retry
cannot consume its one tool response. It does
not change the user's Claude Code configuration or replace Cowork UI acceptance.

The package follows Anthropic's [plugin MCP configuration](https://code.claude.com/docs/en/plugins-reference#mcp-servers)
and [local Cowork architecture](https://support.claude.com/en/articles/14479288-claude-cowork-architecture-overview).
See [plugin installation](https://support.claude.com/en/articles/13837440-use-plugins-in-claude)
for the current upload UI. Local plugin support and cloud connectors are
different mechanisms.
