# Set up atmee (instructions for agents)

A person asked you to set up atmee: the skills and server that let them make
and test Atmee talking avatars by talking to you. Complete every step yourself
with your own tools. A step marked **Ask the person** needs them (loading the
plugin, a browser sign-in): ask in one short line, wait until they say it's done, then
go on. Tell them what to do and what they'll see; don't explain how the
plugin works inside.

What gets installed:

- Five skills: `create-avatar` (builds an avatar with them: a name, a photo
  and a voice, its first seed, then its character and a test), `plan-avatar-app` (shapes an app idea
  into a brief), `build-avatar-app` (helps them code an app or website
  around an avatar), `atmee-setup` (checks everything and fixes what it
  can) and `atmee-product` (what Atmee offers and costs, and which path fits
  an idea).
- One server, `atmee-avatars`: it creates and updates avatars, takes uploads,
  reports the balance and generates portraits and voices (paid per call).
  The skills-only route adds the same server under the name `atmee`.

## 0. Before you start

- **Repository.** Everything installs from the public GitHub repo
  `Atmanity/toolkit`; no GitHub account or sign-in is needed. Check that this
  machine can reach it:

  ```
  git ls-remote https://github.com/Atmanity/toolkit.git HEAD
  ```

  If that fails, the machine can't reach GitHub (no network, a proxy or a
  firewall). Stop and tell the person: "This machine can't reach
  github.com/Atmanity/toolkit. Check the network or proxy settings and ask me
  again."
- **Already installed?** Skip to [step 4](#4-verify) only when you have all
  five skills (`create-avatar`, `plan-avatar-app`, `build-avatar-app`,
  `atmee-setup` and `atmee-product`) and you
  list `atmee-avatars` or `atmee` tools. Servers without the skills (for example added by hand) still
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

| It prints | You are | Go to |
|---|---|---|
| `claude-code …` | Claude Code (terminal, IDE, or a local session in the desktop app's Code tab) | [2a. Claude Code](#2a-claude-code) |
| `codex` | Codex (CLI, IDE or desktop) | [2b. Codex](#2b-codex) |
| `cursor editor` or `cursor cli` | Cursor | [2c. Cursor](#2c-cursor) |
| `gemini-cli`, `opencode` | another agent | [2d. Other agents](#2d-other-agents) |
| `unknown` | judge for yourself: Claude Code, Codex and Cursor use their rows; anything else (Antigravity, Copilot, Windsurf, Cline, …) goes to 2d | |

If it can't run (no shell), judge for yourself the same way.

## 2a. Claude Code

First see what is there already:

```
claude plugin marketplace list
claude plugin list
```

Then run the lines that apply:

- No `atmee` marketplace: `claude plugin marketplace add Atmanity/toolkit`
- The marketplace is there: `claude plugin marketplace update atmee`
- `atmee@atmee` is not installed: `claude plugin install atmee@atmee`
- `atmee@atmee` is installed: `claude plugin update atmee@atmee`

Check it worked:

```
claude plugin list | grep -A4 'atmee@atmee'
```

The plugin should show as enabled. If it shows as disabled, run
`claude plugin enable atmee@atmee`. The plugin has no settings: the server
address is fixed in it.

Claude Code loads the plugin into this session without a restart. You can't
type slash commands yourself, so ask once, together with the sign-in:

**Ask the person:** "Type `/reload-plugins --force` to load atmee here. Then
run `/mcp`, pick `atmee-avatars` and choose Authenticate. Tell me when you're
done."

`--force` is needed because the plugin adds a server: without it Claude Code
holds it back until the next session. In the desktop app, use a local
session: plugins don't load in cloud sessions.

Only if Claude Code doesn't know `/reload-plugins` (an older version), or
`/mcp` still lists no atmee server after it, fall back to a restart:
**Ask the person:** "Type `/exit`, run `claude -c` in this folder to reopen
this conversation, and tell me to go on." Then ask for the sign-in as in
[step 3](#3-sign-in).

Then go to [step 4](#4-verify).

## 2b. Codex

First see what is there already:

```
codex plugin marketplace list
codex plugin list
```

Then run the lines that apply:

- No `atmee` marketplace:
  `codex plugin marketplace add https://github.com/Atmanity/toolkit.git`
- The marketplace is there: `codex plugin marketplace upgrade`
- `atmee@atmee` is not installed, or an upgrade brought a new version:
  `codex plugin add atmee@atmee`

Check it worked:

```
codex plugin list
```

`atmee@atmee` should be listed as installed and enabled, and `codex mcp list`
should show `atmee-avatars` as "Not logged in" until step 3.

**Ask the person:** "Restart Codex, then tell me to go on." In the Codex
desktop app they need to quit it fully and reopen it.

Then go to [step 3](#3-sign-in).

## 2c. Cursor

Cursor installs plugins from its own settings, so this one needs the person.

**Ask the person** to do one of these, then restart Cursor:

- In a team: an admin opens Cursor's **Plugins > Import from Repo**, imports
  `Atmanity/toolkit`, and members install `atmee`.
- Alone: you can do this part. Copy the plugin into Cursor's local plugins:

  ```
  src="$(mktemp -d)"
  git clone --depth 1 https://github.com/Atmanity/toolkit.git "$src/toolkit"
  mkdir -p ~/.cursor/plugins/local
  rm -rf ~/.cursor/plugins/local/atmee
  cp -R "$src/toolkit/plugins/atmee" ~/.cursor/plugins/local/atmee
  rm -rf "$src"
  ```

  Running it again replaces the installed copy with the latest one.

  Then ask the person to restart Cursor.

Then go to [step 3](#3-sign-in).

## 2d. Other agents

Install the skills with the skills CLI (needs Node.js 22.20 or newer; check
with `node --version`). Use your own agent id, for example `codex`, `cursor`,
`claude-code`, `antigravity-cli`, `gemini-cli`, `github-copilot`, `windsurf`,
`cline` or `opencode`:

```
npx -y skills add Atmanity/toolkit --skill '*' --agent <agent-id> --yes --global
```

Check it worked:

```
npx -y skills list --global
```

`create-avatar`, `plan-avatar-app`, `build-avatar-app`, `atmee-setup` and `atmee-product` should be listed. This route installs no
server: the `atmee-setup` skill adds the `atmee` server to your agent the
first time it runs, asks the person first, and says which file it changed and
how to undo it.

**Ask the person:** "Start a new session of your agent, then tell me to go
on." New skills load only in a new session.

Then go to [step 4](#4-verify): the setup skill handles the sign-in itself.

## 3. Sign in

The server signs in through the "Connect an app to Atmee" page on atmee.ai.

- **Claude Code.** Step 2a asked for it together with the reload. After a
  restart instead, **ask the person:** "Run `/mcp`, pick `atmee-avatars` and
  choose Authenticate."
- **Codex.** The login opens the sign-in page in the person's browser and
  waits until they finish there, so tell them first. **Ask the person:** "An
  atmee sign-in page will open in your browser. Sign in there and tell me
  when you're ready." Then run:

  ```
  codex mcp login atmee-avatars
  ```

  If the login times out or fails, run it again.
- **Cursor. Ask the person:** "Open Settings > MCP and sign in to
  `atmee-avatars`."

## 4. Verify

Run the setup skill: `/atmee:atmee-setup` in Claude Code with the plugin,
otherwise ask yourself to "set up atmee", which runs `atmee-setup`. It checks
the sign-in, the account, the credit balance, updates,
and Claude Code's prompts before each atmee tool call, fixes what it can,
and ends with one line.

- **"All set: …"** means the install worked. Go to step 5.
- Anything else is one line per problem with the one step to take. Do the
  steps you can, give the person the rest, and run the check again.

Without the setup skill, call `get_balance` on `atmee-avatars`: it is free,
and a balance in the answer means the server is signed in.

## 5. Finish

Tell the person, in one or two lines, for example:

> atmee is ready: signed in, 120 credits. Say "make me a talking avatar" to
> start: give it a name, a photo and a voice, and you'll see it talk within a
> few minutes.

## If something goes wrong

| What you see | What to do |
|---|---|
| `git ls-remote` or the marketplace add fails | The machine can't reach GitHub. Check the network or proxy, as in step 0. The repo is public, so no GitHub sign-in is needed. |
| Claude Code lists the plugin but no atmee tools | Ask the person to type `/reload-plugins --force`, or to restart Claude Code if that doesn't help; if the tools are still missing, `/mcp` shows each server's status. |
| Codex shows no atmee tools after a restart | Run `codex mcp list`. If `atmee-avatars` is missing, add it with the plugin's own address: `codex mcp add atmee-avatars --url https://api.atmanity.us/mcp`. Then sign in as in step 3. |
| `npx skills` says Node.js is too old | Ask the person to install Node.js 22.20 or newer, or use the plugin route if their agent has one. |
| `atmee-setup` says a newer plugin is out | Claude Code: `claude plugin marketplace update atmee`, then `claude plugin update atmee@atmee`, then the person types `/reload-plugins` (a restart works too). Codex: `codex plugin marketplace upgrade`, then `codex plugin add atmee@atmee`, then a restart. |

## Remove atmee

- Claude Code: `claude plugin uninstall atmee@atmee`, then
  `claude plugin marketplace remove atmee`.
- Codex: `codex plugin remove atmee@atmee`, then
  `codex plugin marketplace remove atmee`.
- If you added `atmee-avatars` to Codex by hand (troubleshooting), also run
  `codex mcp remove atmee-avatars`.
- Cursor: uninstall `atmee` under Plugins, or for the local copy run
  `rm -rf ~/.cursor/plugins/local/atmee`, then restart Cursor.
- Skills route: `npx -y skills remove create-avatar plan-avatar-app build-avatar-app atmee-setup atmee-product --global --yes`,
  and remove the `atmee` server the setup skill added (it said which file).
