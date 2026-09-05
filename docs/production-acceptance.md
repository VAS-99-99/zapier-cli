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

## New-chat skill acceptance

Install with `--agent claude` or `--agent codex`, or Windows `-Agent Claude`
or `-Agent Codex`. Verify `zapier-read-only@vas-zapier-cli` is installed and
enabled with the host's plugin list. Start a fresh chat outside the repository
and ask "Check my Zapier runs." It should reuse the saved connection, run only
the session check, and show the exact account before asking for confirmation.
No new login window should open. Repeat the update installer and confirm the
same behavior. MCP registration is optional for this skill-based workflow.

Developer checks: `python3 scripts/tests/plugin_contract_test.py` validates
both catalogs and the shared skill. With Claude Code installed,
`python3 scripts/tests/claude_plugin_discovery_test.py` installs into an isolated
profile and confirms the skill is advertised to a fresh chat outside the repo.
That test uses a local synthetic model response, not a real model or Zapier
account. Real model selection and live account reads remain manual acceptance.
