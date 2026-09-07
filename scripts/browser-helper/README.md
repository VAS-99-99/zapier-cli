# Windows browser helper patch

The Windows sign-in helper is agent-browser 0.36.0 at upstream commit
`eb05921bad874cd2a1b4fa5d1149f1ed26576cae`, with one change: the detached Windows
daemon writes stderr to `NUL` instead of a pipe whose reader disappears after
startup. This prevents warning prints from panicking and closing Chrome.

`patch_windows_stderr.py` refuses to run if the expected launch site changes.
The GitHub `browser-helper.yml` workflow builds with the upstream Cargo.lock
and includes source provenance, the patch, license, and SHA-256 checksum in
its artifact. This private sign-in helper omits the optional dashboard assets.

Windows daemon warnings are discarded. Structured command errors still reach
the CLI; early daemon startup errors use its generic startup failure message.
macOS and Linux continue using unmodified upstream release binaries.

The release gate `TestAgentBrowserWindowsWarningKeepsBrowserAlive` opens a
blank disposable browser and triggers a harmless warning. It must pass before
publication. It does not sign in or read cookies.
