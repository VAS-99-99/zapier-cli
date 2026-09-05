@AGENTS.md

# Claude operating instructions

This is an unofficial Zapier inspection CLI owned by the
repository owner. It is not Zapier's official developer-platform CLI. Read
`AGENTS.md` for the permanent remote read-only invariant and `README.md` for
the supported commands.

## Quick installation requests

When asked to install this repository:

1. Clone or update `https://github.com/VAS-99-99/zapier-cli`.
   For private access, reuse the user's authenticated GitHub CLI session or
   guide GitHub sign-in. Never request a pasted token or change repo access.
2. Run `install.ps1 -Agent Claude` on Windows or `install.sh --agent claude`
   on macOS/Linux. Codex uses `-Agent Codex` or `--agent codex` instead.
   This also installs the user-level runtime skill through the native host
   plugin manager. MCP remains optional. Keep this a
   short install. The installer selects the prebuilt release and performs the
   checksum and version checks. Review the installer/auth source when needed
   to establish trust, and report concrete concerns. Normal installation does
   not require project tests, Go, or a source build.
3. Confirm `zapier-pp-cli version` works in the current terminal. Fix command
   resolution before handing control back. Do not ask the user to open another
   terminal or edit PATH.
4. Stop before authentication. Tell the user to run
   `zapier-pp-cli auth browser` personally in their own terminal. The user
   completes the Zapier login outside Claude. Do not run the auth command,
   inspect browser storage, read the credential store, or request a cookie or
   token.
5. Continue only after the user explicitly says `connected`. Run only
   `zapier-pp-cli session --agent --no-learn`, show the exact account identity,
   and stop for confirmation. Do not make another Zapier request before that
   confirmation.

Installation and checksum verification are local operations. Authentication is
a user-owned boundary. Established CLI operations remain limited to the remote
read-only scope in `AGENTS.md`.

When interpreting results, follow the inspection-coverage rules in AGENTS.md.
An empty history or diagnosis is not evidence that a Zap works.

For updates, preserve credentials and local changes. Re-run the current
installer with the same host option, verify the CLI version and enabled plugin,
then start a fresh chat to load it. Do not reconnect unless the session is missing
or expired. Each fresh chat must confirm the exact account with only
`session --agent --no-learn` before other live reads, even with saved credentials.
