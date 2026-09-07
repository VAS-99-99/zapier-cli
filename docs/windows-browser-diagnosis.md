# Windows browser closes during sign-in

Confirmed on Dad Asus (Windows 10 19045), 2026-09-07, with the pinned
agent-browser 0.36.0 helper. The fix is included in CLI v0.1.0-rc.7.

## Cause

The helper's Windows launcher starts its daemon with piped stderr, then drops
the pipe reader once startup succeeds. The daemon redirects stderr on Unix,
but the equivalent protection is missing on Windows.

On the public Zapier login page, preparing an iframe can time out at
`Page.enable`. The helper reports that recoverable error using `eprintln!`.
Writing to the closed stderr pipe panics; the daemon exits with code 101.
Subsequent helper reads may start another daemon/browser, explaining the
close/reopen loop. Our session checks then fail or observe an inactive session.

Relevant upstream code at tag `v0.36.0`:

- `cli/src/connection.rs`: Windows `ensure_daemon` launch uses
  `.stderr(Stdio::piped())`; the child handle is dropped after readiness.
- `cli/src/native/daemon.rs`: stderr redirection is guarded by `cfg(unix)`.
- `cli/src/native/actions.rs`: iframe setup failure prints
  `Warning: failed to prepare iframe session controls: ...` with `eprintln!`.

## Verification

The tests used new disposable profiles and loaded only the public login page.
No login was performed and no cookies or account data were read.

1. Normal helper launch reproduced the daemon exit on this computer. Windows
   process monitoring recorded PID 4888 exiting with code 101 at 14:13:53;
   a subsequent read used replacement daemon PID 2956.
2. A directly launched daemon with stderr connected to an open file survived
   the same page. Its log captured `CDP command timed out: Page.enable` during
   iframe preparation. Status checks recovered and explicit close succeeded.
3. The same direct-daemon harness with stderr connected to a pipe whose reader
   was closed exited with code 101. Status connections then failed.

The comparison isolates stderr handling from the Zapier login and CLI session
validation. A browser-control timeout triggers the warning; the broken pipe
turns that warning into the process crash.

## Repair and verification

The Windows launcher now uses `Stdio::null()` for daemon stderr, so later
warning writes have a durable sink. Structured command errors remain available;
early Windows daemon startup failures use the generic launch error. The patch
and pinned-source build workflow are under `scripts/browser-helper/` and
`.github/workflows/browser-helper.yml`. The CLI verifies the repaired helper's
SHA-256 before installation; other platforms retain their upstream binaries.

On Dad Asus, `TestAgentBrowserWindowsWarningKeepsBrowserAlive` failed with the
original helper and passed with the repaired binary. This regression triggers
a harmless warning on a blank page. The original public-login-page probe then
completed 36 status checks with one daemon PID, no failed status calls, and a
successful explicit close.

User-owned sign-in remains a separate acceptance check: run
`zapier-pp-cli auth browser` personally, then verify the saved account with
only `session --agent --no-learn`. No agent performed a login during testing.
