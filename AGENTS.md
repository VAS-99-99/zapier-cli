# Zapier Printed CLI Agent Guide

This directory is a generated `zapier-pp-cli` printed CLI. It was produced by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press), so treat systemic fixes as upstream Printing Press fixes first. Keep local edits narrow and document why a generated-tree patch belongs here.

## Remote Read-Only Product Invariant

This product must never implement or invoke a Zapier mutation, including when a
future user asks for one. Zapier traffic is limited to REST `GET` and
query-only GraphQL `POST`; do not add write methods, mutation documents, or
workflows that change Zapier state.

Allowed remote scope is intentionally narrow:

- session and account-health checks;
- Zap listing and search;
- run-history listing;
- run detail; and
- failed-step diagnosis.

Local writes for credentials/configuration, caches, receipts/audit, explicitly
requested file output, local feedback, and the on-device learning store remain
permitted. They never authorize a remote Zapier write.
Remote webhook delivery and remote feedback are prohibited as well: the CLI
must not expose any generic outbound POST escape hatch. Output delivery may use
stdout or a local file only, and feedback must remain on this machine.

## Local Operating Contract

For an established connection, start by asking the generated CLI for current
runtime truth:

```bash
zapier-pp-cli doctor --json
zapier-pp-cli agent-context --pretty
```

Normal-user installation must use the checksummed private GitHub Release via
`install.sh` or `install.ps1`. Keep Go builds in contributor workflows. The CLI
owns its pinned browser helper and browser; users do not install or operate Go,
Node.js, npm, Playwright, agent-browser, a browser extension, or a cookie export
tool.

Immediately after `auth browser`, the account-safety checkpoint overrides that
order: run only `zapier-pp-cli session --agent --no-learn`, show the exact
account identity, and stop for user confirmation. Do not run `doctor` or any
other live read until the user confirms that account.

Use runtime discovery instead of relying on a copied command list:

```bash
zapier-pp-cli which "<capability>" --json
zapier-pp-cli <command> --help
```

Add `--agent` to command invocations for JSON, compact output, non-interactive defaults, and no color:

```bash
zapier-pp-cli <command> --agent
```

This product has no authorized remote mutation. If command help, runtime
discovery, or behavior appears capable of changing Zapier state, do not run the
command. Treat it as a product defect and report it. A dry run or confirmation
flag cannot authorize that command.

## Novel Command Data Sources

Every hand-written novel command must declare its strategy in a Go line comment:

```go
// pp:data-source auto
```

Use exactly one of `auto`, `local`, `live`, or `computed`. Keep `auto` when the command honors `--data-source auto|local|live` by preferring live data with a local fallback; use `local` for local-only reads, `live` for remote-only reads, and `computed` for pure computation from embedded rules. Change a generated scaffold's `auto` default deliberately when its implementation has a narrower source, and reject incompatible `--data-source` requests with a clear error. TODO stubs still fail dogfood even when annotated.

## Self-Learning Loop

This CLI ships a self-capturing teach/recall loop backed by the local SQLite store. The CLI journals every invocation, derives `flag_alias` candidates from failed-flag + corrected-retry pairs, and synthesizes a playbook candidate when a family is taught without one - no manual failure bookkeeping. The agent's role is judgment:

1. On a new user question, call `zapier-pp-cli recall "<question>" --agent` FIRST. If `found=true` and the top result has `entity_match == "exact"` and `confidence >= 2`, skip discovery and go straight to the live fetch for the returned resource IDs. If the store is cold (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of the session.
2. When the envelope carries a `candidates` section (warning `candidates_present`), candidates are try-then-confirm, never facts: follow each candidate's two-step `next_action` verbatim (trial command first, then `learnings confirm <id>` only after the trial verified the behavior), and reject wrong ones with `learnings reject <id>`. Never re-teach something recall surfaced as a candidate; confirm or reject it instead.
3. After answering, always fire `zapier-pp-cli teach --query "<question>" --resource <id> --resource-type <type> &` in the background - teaching is unconditional and is the anchor that triggers playbook synthesis. Teach the structural question with identifiers stripped (no names, emails, phone numbers, account ids); the CLI warns on obvious PII shapes but does not block.
4. Use `learnings list` to inspect taught rows, `learnings forget "<question>"` to undo a bad teach, `learnings candidates` for the full open candidate set, and `learnings stats` for the loop's local metrics. `teach-pattern` and `teach-lookup` install manual generalization rules when one teach should cover a whole family (e.g. one country alias unlocks every per-country query).
5. If `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and keep the rest of the flow.

Annotations: `recall`, `learnings list`, `learnings candidates`, and `learnings stats` carry `mcp:read-only=true`; `teach`, `teach-playbook`, `playbook amend`, `learnings confirm`, `teach-pattern`, and `teach-lookup` carry `mcp:local-write=true` (writes land only in the CLI's own local store); `learnings forget` and `learnings reject` keep honest may-write/destructive defaults.

### Success definition

Measurement is local-only: the `learn_events` table and `learnings stats`; nothing leaves this machine. Judge the loop on recall hit rate and teach-to-reuse at a minimum denominator of 50+ recall events. Near-zero rates at that denominator mean the loop is not earning its keep for this CLI - surface that in retros. An empty or thin events table means insufficient adoption, not failure.

The store's schema stamp is one-way: once this binary opens the database, an older binary refuses it (README.md carries the upgrade note).

Disable the loop with `--no-learn` per-invocation or `ZAPIER_NO_LEARN=true` for the whole session - useful for deterministic agent flows that don't want a learning row to silently change subsequent query results.

## Platform Credential References

Normal API authentication is separate from optional platform-source credential
resolution. If this CLI uses indirect references for a tenant-gated platform
source, add the downstream registration in a preserved hand-authored file
under `internal/cli/` and provide both `CredentialResolverFactory` and
`ValidateSourceProfile` on `platformSourceRegistration` for any selected source
that has references. A source with no references may omit both hooks and receives
an empty credential map. Keep reference values opaque to shared profile code,
validate only the selected source in the downstream hook, and never persist
resolved credential bytes. Do not edit generator-owned `internal/platform`
packages; a reprint refreshes those files while retaining the downstream
registration file.

For private-release install, auth, MCP setup, reconnect, uninstall, examples,
and longer product guidance, read `README.md` and `SKILL.md`. This file stays
small so repo-local agents get invariant local guidance without duplicating the
generated docs.

## Release Ledger

`CHANGELOG.md` and `.printing-press-release.json` are the public library's per-CLI release ledger. Fresh prints carry an unstamped runtime version such as `0.0.0-dev`; the final `YYYY.M.N` CLI release version is assigned only after a publish PR merges in `mvanhorn/printing-press-library`. Do not hand-bump those files or edit `var version = ...` for release bookkeeping; preserve existing ledger files on reprint and let the library workflow stamp the next release.

## Local Customizations

This directory is **generated output** -- a fresh print can overwrite the whole tree, so ad-hoc hand-edits don't survive on their own. If you modify the generated code, record each change under `.printing-press-patches/` (parallel to `.printing-press.json`). Regen and publish-validate read those records and fail closed when a recorded file or call site is gone, so a dropped customization cannot ship as if it were still applied.

The entry shape, and the altitude to write it at -- a durable reprint-guard, not a changelog -- live in the public library's `AGENTS.md`, which is the single source of truth; this guide intentionally doesn't duplicate them.
