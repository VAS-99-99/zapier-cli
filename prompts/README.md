# Copy-paste prompts

Choose one prompt for your task. Open the `.txt` file, copy its full contents,
and paste it into the destination below. These are instructions to an agent,
not commands to paste into a terminal shell.

## First installation

| Prompt | What it sets up | Paste into |
| --- | --- | --- |
| [Install CLI](install-cli.txt) | CLI, MCP executable, and Claude Code or Codex user-level skill | Claude Code or Codex on your computer |
| [Install Cowork from repository](install-cowork-repository.txt) | CLI plus guidance for adding the matching Cowork repository plugin | Claude Code or Codex on your Mac or Windows computer |
| [Install Cowork from ZIP](install-cowork-zip.txt) | CLI plus a verified Cowork ZIP and upload guidance | Claude Code or Codex on your Mac or Windows computer |

Choose one Cowork installation route. You personally complete any sign-in and
desktop approval. Cowork's Linux shell cannot install the host executables.

## After installation

| Prompt | When to use it | Paste into |
| --- | --- | --- |
| [Update CLI](update-cli.txt) | Update the CLI and terminal agent skill, preserving login | Claude Code or Codex on your computer |
| [Scheduled check](scheduled-check.txt) | After Cowork works and you have confirmed the Zapier account | Cowork's scheduled-task Instructions field |

The scheduled prompt has two account placeholders you must replace. Follow the
[scheduling guide](../docs/teammate-setup.md#schedule-the-daily-check) for the
computer and permission settings. For Cowork plugin updates, use the
[update guide](../docs/cowork.md#update-or-remove).

All five prompts are also copyable in the [main README](../README.md#start-here).
Tests keep those copies aligned. Agent operating instructions live in the
installed skills; see [what a new project receives](../README.md#what-a-new-project-receives).
