# Zapier CLI

`zapier-pp-cli` is a private, remote read-only tool for inspecting a connected
Zapier account: session health, Zaps, run history, run detail, and failed-step
diagnosis. It never changes a Zap or runs a Zapier mutation.

The remote boundary also disables webhook output delivery and remote feedback.
`--deliver` supports stdout or a local file only; feedback stays on the local
machine.

## Five-step setup for a new user

No Zapier developer key or browser extension is required. This CLI connects to
the Zapier account already signed in through a browser. It does not have a
one-click OAuth flow: the user privately copies one authenticated request's
`Cookie` header into the CLI's hidden local prompt.

1. The repository owner invites the recipient's GitHub account to
   [`VAS-99-99/zapier-cli`](https://github.com/VAS-99-99/zapier-cli).
2. The recipient copies the prompt below into a coding agent with terminal
   access.
3. The agent installs the CLI and opens Zapier's login page. It asks before
   installing any missing system dependency or browser.
4. The recipient signs in and follows the agent's browser instructions. The
   session cookie goes directly into a hidden local terminal prompt, never into
   chat.
5. The agent checks the session, shows which Zapier account is connected, and
   waits for confirmation before reading sample account data.

### Copy this to your agent

```text
Install the private, remote read-only Zapier CLI from:
https://github.com/VAS-99-99/zapier-cli

Complete these five stages and explain each one in plain language:

1. ACCESS — Confirm that my GitHub account can read the private repository.
   Prefer `gh auth status` and `gh repo view VAS-99-99/zapier-cli`. If GitHub
   authentication, Git, or GitHub CLI is missing, guide me through the normal
   browser sign-in or official installer. Stop if I have not been invited.

2. INSTALL — Detect my OS and CPU. Prefer a matching asset from a private
   GitHub release if one exists. Otherwise clone or safely update the repository,
   verify Go 1.26.6 or newer, and build both `./cmd/zapier-pp-cli` and
   `./cmd/zapier-pp-mcp`. Ask before installing Go or changing system settings.
   Put the binaries in a user-local bin directory, add it to PATH when needed,
   and verify `zapier-pp-cli --version`.

3. BROWSER — Run `zapier-pp-cli auth setup --launch`. If there is no supported
   browser, ask before installing one or opening its official download page.
   Do not install or recommend a cookie-export extension; built-in browser
   developer tools are enough.

4. CONNECT — Guide me to copy the complete Cookie request-header value from an
   authenticated zapier.com request. Never ask me to paste that cookie into
   chat, never read or echo it, and never place it in command arguments, logs,
   source files, or shell history. Have me paste it directly into the CLI's
   hidden local terminal flow printed by `auth setup`. On Windows, use the
   README's `Read-Host -AsSecureString` recipe instead of a plaintext temporary
   file. If your terminal cannot accept secret interactive input, tell me
   exactly which command to run in my own terminal and wait for me to say it
   completed.

5. VERIFY, THEN STOP — Run `zapier-pp-cli doctor --json`, then
   `zapier-pp-cli session --agent --no-learn`. Tell me the account identity
   returned by that live session check and stop for my confirmation. Only after
   I confirm the account, run this read-only smoke test:
   `zapier-pp-cli zaps list --limit 3 --agent --no-learn`.

Read and obey this repository's AGENTS.md. Zapier access is strictly read only:
never create, edit, enable, disable, replay, cancel, delete, publish, rename, or
otherwise change anything in Zapier. Never add a generic outbound POST,
webhook delivery, or remote feedback path. Local installation and credential
storage are allowed.
```

### Finding the Cookie header

<details>
<summary>Chrome or Edge</summary>

Sign in to Zapier, open Developer Tools, select **Network**, and reload the
page. Select an authenticated request to `zapier.com`, open **Headers**, and
copy the complete value beside **Cookie** under **Request Headers**.

</details>

<details>
<summary>Firefox</summary>

Sign in to Zapier, open Web Developer Tools, select **Network**, and reload the
page. Select an authenticated `zapier.com` request, open **Headers**, and copy
the complete **Cookie** request-header value.

</details>

<details>
<summary>Safari</summary>

Enable Safari's Develop menu if it is hidden, sign in to Zapier, open **Develop
→ Show Web Inspector**, select **Network**, and reload the page. Select an
authenticated `zapier.com` request and copy the complete **Cookie** request
header.

</details>

The cookie grants access to the signed-in Zapier account. Treat it like a
password. Do not paste it into chat, screenshots, logs, source control, or a
cookie-export extension. Logging out of Zapier invalidates the browser session;
repeat setup with a fresh Cookie header when that happens.

## Manual installation and private distribution

This repository is private and currently has no GitHub release, so the prompt
above falls back to a local source build. The public Printing Press catalog and
public latest-release installer are also unavailable for this CLI.

```bash
git clone https://github.com/VAS-99-99/zapier-cli.git
cd zapier-cli
mkdir -p ./bin
go build -o ./bin/zapier-pp-cli ./cmd/zapier-pp-cli
go build -o ./bin/zapier-pp-mcp ./cmd/zapier-pp-mcp
```

For a recipient without Go, the repository owner can privately share the
matching approved archive from `build/share/`. Extract it, make the binaries
executable on macOS/Linux, and add their directory to `PATH`:

```bash
export PATH="/path/to/extracted-zapier-cli:$PATH"
zapier-pp-cli --version
```

Share the matching `.mcpb` bundle through the same private channel for an
MCPB-capable host. Otherwise, point the MCP host at the locally built
`zapier-pp-mcp` binary and reconnect or restart that host.

## Manual credential setup

Run `zapier-pp-cli auth setup --launch` for the current instructions. On
macOS/Linux, the printed flow uses a hidden prompt so the cookie does not enter
shell history:

```bash
printf 'Paste Zapier session cookie: ' >&2; IFS= read -rs ZAPIER_SESSION_COOKIE; printf '\n' >&2
printf '%s' "$ZAPIER_SESSION_COOKIE" | zapier-pp-cli auth set-token
unset ZAPIER_SESSION_COOKIE
zapier-pp-cli doctor --json
```

On Windows PowerShell, use this hidden prompt instead of the temporary-file
fallback printed by `auth setup`:

```powershell
$SecureCookie = Read-Host 'Paste Zapier session cookie' -AsSecureString
$CookiePtr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($SecureCookie)
try {
  [Runtime.InteropServices.Marshal]::PtrToStringBSTR($CookiePtr) | zapier-pp-cli auth set-token
} finally {
  [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($CookiePtr)
  Remove-Variable SecureCookie, CookiePtr -ErrorAction SilentlyContinue
}
```

This writes no plaintext cookie file, zeroes the temporary unmanaged buffer,
and clears its PowerShell variables when it finishes.

`auth set-token` stores the cookie in the local credentials file. Do not share
that file. MCP hosts need the same `ZAPIER_SESSION_COOKIE` value in their
private host configuration; never commit the host configuration.

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
