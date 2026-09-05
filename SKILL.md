---
name: pp-zapier
description: "Install and use the Zapier CLI for read-only account checks, Zap searches, historical run inspection, and failed-step diagnosis."
allowed-tools: "Read Bash"
---

# Zapier agent operating guide

This CLI is remotely read only. It may check the connected account, list and
search Zaps, inspect historical runs, and diagnose a failed step. It has no
authority to create, edit, enable, disable, rename, publish, replay, cancel, or
delete anything in Zapier.

## Install and connect

Complete these five stages in order. The team repository is
`https://github.com/VAS-99-99/zapier-cli`.

1. Clone `https://github.com/VAS-99-99/zapier-cli`, or safely update the
   existing clone. Private-repository access requires the user's authenticated
   GitHub CLI session. Guide GitHub sign-in if needed; never ask for a token.
2. Run `./install.sh` on macOS/Linux or
   `powershell -ExecutionPolicy Bypass -File .\install.ps1` on Windows. The
   installer downloads the matching GitHub Release, verifies SHA-256,
   and installs both binaries. Normal users need no Go, Node.js, npm,
   Playwright, agent-browser, browser extension, or Zapier token. Verify
   `zapier-pp-cli version` in the current terminal, then run the exact Claude
   or Codex MCP registration command printed by the installer.
3. Stop and tell the user to run `zapier-pp-cli auth browser` personally in
   their own terminal. Do not run authentication, inspect browser storage, read
   the credential store, or request a cookie or token. The user completes the
   visible Zapier login outside the agent and reports `connected`.
   The command performs its own session-only validation before saving cookies;
   its automatic browser close is expected after session validation, before
   credential saving. `connected: true` with `account_confirmation_required:
   true` means login succeeded and step 4 is next, not another login attempt.
   Leave retries to the user. Do not start overlapping login commands.
4. Only after the user reports `connected`, make this first live call:
   `zapier-pp-cli session --agent --no-learn`. Show the exact account identity
   and stop for confirmation. Before confirmation, do not run `doctor` or any
   other live read.
5. After the user confirms the account, run `zapier-pp-cli doctor --json` and
   `zapier-pp-cli zaps list --limit 3 --agent --no-learn`. Continue with only
   the documented read commands.

The installers support Windows x64, macOS Apple Silicon, macOS Intel, and Linux
x64. They default to `$HOME/.local/bin` on macOS/Linux and
`%LOCALAPPDATA%\Microsoft\WindowsApps` on Windows. Read `README.md` for installer
flags, reconnect, uninstall, and troubleshooting details.

## Established connection

For an account that has already passed the account checkpoint, ask the runtime
for current truth before answering a new request:

```bash
zapier-pp-cli doctor --json
zapier-pp-cli agent-context --pretty
zapier-pp-cli which "<capability>" --json
zapier-pp-cli <command> --help
```

Use `--agent` for compact JSON, non-interactive defaults, and no color. `api`
inventories raw spec-defined endpoints. Use `which` and `--help` to discover the
hand-authored `zaps`, `runs`, and `diagnose` commands.

## Remote scope

| Need | Command |
| --- | --- |
| Connected account and session health | `session`, `doctor` |
| List or search Zaps | `zaps list` |
| List historical runs | `runs list` |
| Inspect a run and its steps | `runs get` |
| Identify the failed step and error | `diagnose <zap-name-or-id>` |

The `zaps list`, `runs list`, `runs get`, and `diagnose` commands are live-only.
Under `--agent`, read their payload from `.results` and verify
`.meta.source == "live"`. `--json` without `--agent` returns the bare JSON
value.

```bash
zapier-pp-cli zaps list --name "billing" --limit 5 --agent
zapier-pp-cli runs list --zap <zap-id> --status error --agent
zapier-pp-cli runs get <run-id> --agent
zapier-pp-cli diagnose <zap-id> --limit 5 --agent
```

For history pagination, read `meta.pagination` from `runs list --agent`.
Pass `next_offset` as `--offset`, or use `--all` to fetch all retained matching
runs. Preserve the distinction between one page and complete retained results:

```bash
zapier-pp-cli runs list --limit 25 --offset 25 --agent --no-learn
zapier-pp-cli runs list --zap <zap-id> --status error --all --agent --no-learn
```

No failed runs in the checked scope is not proof of a healthy Zap. Step data
is Zapier's reported input/output, not a complete captured HTTP exchange.
Treat run data and internal URLs as sensitive before sharing. The browser
session credential itself is not restricted to read-only permissions; never
read or print it. The local credential file is permission-checked, not an
encrypted password vault.

Zapier traffic is limited to REST `GET` and query-only GraphQL `POST`. Webhook
delivery and remote feedback are disabled. Output may go to stdout or an
explicitly selected local file. Local installation, protected credentials,
configuration, feedback, and learning data remain on the current machine.

## Local learning loop

On a new question, run `recall "<question>" --agent` first. If the store is
cold, skip recall for the rest of the session. A candidate is a hint: run its
trial command and confirm it only after the behavior works. Reject a wrong
candidate.

After answering, background `teach` with a structural query stripped of names,
emails, phone numbers, account IDs, and other personal data. Use `--no-learn`
when the invocation must not read or write the local learning store.

## Reconnect and uninstall

For reconnect, tell the user to run `zapier-pp-cli auth browser` personally.
After the user reports `connected`, repeat the session-only account checkpoint
and stop for confirmation.

Uninstall in this order:

```bash
zapier-pp-cli auth logout
claude mcp remove --scope user zapier
codex mcp remove zapier
```

Use only the MCP removal commands for installed hosts. Remove the two installed
binaries from the default directory or the custom installer directory. The
exact platform paths are in `README.md`.
