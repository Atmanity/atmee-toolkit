
# Set up atmee

## Contents

- 0. Check for updates
- 1. Find the server
- 2. Add the server (only when it is missing)
- 3. Run the checks
- 4. Ask first
- 5. Say the result

You connect the atmee server if it is missing, check everything the
create-avatar and make-video skills need, fix what you can, and give the
creator the one step for each thing only they can do. Those skills run this
first; the creator can also run it on its own.

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
- **`cli` is not installed (an `info` line):** the CLI is optional; avatars,
  clips and apps all work through the server. Leave it out of the result,
  unless the creator is building an app backend or working from a shell: then
  offer the printed install command once.
- **`cli` is `fix`:** ask in your question tool "Update the atmee CLI now?"
  with "Yes, update it (Recommended)" and "Not now", and on yes run the
  printed install command. It downloads the release's prebuilt `atmee`
  (no Go needed); only a `go install` line, printed when the plain download
  can't reach the release, builds from source: when `go` is missing then,
  say the CLI needs Go from https://go.dev/dl and go on without it.

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
no portraits, voices, avatars, clips or test calls. The creator can run this again
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
| Cursor | add `"atmee": { "url": "<url>" }` under `mcpServers` in `~/.cursor/mcp.json` (create the file if missing, keep other entries), then `cursor-agent mcp enable atmee` (the sign-in follows below) | `~/.cursor/mcp.json` | remove the `atmee` entry |
| Antigravity | `agy mcp add` when the `agy` command exists (check `agy mcp add --help` for its flags); otherwise add `"atmee": { "serverUrl": "<url>" }` under `mcpServers` in `~/.gemini/config/mcp_config.json` | the command's config, or that file | `agy mcp remove atmee`, or remove the entry |

Then sign in from the shell where the agent has a login command, so the
server is ready when it loads. Use the name of the server you added or kept
(`atmee`, or an `atmee-avatars` entry that was there already; a plugin's
server is signed in through `/mcp` or its `authenticate` tool instead, see
step 5): tell the creator "An atmee sign-in page will open in your browser;
sign in there", then open it for them by running `claude mcp login <server>`
(Claude Code), `codex mcp login <server>` (Codex, only when `codex mcp list`
doesn't show it signed in: the add already started the sign-in) or
`cursor-agent mcp login <server>` (Cursor). These refuse to run without a
terminal and your shell isn't one, so wrap the command: `script -q /dev/null
<command>` on macOS, `script -qec "<command>" /dev/null` on Linux; run it in
the background where you can (or with a timeout of several minutes). It
opens the page and finishes once the creator signs in: wait for that before
asking for a new session, which would end the login. If no browser opened,
give the creator the link it printed. Over SSH that link alone doesn't
finish: ask them to run your agent's own login in their SSH terminal and
follow its prompt (Claude Code: `claude mcp login <server> --no-browser`,
then paste the address they land on back into it; Codex and Cursor: their
own login command above, never Claude's). A running session doesn't
load a server added from the shell, so name the one step left and end your
turn: "Start a new session (in Claude Code: `/exit`, then `claude -c` here;
otherwise restart <your agent>), then tell me to go on." Without a login
command, add "and sign in to atmee when it asks (the "Connect an app to
Atmee" page on atmee.ai)". On the next turn start again at step 1; trust only
what you can see then.

## 3. Run the checks

Do all of these before saying anything:

- **Sign-in.** Call `get_api_key`. A tool that fails asking for sign-in, or
  is missing, means the server is not signed in.
- **Account.** If `get_api_key` fails with `account_ambiguous`, the sign-in
  belongs to several accounts. Use the `accountId` in
  `atmee-avatars/settings.json` if it is one of the error's choices; otherwise
  ask in step 4.
- **Credits.** `spendableMicrocredits` from `get_balance`, divided by 100.
  It is the only balance to report: one kind of credit pays for every model.
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
  asking each time? The atmee skills still ask you before anything that
  costs credits." with "Yes, stop the prompts (Recommended)" and "No, keep asking".
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
<balance> credits." Then go on (back to the skill that ran you, if one did).

**Something to do:** one line per problem, only the problems, each with its one
step. No table, no list of what passed.

| Problem | Step to give |
|---|---|
| Not signed in | If you have an `authenticate` tool for the server, call it and give the link. Otherwise open the sign-in page for them (say so first): run the login wrapped in `script` as step 2 says (`claude mcp login <server>`, `codex mcp login <server>` or `cursor-agent mcp login <server>`), and give the link it prints if no browser opens; in Claude Code the creator can also run `/mcp`, pick the server and choose Authenticate; Cursor, also Settings > MCP; Antigravity, sign in from its MCP settings. Sign-in goes through the "Connect an app to Atmee" page on atmee.ai. |
| Tools still missing after adding | The agent hasn't reloaded or can't reach the server: `/mcp` in Claude Code, `codex mcp list` in Codex, `cursor-agent mcp list` in Cursor, or Antigravity's MCP settings show its status. |
| No credits left | Give the billing page on atmee.ai, where credits are topped up. |
| `plugin` is `fix` | A newer version is out: give the update steps it prints. You can't update the plugin yourself. |

Run again whenever the creator asks; every check is safe to repeat.
