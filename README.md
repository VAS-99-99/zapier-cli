# Zapier CLI

`zapier-pp-cli` is a private, remote read-only tool for inspecting a connected
Zapier account: session health, Zaps, run history, run detail, and failed-step
diagnosis. It never changes a Zap or runs a Zapier mutation.

The remote boundary also disables webhook output delivery and remote feedback.
`--deliver` supports stdout or a local file only; feedback stays on the local
machine.

## Share it privately

Clone the authorized private source repository, then build locally. This is
the supported source of truth; the public Printing Press catalog and a public
latest-release installer are currently unavailable until this CLI is separately
published.

```bash
git clone https://github.com/VAS-99-99/zapier-cli.git
cd zapier-cli
mkdir -p ./bin
go build -o ./bin/zapier-pp-cli ./cmd/zapier-pp-cli
go build -o ./bin/zapier-pp-mcp ./cmd/zapier-pp-mcp
```

For recipients without a Go toolchain, share an approved prebuilt archive from
`build/share/` through your private distribution channel. Extract it, make the
binaries executable on Unix, and add the extracted directory containing the
binaries to `PATH`:

```bash
export PATH="/path/to/extracted-zapier-cli:$PATH"
zapier-pp-cli --help
```

Share the matching `.mcpb` archive through the same private channel when a
recipient uses an MCPB-capable host. Otherwise, point the MCP host at the
locally built `zapier-pp-mcp` binary. Reconnect or restart the host after
changing its MCP configuration.

## Credentials

Run `zapier-pp-cli auth setup` for the current steps. Sign in at Zapier, copy
the complete `Cookie` request-header value from an authenticated browser
request, and treat it like a password. Never put it in chat, logs, source
control, or a command that displays it.

On macOS/Linux, use a hidden prompt so the cookie does not enter shell history:

```bash
printf 'Paste Zapier session cookie: ' >&2; IFS= read -rs ZAPIER_SESSION_COOKIE; printf '\n' >&2
printf '%s' "$ZAPIER_SESSION_COOKIE" | zapier-pp-cli auth set-token
unset ZAPIER_SESSION_COOKIE
zapier-pp-cli doctor
```

On Windows PowerShell, use a temporary, access-restricted file, then run
`Get-Content -Raw .\private-cookie-file | zapier-pp-cli auth set-token` and
remove the file with `Remove-Item .\private-cookie-file`.

`auth set-token` stores the cookie in the local credentials file. Do not share
that file. MCP hosts need the same `ZAPIER_SESSION_COOKIE` value in their
private host configuration; never commit the host configuration.

## Recipient quick start

1. Receive an authorized source checkout or private archive.
2. Build or extract both CLI and MCP binaries, then add their directory to `PATH`.
3. Configure the browser session cookie as above and run `doctor`.
4. Connect or reconnect the MCP host if needed.
5. Start with `zapier-pp-cli which "failed runs" --json`.

## What it can do

| Capability | Command | Remote effect |
| --- | --- | --- |
| Check session/account health | `session`, `doctor` | Read only |
| List or search Zaps | `zaps list --name <name>` | Read only |
| List run history | `runs list --status error` | Read only |
| Inspect one run | `runs get <run-id>` | Read only |
| Diagnose failed steps | `diagnose <zap-name-or-id>` | Read only |

It cannot create, edit, enable, disable, replay, cancel, delete, publish, or
otherwise mutate remote Zapier state.

`api` inventories only raw endpoints defined by the spec. Use `which` and
`--help` to discover the hand-authored `zaps`, `runs`, and `diagnose` commands.

## Machine output

Use `--agent` for compact JSON and no prompts, `--json` for JSON, `--csv` for
CSV, or `--plain` for tab-separated text. Nested values serialize as compact
JSON in CSV and plain output.

```bash
zapier-pp-cli zaps list --agent
zapier-pp-cli runs list --status error --json
zapier-pp-cli diagnose "Billing sync" --plain
```

For the four inspection commands, `--agent` returns a stable envelope; parse
the payload from `.results` and confirm `.meta.source == "live"`:

```json
{"meta":{"source":"live"},"results":[{"id":101,"state":"on","title":"Example Zap"}]}
```

`--json` without `--agent` returns the bare JSON value instead. The `zaps list`,
`runs list`, `runs get`, and `diagnose` commands are live-only: pass
`--data-source live` when making the choice explicit. A working account
connection is a prerequisite; `doctor` reports whether the saved session
cookie is present.

## Troubleshooting

If a recipient cannot run the binary, check `PATH` and reconnect the MCP host
after updating its command or environment. If authentication fails, repeat the
browser-session setup with a fresh Cookie header; do not print its value while
debugging.

## Unique Features

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
