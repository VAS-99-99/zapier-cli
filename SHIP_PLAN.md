# Zapier Read-Only CLI Release Plan

## Outcome

Ship the private Zapier CLI as checksummed GitHub Release archives so ordinary
users do not install Go, Node.js, npm, Playwright, or a browser extension. Their
agent downloads the correct archive, installs the two binaries, runs one
connection command, and registers the MCP server.

The CLI remains remotely read-only. It may inspect the connected account, Zaps,
historical runs, run detail, and failed steps. It must never create, edit,
enable, disable, rename, replay, or delete anything in Zapier. Local installation
and protected credential storage are permitted.

## Supported release targets

- Windows x64: `zapier-cli_windows_x86_64.zip`
- macOS Apple Silicon: `zapier-cli_darwin_arm64.tar.gz`
- macOS Intel: `zapier-cli_darwin_x86_64.tar.gz`
- Linux x64: `zapier-cli_linux_x86_64.tar.gz`

Each archive contains `zapier-pp-cli`, `zapier-pp-mcp`, `README.md`, `SKILL.md`,
and `LICENSE`. Windows executables use `.exe`. The release also contains one
`SHA256SUMS` file.

## User journey (maximum five stages)

1. Give the public repository URL to Claude or Codex and let the agent download
   the latest release asset for the current OS and architecture. Private-repo
   access is handled through GitHub CLI authentication.
2. Run the platform installer. It verifies the checksum, puts both binaries in
   a user-local directory, updates PATH when needed, and registers
   `zapier-pp-mcp` with the current agent host.
3. Run `zapier-pp-cli auth browser`. The CLI downloads its pinned browser helper
   and Chrome for Testing if missing, opens a visible dedicated Zapier window,
   waits for the user to sign in, saves only Zapier-scoped cookies to protected
   local storage, and closes the window. No credential is shown or copied.
4. Run only `zapier-pp-cli session --agent --no-learn`, show the exact connected
   account identity, then stop for the user's confirmation.
5. After confirmation, run the read-only smoke test and use the CLI/MCP normally.

## Parallel work packages

### WP1 — Release packaging and automation

Owner files: `.github/workflows/release.yml`, `scripts/package-release.sh`, and
release-specific tests.

- Build both binaries reproducibly with `CGO_ENABLED=0` for the four targets.
- Inject the tag version through existing linker variables without modifying
  the Printing Press release ledger.
- Create deterministic archives, include docs/license, and generate
  `SHA256SUMS`.
- Trigger only on an explicit version tag or manual workflow dispatch.
- Upload artifacts to a public GitHub Release using the repository token.
- Add a verification script that inspects archive contents and validates every
  checksum.

Handoff: a local dry run produces all four archives; cross-built CLI and MCP
binaries report the injected version; archive and checksum verification passes.

### WP2 — No-Go installers

Owner files: `install.ps1`, `install.sh`, and installer tests.

- Detect OS and architecture, select the matching release asset, and fail with
  a precise unsupported-platform message.
- Use an authenticated `gh` session to resolve and
  download the latest release. Never embed a GitHub token.
- Verify the downloaded archive against `SHA256SUMS` before extraction.
- Install both executables in a user-local application bin directory without
  administrator rights; preserve existing installs by replacing only these two
  files after verification.
- Print exact PATH and Claude/Codex MCP registration commands. Support a
  non-interactive verification mode for clean-machine tests.

Handoff: tests cover platform selection, checksum failure, missing GitHub
authentication, install replacement, and MCP guidance.

### WP3 — Beginner and agent documentation

Owner files: `README.md`, `SKILL.md`, `AGENTS.md`, `manifest.json`, and
`tools-manifest.json`.

- Put one copyable “Give this to your agent” block near the top of the README.
- Keep installation and connection to five stages or fewer.
- State plainly that normal users do not need Go, Node.js, npm, Playwright, or a
  cookie extension; `auth browser` owns its browser prerequisite.
- Document private-repository access, Windows/macOS/Linux commands, Claude and
  Codex MCP registration, reconnecting, uninstalling, and capability limits.
- Preserve the first-live-call account checkpoint and strict remote read-only
  invariant everywhere.

Handoff: a terminology scan finds no normal-user Go build or manual-cookie
instruction, and all docs agree on commands, supported targets, and safety.

### Root integration — security, verification, test, and release

- Review all agent diffs and resolve overlaps without discarding existing work.
- Run unit tests, race-sensitive auth tests, vet, Windows cross-build, read-only
  transport/source scans, secret-output scans, workflow validation, installer
  tests, and release dry-run verification.
- Create a prerelease in the public repository and perform a clean Windows x64
  install from that release with no Go toolchain involved.
- Open the visible login window. After sign-in, make `session --agent --no-learn`
  the only live Zapier call, report the exact account, and stop for confirmation.
- After confirmation, complete bounded read-only acceptance tests, publish the
  final public release, and verify its downloadable assets and checksums.

## Plan re-verification

- **No-Go install:** yes. End users consume release archives; Go is only a CI
  build dependency.
- **Cross-platform auth:** yes. The CLI-managed pinned browser helper supports
  the declared targets. Printing Press's `press-auth` is not used because its v1
  documentation explicitly limits it to macOS.
- **Credential safety:** yes. Cookies stay between the dedicated browser helper
  and the CLI process, are filtered to Zapier, and enter protected local storage;
  installers and MCP configs contain no Zapier or GitHub token.
- **Remote read-only:** yes. Packaging adds no network capability, and the
  existing REST/GraphQL guards remain required release gates.
- **Account safety:** yes. `session --agent --no-learn` is the only first live
  call and there is an explicit stop before any other account read.
- **Agent-ready delegation:** yes. Packaging, installers, and docs have separate
  file ownership and objective handoffs; root owns integration and publication.
- **Beginner usability:** yes. The happy path is at most five stages and requires
  no credential copying or development toolchain.
- **Release honesty:** yes. The final release is created only after clean-machine
  install, checksum, account checkpoint, and read-only acceptance pass.

## Final evidence required

1. Passing test/vet/security reports from the exact release commit.
2. Four verified archives plus `SHA256SUMS` in the public GitHub Release.
3. A clean Windows x64 installation performed from the release, without Go.
4. The exact account identity from the session-only checkpoint and the user's
   explicit confirmation before broader live tests.
5. A final private-repository visibility check and working release link.
