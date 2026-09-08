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

## Follow-up: clean-install timeout and shutdown (2026-09-08)

A clean install of rc.7 still reported an initial navigation timeout. Its
helper hash matched the repaired binary, so this was not an old download.
The isolated public login page loaded successfully; that test did not reproduce
the initial failure or establish why Chrome startup was slow.

The Windows blank-page regression exposed a separate shutdown defect: `close`
failed at 10.04 seconds. The test had ignored the close result, sometimes passing
despite that failure and sometimes failing while removing a locked profile.
The helper sends a launch command before ordinary commands when supplied with
headed/profile settings, even before `close`. Cleanup must not pass those
settings, because its launch check can restart a missing or unresponsive browser.

Shutdown now uses an isolated empty config and a 45-second deadline, covering
the helper's 30-second CDP response timeout plus its five-second process wait.
The regression checks the close response. Startup has a separate 150-second
budget to cover the helper's three possible 30-second Chrome launch attempts,
CDP setup, and navigation; ordinary read deadlines remain unchanged. This budget
change prevents the CLI from canceling the helper midway through its startup
attempts; it does not establish the cause of a slow Chrome launch.

For a disposable public-page check on Windows, set
`ZAPIER_TEST_NATIVE_BROWSER=1` and `ZAPIER_TEST_PUBLIC_LOGIN=1`, then run
`go test ./internal/cli -run '^TestAgentBrowserWindowsWarningKeepsBrowserAlive$' -v -count=1 -timeout 4m`.
Do not sign in during this test. It does not read cookies or saved credentials.

On the Asus, the corrected shutdown passed two blank-page runs (19.44 and
19.66 seconds) and the public login-page run (30.88 seconds), including profile
removal. The full Go suite and vet passed. User-owned sign-in after this change
still needs an acceptance check; these tests deliberately do not authenticate.
