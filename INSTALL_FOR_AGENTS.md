# Set up atmee (instructions for agents)

A person asked you to set up atmee: the skills and server that let them make
and test Atmee talking avatars by talking to you. Complete every step yourself
with your own tools. A step marked **Ask the person** needs them (a browser
sign-in, a new session): ask in one short line, wait until they say it's done, then
go on. Tell them what to do and what they'll see; don't explain how
atmee works inside.

What gets installed:

- Six skills: `create-avatar` (builds an avatar with them: a name, a photo
  and a voice, its first seed, then its character and a test), `make-video`
  (makes a short video clip from a picture, or of a saved avatar speaking),
  `plan-avatar-app` (shapes an app idea
  into a brief), `build-avatar-app` (helps them code an app or website
  around an avatar), `atmee-setup` (checks everything and fixes what it
  can) and `atmee-product` (what Atmee offers and costs, and which path fits
  an idea).
- One remote MCP server, `atmee`: it creates and updates avatars, takes uploads,
  reports the balance and generates portraits, voices and short videos: a
  clip of a picture moving as a prompt says, speaking a voice line when one
  is given, or of a saved avatar speaking a script (paid per call). The optional plugin adds the same
  server under the name `atmee-avatars`.

## 0. Before you start

- **Repository.** Everything installs from the public GitHub repo
  `Atmanity/atmee-toolkit`; no GitHub account or sign-in is needed. Check that this
  machine can reach it:

  ```
  git ls-remote https://github.com/Atmanity/atmee-toolkit.git HEAD
  ```

  If that fails, the machine can't reach GitHub (no network, a proxy or a
  firewall). Stop and tell the person: "This machine can't reach
  github.com/Atmanity/atmee-toolkit. Check the network or proxy settings and ask me
  again."
- **Already installed?** Skip to [step 6](#6-verify) only when you have all
  six skills (`create-avatar`, `make-video`, `plan-avatar-app`,
  `build-avatar-app`, `atmee-setup` and `atmee-product`) and you
  list `atmee` or `atmee-avatars` tools. Servers without the skills (for example added by hand) still
  need step 2, which installs or updates the skills.

## 1. Which agent are you?

Run this. It reads only environment variables your agent sets and prints
where you run (the skills carry the same check as `scripts/detect-host`):

<!-- detect-host:start -->
```
bash <<'EOF'
# Prints which agent runs this command, and where when it can tell:
# claude-code desktop, claude-code cli, claude-code ide, claude-code, codex,
# cursor editor, cursor cli, gemini-cli, opencode, or unknown (then judge
# for yourself). It reads only environment variables and always exits 0.
# Most specific first: a host started inside another inherits its variables.
# Variables are read by name, because skill checks reject the literal form.
v() { local n="$1"; printf '%s' "${!n-}"; }
if [ -n "$(v CODEX_THREAD_ID)$(v CODEX_CI)" ]; then
  echo codex
elif [ -n "$(v CURSOR_AGENT)" ]; then
  if [ "$(v CURSOR_EXTENSION_HOST_ROLE)" = agent-exec ]; then echo "cursor editor"; else echo "cursor cli"; fi
elif [ -n "$(v GEMINI_CLI)" ]; then
  echo gemini-cli
elif [ -n "$(v OPENCODE)" ]; then
  echo opencode
elif [ -n "$(v CLAUDE_CODE_CHILD_SESSION)$(v CLAUDECODE)" ]; then
  case "$(v CLAUDE_CODE_ENTRYPOINT)" in
    claude-desktop) echo "claude-code desktop" ;;
    claude-vscode) echo "claude-code ide" ;;
    cli) echo "claude-code cli" ;;
    *) echo claude-code ;;
  esac
else
  echo unknown
fi
EOF
```
<!-- detect-host:end -->

| It prints | You are | Skills agent id |
|---|---|---|
| `claude-code …` | Claude Code (terminal, IDE, or a local session in the desktop app's Code tab) | `claude-code` |
| `codex` | Codex (CLI, IDE or desktop) | `codex` |
| `cursor editor` or `cursor cli` | Cursor | `cursor` |
| `gemini-cli`, `opencode` | that agent | the same name |
| `unknown` | judge for yourself (Antigravity: `antigravity-cli`; others: `github-copilot`, `windsurf`, `cline`, …) | |

If it can't run (no shell), judge for yourself the same way.

**The plugin is optional.** These steps install the skills and add the
server, the same way in every agent. Use the plugin instead only when the
person asks for it, or when `claude plugin list` or `codex plugin list`
already shows `atmee@atmee`, or Cursor already has the `atmee` plugin
(`~/.cursor/plugins/local/atmee` exists, or Cursor's Plugins settings list
it): then go to [Plugin instead](#plugin-instead)
and don't also add the skills or the `atmee` server (two copies of every
tool).

## 2. Install the skills

```
npx -y skills add Atmanity/atmee-toolkit --skill '*' --agent <agent-id> --yes --global
```

It needs Node.js 22.20 or newer (`node --version`). Check it worked:

```
npx -y skills list --global
```

`create-avatar`, `make-video`, `plan-avatar-app`, `build-avatar-app`,
`atmee-setup` and `atmee-product` should be listed. Running the same command again updates
them.

**Claude Code without Node.js:** unzip each skill from the latest release
into its skills folder instead:

```
dir="${CLAUDE_CONFIG_DIR:-$HOME/.claude}/skills"; mkdir -p "$dir"
for s in create-avatar make-video plan-avatar-app build-avatar-app atmee-setup atmee-product; do
  tmp="$(mktemp -d)"
  # Replace an installed copy only once the new one downloaded and unpacked.
  if curl -fsSL "https://github.com/Atmanity/atmee-toolkit/releases/latest/download/$s.zip" -o "$tmp/$s.zip" &&
     unzip -q "$tmp/$s.zip" -d "$tmp/new" && [ -f "$tmp/new/$s/SKILL.md" ]; then
    rm -rf "$dir/$s" && mv "$tmp/new/$s" "$dir/$s"
  else
    echo "could not download $s; the installed copy, if any, is unchanged"
  fi
  rm -rf "$tmp"
done
```

## 3. Add the server

The server is `https://api.atmanity.us/mcp`, added under the name
`atmee`. First check that no `atmee` or `atmee-avatars` server is there
already; if one is, keep it and skip this step.

| Agent | Add it | Check |
|---|---|---|
| Claude Code | `claude mcp add --transport http atmee https://api.atmanity.us/mcp --scope user` | `claude mcp get atmee` says it needs authentication |
| Codex | It starts the sign-in at once, so first give the person step 4's line about the sign-in page. Then `codex mcp add atmee --url https://api.atmanity.us/mcp`, and in `~/.codex/config.toml`, under `[sandbox_workspace_write]`, set `network_access = true` (uploads and the update check need the network) | `codex mcp list` |
| Cursor | add `"atmee": { "url": "https://api.atmanity.us/mcp" }` under `mcpServers` in `~/.cursor/mcp.json` (create it if missing, keep other entries), then `cursor-agent mcp enable atmee` | `cursor-agent mcp list` |
| Antigravity | `agy mcp add` when `agy` exists (see `agy mcp add --help`); otherwise add `"atmee": { "serverUrl": "https://api.atmanity.us/mcp" }` under `mcpServers` in `~/.gemini/config/mcp_config.json` | its MCP settings |
| Any other agent | its own way to add a remote HTTP MCP server with that name and address | |

Tell the person which file changed (the table's command names it) and that
`claude mcp remove atmee --scope user`, `codex mcp remove atmee` or removing
the entry undoes it.

## 4. Sign in

The server signs in through the "Connect an app to Atmee" page on atmee.ai.
The login command opens it in the person's browser and waits until they
finish, so tell them first. Use the server's name from step 3: `atmee`, or
the `atmee-avatars` entry you kept (`<name>` below; a plugin's server signs
in as [Plugin instead](#plugin-instead) says). **Ask the person:** "An atmee
sign-in page will open in your browser. Sign in there; I'll carry on when
it's done."

Open it for them: the login commands refuse to run without a terminal
("this isn't an interactive terminal"), and your shell isn't one, so run the
command below inside one with `script`, in the background where your agent
can (or with a timeout of several minutes):

- macOS: `script -q /dev/null <command>`
- Linux: `script -qec "<command>" /dev/null`

It opens the page in the person's browser and finishes by itself once they
sign in. Wait for it to finish before step 5: a new session started earlier
ends the login. If no browser opened, give the person the link it printed.
Over SSH that link alone doesn't finish (the page sends the browser back to
`localhost` on the person's own machine): ask them to run their agent's own
login in their SSH terminal and follow its prompt. In Claude Code that is
`claude mcp login <name> --no-browser`: open the link it shows, then paste
the address they land on back into that prompt. Codex and Cursor sign in
only through their own command (`codex mcp login <name>`,
`cursor-agent mcp login <name>`), never Claude's.

| Agent | Sign in |
|---|---|
| Claude Code | `claude mcp login <name>`; or the person runs `/mcp` in Claude Code, picks the server and chooses Authenticate |
| Codex | nothing when `codex mcp list` already shows `<name>` signed in (the add in step 3 did it); otherwise `codex mcp login <name>` |
| Cursor | `cursor-agent mcp login <name>`, or ask the person to sign in under Settings > MCP |
| Others | ask the person to sign in from the agent's MCP settings |

If the login times out or fails, run it again.

## 5. Load it

A running session doesn't pick up a server added from the shell. **Ask the
person** to start a new one, which keeps this conversation where the agent
allows it:

- Claude Code: "Type `/exit`, then run `claude -c` in this folder, and tell
  me to go on." In the desktop app, start a new local session.
- Codex: "Restart Codex (quit the desktop app fully and reopen it), then
  tell me to go on."
- Cursor: "Restart Cursor, then tell me to go on."
- Others: "Start a new session, then tell me to go on."

## 6. Verify

Ask yourself to "set up atmee", which runs the `atmee-setup` skill
(`/atmee-setup` in Claude Code). It checks the sign-in, the account, the
credit balance, updates and Claude Code's prompts before each atmee tool
call, fixes what it can, and ends with one line.

- **"All set: …"** means the install worked. Go to step 7.
- Anything else is one line per problem with the one step to take. Do the
  steps you can, give the person the rest, and run the check again.

Without the setup skill, call `get_balance` on `atmee`: it is free, and a
balance in the answer means the server is signed in.

## 7. Finish

Tell the person, in one or two lines, for example:

> atmee is ready: signed in, 120 credits. Say "make me a talking avatar" to
> start: give it a name, a photo and a voice, and you'll see it talk within a
> few minutes.

Then call `get_account` (free) for the clip line. Clips are offered on this
server when its `pricing.generation` has `clipSizes` (an older server:
`clipPer15SecondsMicrocredits` or `clipPerSecondMicrocredits`); without any,
say nothing about clips. When they are offered and `limits.videoGeneration` is not
false (absent counts as allowed), add one line saying atmee can also make videos, for example:

> atmee can also make short videos: give a photo and a prompt, and you
> get an mp4 of them moving; add a voice line and they speak it. Say "make a
> video clip" to try it.

Clips are on every plan, Free included, and need no Atmee V2 access:
don't check `v2Access` for them. With `limits.videoGeneration` false,
video generation was turned off for this account: say nothing about clips.

## Plugin instead

The plugin bundles the same skills with the server under the name
`atmee-avatars`, and updates them together. It suits Cursor teams (an admin
imports it once) and people who prefer plugins. Plugins don't load in
Claude Code's cloud sessions.

**Claude Code.** See what is there with `claude plugin marketplace list` and
`claude plugin list`, then run the lines that apply:

- No `atmee` marketplace: `claude plugin marketplace add Atmanity/atmee-toolkit`
- The marketplace is there: `claude plugin marketplace update atmee`
- `atmee@atmee` is not installed: `claude plugin install atmee@atmee`
- `atmee@atmee` is installed: `claude plugin update atmee@atmee`

If `claude plugin list` shows it disabled, run `claude plugin enable atmee@atmee`.
**Ask the person:** "Type `/reload-plugins --force` to load atmee here, then
tell me to go on." (`--force` is needed because the plugin adds a server.)
Then sign in: until it is signed in, Claude Code gives you an `authenticate`
tool for the server (`mcp__plugin_atmee_atmee-avatars__authenticate`; load
it through your tool search when it is deferred). Call it and **ask the
person:** "Open this link and sign in to atmee.ai: <link>. Tell me when the
page says you're connected." The atmee tools appear by themselves once they
finish; when the person is in a remote session and the page they land on
fails to load, pass its address to the server's `complete_authentication`
tool. Without an `authenticate` tool, **ask the person:** "Run `/mcp`, pick
`atmee-avatars` and choose Authenticate" (a plugin's server isn't one
`claude mcp login` knows). Then go to [step 6](#6-verify), running
`/atmee:atmee-setup`.

**Codex.** `codex plugin marketplace add https://github.com/Atmanity/atmee-toolkit.git`
(or `codex plugin marketplace upgrade` when it is there), then
`codex plugin add atmee@atmee`. **Ask the person** to restart Codex, then
sign in with `codex mcp login atmee-avatars` as in step 4, and go to step 6.

**Cursor.** In a team, an admin opens **Plugins > Import from Repo**, imports
`Atmanity/atmee-toolkit`, and members install `atmee`. Alone, copy the plugin
into Cursor's local plugins:

```
src="$(mktemp -d)"
git clone --depth 1 https://github.com/Atmanity/atmee-toolkit.git "$src/toolkit"
mkdir -p ~/.cursor/plugins/local
rm -rf ~/.cursor/plugins/local/atmee
cp -R "$src/toolkit/plugins/atmee" ~/.cursor/plugins/local/atmee
rm -rf "$src"
```

Then **ask the person** to restart Cursor and sign in to `atmee-avatars`
under Settings > MCP, and go to step 6.

## If something goes wrong

| What you see | What to do |
|---|---|
| `git ls-remote` or a download fails | The machine can't reach GitHub. Check the network or proxy, as in step 0. The repo is public, so no GitHub sign-in is needed. |
| `npx skills` says Node.js is too old | Ask the person to install Node.js 22.20 or newer. In Claude Code, the unzip route in step 2 needs no Node. |
| No atmee tools after the new session | `claude mcp list`, `codex mcp list` or `cursor-agent mcp list` shows the server's status. Missing: add it as in step 3. Needs authentication: sign in as in step 4, then start a new session again. |
| `claude mcp login` says the server is unknown | It was added in another scope or under another name: `claude mcp list` shows it; sign in to that name. |
| `atmee-setup` says newer skills are out | Run the update it prints (for the skills: the step 2 command again), then start a new session. |
| With the plugin: tools missing after the reload | Ask the person to type `/reload-plugins --force` again, or to restart Claude Code; `/mcp` shows each server's status. |

## Remove atmee

- Skills: `npx -y skills remove create-avatar make-video plan-avatar-app build-avatar-app atmee-setup atmee-product --global --yes`
  (or delete those six folders from the skills folder).
- Server: `claude mcp remove atmee --scope user`, `codex mcp remove atmee`
  (and the `network_access` line if nothing else needs it), or remove the
  `atmee` entry from Cursor's or Antigravity's MCP file.
- Plugin: Claude Code `claude plugin uninstall atmee@atmee`, then
  `claude plugin marketplace remove atmee`; Codex `codex plugin remove atmee@atmee`,
  then `codex plugin marketplace remove atmee`; Cursor: uninstall `atmee`
  under Plugins, or `rm -rf ~/.cursor/plugins/local/atmee`, then restart.
