# Manual release acceptance

The user runs this checklist after automated release checks pass. Until then,
the release is a candidate, not a production approval. Use only a confirmed
account's existing runs. Do not trigger, replay, edit, or delete a Zap to create
test data.

1. Install the candidate's prebuilt binaries on the target OS without a source
   build. Verify `zapier-pp-cli version` works in the current terminal. Check
   installation without a tag as well as an explicitly selected version.
   GitHub repository access and release download access are prerequisites for
   this private repository.
2. Personally run `zapier-pp-cli auth browser`. Complete login, including MFA
   if enabled. The browser should wait through login and close after saving a
   valid session. Run `zapier-pp-cli session --agent --no-learn`, confirm the
   email and account ID, and only then permit other live reads.
3. List/search Zaps, inspect a successful and a failed historical run, and
   compare the failed step/input/output/error with the Zapier UI. Check an
   empty-history Zap without declaring it healthy. Exercise a second page and
   `--all`; confirm the paging metadata and returned IDs agree.
4. Personally retry login, test abandoning the window, and reconnect after an
   expired session when available. A failed reconnect must not replace a prior
   saved credential. Failure messages must not contain credential values.
   Register the MCP server using the installer's absolute path and confirm the
   intended Claude/Codex host can discover it and perform a read-only query.
   CLI success alone does not certify MCP host setup.
5. Upgrade to the candidate and verify version resolution again. When finished
   testing, personally run `zapier-pp-cli auth logout`, remove only the MCP
   registrations you added, and remove the two installed binaries if desired.
   Confirm the credential and temporary login profile are gone without opening
   their contents. Keep unrelated browsers, profiles, and installations intact.

Record the release tag, OS/shell, checks passed/failed, and redacted errors. Do
not attach credential files, browser profiles, or unredacted run payloads.
