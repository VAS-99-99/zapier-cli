---
name: pp-zapier
description: "Read-only Zapier account inspection through zapier-pp-cli."
allowed-tools: "Read Bash"
---

# Zapier agent operating guide

This CLI is a remote read-only surface. It may inspect account/session health,
list and search Zaps, list run history, fetch a run, and diagnose failed steps.
It intentionally cannot create, edit, enable, disable, replay, cancel, delete,
publish, or otherwise change Zapier state.

## Discover before acting

Ask the runtime rather than assuming a copied command list:

```bash
zapier-pp-cli doctor --json
zapier-pp-cli agent-context --pretty
zapier-pp-cli which "<capability>" --json
zapier-pp-cli <command> --help
```

Add `--agent` to use compact JSON, no prompts, and no color. Use `--json`,
`--csv`, or `--plain` when a specific machine format is needed; nested cells in
CSV/plain are compact JSON, and live results identify their provenance.

`api` inventories only raw spec-defined endpoints. Use `which` and `--help` to
discover the hand-authored `zaps`, `runs`, and `diagnose` commands.

## Connection prerequisite

All remote reads require a connected Zapier browser session. Run
`zapier-pp-cli auth setup`, sign in at `https://zapier.com/app/login`, and copy
the complete `Cookie` request-header value from an authenticated request.
Treat it as a password: never send it to chat, logs, or a displaying command.

```bash
printf 'Paste Zapier session cookie: ' >&2; IFS= read -rs ZAPIER_SESSION_COOKIE; printf '\n' >&2
printf '%s' "$ZAPIER_SESSION_COOKIE" | zapier-pp-cli auth set-token
unset ZAPIER_SESSION_COOKIE
zapier-pp-cli doctor
```

On Windows PowerShell, put the cookie in a temporary, access-restricted file,
run `Get-Content -Raw .\private-cookie-file | zapier-pp-cli auth set-token`,
then remove it with `Remove-Item .\private-cookie-file`.

The cookie is an account connection, not a developer key. `auth status` only
reports local presence; it does not prove the browser session remains valid.

These public-catalog commands are unavailable until this CLI is separately
published. Do not use them today; build from the authorized private source or
use an approved private archive as described in `README.md`.

<details><summary>Future public install, currently unavailable</summary>

## Prerequisites: Install the CLI

This skill drives the `zapier-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install zapier --cli-only
   ```
2. Verify: `zapier-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/zapier/cmd/zapier-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Read-only CLI for your Zapier account: list zaps, check run history, diagnose failures. Never modifies, deletes, or toggles anything.

</details>

## Read-only recipes

The `zaps list`, `runs list`, `runs get`, and `diagnose` commands are live-only.
They accept `--data-source live`; do not ask for a local fallback or use them if
`doctor` reports no connection.

```bash
# Discover a Zap, then inspect recent failures.
zapier-pp-cli zaps list --name "billing" --agent
zapier-pp-cli runs list --zap <zap-id> --status error --agent

# Inspect a known run or diagnose one Zap end to end.
zapier-pp-cli runs get <run-id> --agent
zapier-pp-cli diagnose "Billing sync" --limit 10 --agent
```

`--agent` wraps these commands as
`{"meta":{"source":"live"},"results":...}`. Read data from `.results` and
verify `.meta.source == "live"`. `--json` without `--agent` returns the bare
JSON value.

Use `--dry-run --agent` before an unfamiliar invocation. It does not contact
Zapier. Do not infer permission to mutate anything from a successful read.

## Available remote scope

| Need | Command |
| --- | --- |
| Session/account health | `session`, `doctor` |
| List/search Zaps | `zaps list` |
| List run history | `runs list` |
| Run detail | `runs get` |
| Failed-step diagnosis | `diagnose <zap-name-or-id>` |

Local configuration, credential storage, feedback, and learning commands may
write only on this machine. They do not authorize or trigger a remote Zapier
write. Webhook output delivery and remote feedback are disabled; `--deliver`
supports stdout or a local file only.

## Local learning loop

Before new discovery, run `recall "<question>" --agent`. Candidates are hints,
not facts: perform the supplied trial first, then `learnings confirm <id>` only
after it works; use `learnings reject <id>` when it does not. After answering,
always background `teach` with a structural query stripped of names, emails,
phone numbers, account IDs, and other PII. Use `--no-learn` when a deterministic
invocation must not read or write the local learning store.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Read-only Zap inspection
- **`zaps list`** — List Zaps and filter by a case-insensitive title substring without changing them.

  _Agents can safely resolve a Zap before inspecting its historical runs._

  ```bash
  zapier-pp-cli zaps list --name webhook --limit 5 --agent
  ```

### Read-only run inspection
- **`runs list`** — List historical Zap runs with optional Zap and status filters.

  _Agents can find the relevant failed execution using a bounded live read._

  ```bash
  zapier-pp-cli runs list --status error --limit 10 --agent
  ```
- **`runs get`** — Open one historical run and return every step's status, input, output, and error.

  _Agents can inspect exactly what happened inside a known run without replaying it._

  ```bash
  zapier-pp-cli runs get <run-id> --agent
  ```

### Read-only diagnosis
- **`diagnose`** — Resolve a Zap and report the exact failed step and error from recent failed runs.

  _Agents can answer where and why a Zap failed while preserving the product's strict read-only boundary._

  ```bash
  zapier-pp-cli diagnose <zap-id> --limit 5 --agent
  ```
