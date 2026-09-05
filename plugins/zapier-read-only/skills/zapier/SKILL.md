---
name: zapier
description: Inspect Zapier Zaps, run history, failed runs, and step inputs, outputs, or errors using the installed zapier-pp-cli. Use when the user asks to check their Zaps, inspect Zapier runs, diagnose a failed Zap, or connect this unofficial read-only CLI. Not for building Zapier integrations or changing Zaps.
---

# Zapier read-only inspection

Use `zapier-pp-cli`, the unofficial CLI from
`https://github.com/VAS-99-99/zapier-cli`. This skill is usable outside its
source repository. MCP is optional; the CLI works through the agent's terminal.

## Locate the installed CLI

Resolve `zapier-pp-cli` on PATH. If missing, check only the default install path:

- Windows: `%LOCALAPPDATA%\Microsoft\WindowsApps\zapier-pp-cli.exe`.
- macOS/Linux: `${XDG_BIN_HOME:-$HOME/.local/bin}/zapier-pp-cli`.

Use the absolute path if the host has an old PATH. If neither exists, ask for
the custom install location or permission to install the checksummed prebuilt
release from the repository. Normal users never need Go or a source build.
Do not search the computer for a source clone. Installing this plugin alone
does not install the executable.

## Confirm the account in each new chat

Saved credentials belong to the same OS user's CLI and survive a new chat.
Do not reconnect merely because this conversation is new.

Before the account has been confirmed in this conversation, run only:

```bash
zapier-pp-cli session --agent --no-learn
```

Show the returned account email/name and current account ID, then stop and ask
the user to confirm. Do not list Zaps or runs, run doctor, or make another live
read before confirmation. Never infer the account from this skill or memory.

If the CLI reports a missing or expired session, tell the user to run
`zapier-pp-cli auth browser` personally. The CLI manages its browser components;
the user signs in. Do not run authentication, inspect browser storage, read the
credential file, or ask for cookies/tokens. After the user reports `connected`,
perform the session-only checkpoint above. A validated saved session awaiting
account confirmation is not a failed login. Leave login retries to the user.

## Inspect the requested records

After account confirmation, use runtime discovery when the syntax is unclear:
`zapier-pp-cli which "<capability>" --json` or the command's `--help`.
Use `--agent --no-learn` for deterministic JSON without local learning writes.

Common reads:

```bash
zapier-pp-cli zaps list --agent --no-learn
zapier-pp-cli runs list --zap <zap-id> --agent --no-learn
zapier-pp-cli runs get <run-id> --agent --no-learn
zapier-pp-cli diagnose <zap-id> --agent --no-learn
```

Report the inspected time window and pagination coverage. An empty failure
list means no failures were found in that scope, not that the Zap is healthy.
Step data is what Zapier returned, not a complete outbound HTTP trace. Redact
sensitive values before sharing run inputs, outputs, errors, or transcripts.

## Remote read-only boundary

Allowed work is session/account checks, Zap listing/search, run history,
run detail, and failed-step diagnosis. Never create, edit, rename, enable,
disable, publish, delete, replay, cancel, or trigger anything in Zapier. Never
send a webhook or use another tool to bypass this boundary. Report a command
that appears able to mutate remote state as a defect instead of running it.

The CLI owns local credentials. Do not put credentials or account identifiers
in this skill, plugin configuration, agent memory, or MCP settings.
