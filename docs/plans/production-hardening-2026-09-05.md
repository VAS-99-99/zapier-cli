# Production hardening

Scope approved by the user: fix the reviewed production gaps; automated tests
only. The user owns the final Windows install and live login test.

## Fixed decisions

- Keep remote operations read-only. No webhook delivery, Zap actions, live
  account tests, remote Windows installations, or public upstream publishing.
- Keep prebuilt installation and preserve the current results array contract.
- User-owned `auth browser` may validate its candidate credential using only
  GET https://zapier.com/api/v4/session before saving it. It must require an
  authenticated, non-temporary identity and a positive current account ID.
  It must not expose the session response or credentials. The agent's first
  post-login command remains `session --agent --no-learn`, then confirmation.
- Record generated-tree customizations in a new durable patch record.

## Parallel ownership

1. Installer: `install.ps1`, `scripts/tests/install_ps1_test.sh`, a native
   `scripts/tests/windows_production_test.ps1`, `.github/workflows/release.yml`.
   Fix array normalization and add actual Windows PowerShell 5.1 release gates.
   Use synthetic releases in tests; test no-tag, explicit tag, bad checksum,
   upgrade, executable resolution, and private credential permission checks.
2. History: `internal/cli/runs.go`, `runs_test.go`, `diagnose.go`,
   `diagnose_test.go`, plus narrowly named history test/helper files.
   Add `--offset` and `--all` pagination, preserve `results` as an array, expose
   paging details under `meta.pagination` for agent output. Reject missing/null
   or malformed reporting data and non-progressing pages. Do not change the
   allowlisted GraphQL query texts or generic client behavior.
3. Authentication: `internal/cli/auth_browser.go`, `auth_browser_test.go`,
   `auth_browser_process_test.go`, and dedicated auth verification tests.
   Validate candidate cookies before save/close; bound every verification
   request; reject foreign redirects, bad responses, expired sessions and
   partial login. Test timeout/cancellation cleanup and reconnect failures
   without touching real accounts or credentials.
4. Main integration: docs, patch record, native test review, release review,
   whole-project tests, optional fixes assigned back to the owning agent.

Review additions accepted within the installation/security scope:

- Both installers reuse an authenticated GitHub CLI session when anonymous
  downloads fail. No repository-permission changes or token copying.
- Check empty temporary private-file permissions before writing any bytes,
  preserving an existing credential if the check fails.
- Exercise the real MCP stdio entry point for initialization and tool discovery
  without executing tools or loading a live account.

## Flexible choices

Helper names, test seams, page-size cap and concise diagnostic wording may be
chosen by implementers. Prefer existing output helpers and credential APIs.

## Verification and stopping rules

- Add failing regression tests before fixes; report commands and red/green.
- No dependencies, ledger bumps, unsafe cleanup, auth storage format changes,
  or edits outside owned files without notifying the main agent.
- Full Go tests/vet, installer/onboarding tests, race checks, Windows build,
  native Windows CI and release artifact validation before release handoff.
- Document remaining manual checks honestly. A passed fixture cannot establish
  a completed live login, MCP host setup, or complete historical coverage.
