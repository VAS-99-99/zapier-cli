# Set up Zapier for a teammate

The original terminal plugin and the Cowork plugin are separate installations
of this unofficial, read-only project.
Both reuse the same saved Zapier connection under your OS user.

| Where you want to use it | Instructions | Where to use them |
| --- | --- | --- |
| macOS Claude Code or Codex | [macOS quick start](../README.md#macos-quick-start) | Terminal |
| Claude Desktop Cowork | [Upload the Cowork ZIP](#upload-the-cowork-zip) | Claude Desktop chat side |

Run the manual Terminal commands in the main README yourself. Normal
installation needs no Go, Node.js, npm, Python, or source build.

## Steps you complete yourself

1. Run `zapier-pp-cli auth browser` in your own host terminal. Complete the
   visible Zapier login and wait for the `Connected to Zapier` message.
2. Confirm the exact account returned by the session check. Authentication and
   account confirmation are separate steps.
3. For the original plugin, start a fresh Claude Code or Codex chat to load the
   installed skill. For Cowork, upload the ZIP below, approve its local MCP
   server, then start a fresh local task.

## Upload the Cowork ZIP

The install prompt prints the exact verified ZIP path. In Claude Desktop's
**chat side**, click **Customize → Plugins → Add → Upload a plugin**. Select
the ZIP itself—do not unzip it or choose its folder. `Add marketplace` is for
repository sources and has no ZIP uploader.

| Computer | Upload file |
| --- | --- |
| Apple Silicon Mac | `zapier-cowork_darwin_arm64.zip` |
| Intel Mac | `zapier-cowork_darwin_x86_64.zip` |
| Windows x64 | `zapier-cowork_windows_x86_64.zip` |

`darwin` means macOS. Select the ZIP itself, not an extracted folder, the CLI
archive, or GitHub's source-code ZIP. Enable the plugin and approve its local
MCP server when the app asks.

## Test a fresh Cowork task

Copy this after installing the plugin:

```text
Test the installed Zapier Cowork plugin using its Zapier skill and local MCP tools.
Verify the skill and session_check tool are available. If either is missing,
report exactly what is missing and stop.

Call only session_check. Show the connected email and account ID, then stop
for my explicit account confirmation before any further Zapier reads. If
logged out, report that and stop. Reuse my saved login. Do not run shell
commands, open a browser, authenticate, or change anything in Zapier.

Report the actual tool result or error. Tool availability alone does not
prove that login or run-history inspection works.
```

Once you have checked the displayed identity, send this reply:

```text
I confirm the account you just displayed. Inspect my Zapier run history for
the previous 24 hours. Summarize failures with Zap name, time, failed step,
error, and suggested action. Include run links when returned. State the Zaps,
time window, and history pages inspected, plus any access or pagination limits.
If none are found, say "No failures found in the inspected scope." Omit
sensitive step inputs and outputs. Do not modify anything in Zapier.
```

## Schedule the daily check

For unattended checks, first confirm the account in an interactive task. Then
explicitly authorize that same email and account ID in this one scheduled
task's instructions. Each run must check both values before reading history.
A missing value, expired login, or mismatch stops the task. Fresh interactive
tasks still require account confirmation.

`Skip all approvals` controls app tool approvals. It does not itself authorize
an account. Use an installed plugin version that supports the explicit scheduled
account authorization below; an older plugin may still stop for confirmation.

In the Mac desktop app, open `Scheduled`, create a task, and use the manual
setup form:

- Name it `Daily Zapier failed-run check`.
- Choose `Daily` at `09:00`. Select `Europe/Sofia` if offered, and verify the
  saved next-run time. The observed form did not show a timezone. DST handling
  remains unverified.
- Turn `Require this computer` on. Keep that computer awake for the run.
- Select app permissions deliberately. `Skip all approvals` allows automatic
  tool execution; `Manually approve` adds app approval prompts.
- Leave any earlier failed or cloud-only copy paused to avoid duplicate runs.

Replace `YOUR_CONFIRMED_EMAIL` and `YOUR_CONFIRMED_ACCOUNT_ID` below with the
values you personally checked. Keep them in your own task, not in this repository.
Pasting the completed instructions explicitly authorizes read-only checks for
that account in this scheduled task. Leave the task paused until the installed
plugin supports this authorization and a test completes.

Paste these task instructions:

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

Save and complete any device approval the app presents. Use `Run now` to test
local MCP access, the exact account check, and historical reads. If the plugin
still requests confirmation, keep the task paused and update it before calling
the schedule unattended. To test the timer separately, set a near-future time and wait without clicking
`Run now`. A new run proves the timer fired; a completed report proves the
inspection finished.

See [Cowork acceptance and troubleshooting](cowork.md) for the observed Mac
results and remaining checks. Installation, session connectivity, completed
history inspection, and scheduled execution are separate acceptance steps.
