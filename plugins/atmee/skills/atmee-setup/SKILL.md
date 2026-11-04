---
name: atmee-setup
description: Connect and check what atmee avatars need - the atmee server, sign-in, one account, credits, updates and tool prompts. Use when setting up or if something is off. NOT for building an avatar.
license: Proprietary
metadata:
  owner: Atmanity
allowed-tools: Bash(*/scripts/update-check*) Bash(*/scripts/detect-host*)
---

# Set up atmee

## Contents

- 0. Check for updates
- 1. Find the server
- 2. Add the server (only when it is missing)
- 3. Run the checks
- 4. Ask first
- 5. Say the result

You connect the atmee server if it is missing, check everything the
create-avatar skill needs, fix what you can, and give the creator the one step
for each thing only they can do. The create-avatar skill runs this first; the
creator can also run it on its own.

Keep it light: a creator who is ready should see one line, not a report. Run
every check quietly first, then ask your questions, then say the result.

The atmee server is `https://api.atmanity.us/mcp`, and anything else
that reaches the Avatar API uses `ATMEE_BASE_URL=https://api.atmanity.us`.
A sign-in or key made against one server fails on any other.

## 0. Check for updates

Once per session, run `scripts/update-check` from this skill's folder (it
also stays quiet when it ran in the last 12 hours). It prints `skills` and
`cli` lines in the same `ok`, `fix` or `info` shape as step 3; an `info` line
(offline, no `gh`, a local build) needs nothing. Never wait on it or stop for
it: what is already installed keeps working.

- **`skills` is `fix`:** say which version is installed and which is out, and
  give the command it prints, once. A plugin update and the reload after it
  are the creator's; the skills CLI command you may run after a yes.
- **`cli` is `fix`:** ask in your question tool "Update the atmee CLI now?"
  with "Yes, update it (Recommended)" and "Not now", and on yes run the
  printed install command (it builds the release's `atmee` from source with
  Go; when `go` is missing, say the CLI needs Go from https://go.dev/dl and
  go on without it).

## 1. Find the server

Decide from what you can see right now, never from config files:

- **Connected** means you list an atmee tool and `get_balance` answers.
  With the atmee plugin the tools come from the server `atmee-avatars`;
  without it, from one server named `atmee`.
- **Listed but asking for sign-in** is connected but not signed in: go on to
  step 3; step 5 gives the sign-in step.
- **No atmee tools at all** means the server isn't added: go to step 2.

## 2. Add the server (only when it is missing)

Ask once, in your question tool (`AskUserQuestion` in Claude Code), or in one
chat line with the options when you have none:

"Add the atmee server to <your agent>? It makes the avatars, voices and calls
this skill uses. You sign in on atmee.ai afterwards." with "Yes, add it
(Recommended)" and "No".

In Codex the same question also says: "Codex will also be allowed to use the
network from its sandbox, which uploads and the update check need. This
applies to every Codex session on this machine."

**On no:** change nothing. Say that nothing can be made without the server:
no portraits, voices, avatars or test calls. The creator can run this again
any time. Stop here.

**On yes:** add it under the name `atmee`, at
`https://api.atmanity.us/mcp`, with your agent's own way: run
`scripts/detect-host` from this skill's folder and use the row it names
(`claude-code …`, `codex`, `cursor …`); on `unknown` or another name, judge
which row is yours. Before writing,
check that no `atmee` or `atmee-avatars` entry exists already; if one does, use
it and skip the write. Then tell the creator which file changed and the undo
line.

| Agent | Add | File | Undo |
|---|---|---|---|
| Claude Code | `claude mcp add --transport http atmee <url> --scope user` | `~/.claude.json` | `claude mcp remove atmee --scope user` |
| Codex | `codex mcp add atmee --url <url>` (it starts sign-in at once); then in `~/.codex/config.toml`, under `[sandbox_workspace_write]`, set `network_access = true` | `~/.codex/config.toml` | `codex mcp remove atmee`, and remove that setting |
| Cursor | add `"atmee": { "url": "<url>" }` under `mcpServers` in `~/.cursor/mcp.json` (create the file if missing, keep other entries), then `cursor-agent mcp enable atmee` and `cursor-agent mcp login atmee` | `~/.cursor/mcp.json` | remove the `atmee` entry |
| Antigravity | `agy mcp add` when the `agy` command exists (check `agy mcp add --help` for its flags); otherwise add `"atmee": { "serverUrl": "<url>" }` under `mcpServers` in `~/.gemini/config/mcp_config.json` | the command's config, or that file | `agy mcp remove atmee`, or remove the entry |

Then name the one step left and end your turn: "Reload <your agent> and sign
in to atmee when it asks (the "Connect an app to Atmee" page on atmee.ai), then
tell me to go on." On the next turn start again at step 1; trust only what you
can see then.

## 3. Run the checks

Do all of these before saying anything:

- **Sign-in.** Call `get_api_key`. A tool that fails asking for sign-in, or
  is missing, means the server is not signed in.
- **Account.** If `get_api_key` fails with `account_ambiguous`, the sign-in
  belongs to several accounts. Use the `accountId` in
  `atmee-avatars/settings.json` if it is one of the error's choices; otherwise
  ask in step 4.
- **Credits.** `spendableMicrocredits` from `get_balance`, divided by 100.
  It is the only balance to report; `v2SpendableMicrocredits` (where present)
  is the part usable on Atmee V2, worth a mention when it differs.
- **Local.** From the project folder, run `atmee-setup` when the command
  exists (Claude Code with the plugin; it also checks the plugin version). It
  prints one line per check: `ok`, `fix` or `info`, the check's name, then a
  message. Without it, in Claude Code with the `atmee` server instead of the
  plugin, check by hand whether `~/.claude/settings.json`
  mentions `"mcp__atmee"` anywhere (a missing rule is the `permissions` fix
  below, with that one rule).
- **Codex sandbox.** In Codex, if the update check or an upload fails
  reaching the network, the sandbox change from step 2 is missing: ask that
  part of the step 2 question on its own.

## 4. Ask first

Ask what only the creator can decide, in one call of your question tool,
before telling them anything else:

- Several accounts: "Which account should these avatars belong to?" with one
  option per account in the `account_ambiguous` error. Save the pick as
  `"accountId"` in `atmee-avatars/settings.json`, then run the sign-in check
  again with it.
- `permissions` is `fix`, unless `atmee-avatars/settings.json` has
  `"keepToolPrompts": true`: "Let atmee's tools run without Claude Code
  asking each time? create-avatar still asks you before anything that costs
  credits." with "Yes, stop the prompts (Recommended)" and "No, keep asking".
  Claude Code keeps a skill's pre-approved tools only until the creator's
  next message, so without this every later atmee call asks again.
  - On yes, edit the settings file the line names with your own file tools:
    add each rule it lists to `permissions.allow`, creating `permissions` and
    `allow` when missing, keeping every other entry, and leaving valid JSON.
    Then name the file and the undo: remove those entries.
  - On no, save `"keepToolPrompts": true` in `atmee-avatars/settings.json`
    and don't ask again in this project; `atmee-setup` then reports
    `permissions` as `info`.
  - Either way, `permissions` never holds back "All set": it is a choice,
    not a problem.

Without a question tool, ask it in one chat line with the options.

## 5. Say the result

**Everything fine:** one line, for example "All set: signed in to atmee,
<balance> credits." Then go on (back to create-avatar if it ran you).

**Something to do:** one line per problem, only the problems, each with its one
step. No table, no list of what passed.

| Problem | Step to give |
|---|---|
| Not signed in | If you have an `authenticate` tool for the server, call it and give the link. Otherwise: Claude Code, run `/mcp`, pick the server, choose Authenticate; Codex, run `codex mcp login <server>` in a terminal; Cursor, `cursor-agent mcp login <server>` or sign in under Settings > MCP; Antigravity, sign in from its MCP settings. Sign-in goes through the "Connect an app to Atmee" page on atmee.ai. |
| Tools still missing after adding | The agent hasn't reloaded or can't reach the server: `/mcp` in Claude Code, `codex mcp list` in Codex, `cursor-agent mcp list` in Cursor, or Antigravity's MCP settings show its status. |
| No credits left | Give the billing page on atmee.ai, where credits are topped up. |
| `plugin` is `fix` | A newer version is out: give the update steps it prints. You can't update the plugin yourself. |

Run again whenever the creator asks; every check is safe to repeat.
