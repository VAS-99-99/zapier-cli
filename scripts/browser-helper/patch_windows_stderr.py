"""Apply the narrow Windows daemon-lifetime fix to pinned agent-browser source."""
from pathlib import Path
import sys

path = Path(sys.argv[1]) / "cli/src/connection.rs"
source = path.read_text()
old = '''cmd.creation_flags(CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS)
                .stdin(Stdio::null())
                .stdout(Stdio::null())
                .stderr(Stdio::piped())'''
new = '''cmd.creation_flags(CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS)
                .stdin(Stdio::null())
                .stdout(Stdio::null())
                // The startup caller drops its child handle after readiness.
                // A pipe here then has no reader: later eprintln! panics on
                // Windows, killing the daemon and its sign-in browser.
                .stderr(Stdio::null())'''
if source.count(old) != 1:
    raise SystemExit("Pinned Windows launch site changed; refusing to patch")
path.write_text(source.replace(old, new))
