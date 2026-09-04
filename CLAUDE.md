@AGENTS.md

# Claude operating instructions

This is a public-source, unofficial Zapier inspection CLI owned by the
repository owner. It is not Zapier's official developer-platform CLI. Read
`AGENTS.md` for the permanent remote read-only invariant and `README.md` for
the supported commands.

## Installation requests

When asked to install this repository:

1. Clone or update `https://github.com/VAS-99-99/zapier-cli`.
2. Inspect the source, `AGENTS.md`, `.github/workflows/release.yml`,
   `scripts/package-release.sh`, and the release `SHA256SUMS`. Confirm that the
   selected asset matches the current OS and CPU and that its checksum passes.
3. Install the matching prebuilt release with the repository installer. The
   installer writes local CLI and MCP files and does not authenticate or
   contact Zapier. Do not build from source for a normal installation.
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
