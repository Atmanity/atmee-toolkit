<p align="center">
  <a href="https://www.atmee.ai">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="assets/atmee-banner-dark.jpg">
      <source media="(prefers-color-scheme: light)" srcset="assets/atmee-banner-light.png">
      <img src="assets/atmee-banner-light.png" alt="atmee.ai" width="600">
    </picture>
  </a>
</p>

<h1 align="center">atmee toolkit</h1>

<p align="center">Build talking avatars from your coding agent or terminal, then build apps around them.</p>

<p align="center">
  <a href="https://github.com/Atmanity/atmee-toolkit/releases/latest"><img src="https://img.shields.io/github/v/release/Atmanity/atmee-toolkit?display_name=release&label=release" alt="Latest release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/Atmanity/atmee-toolkit" alt="License: MIT"></a>
  <a href="https://discord.gg/7nkQFhedKD"><img src="https://img.shields.io/badge/Discord-join-5865F2?logo=discord&logoColor=white" alt="Discord"></a>
</p>

<p align="center">
  <a href="https://www.atmee.ai">Website</a> ·
  <a href="https://www.atmee.ai/docs">Docs</a> ·
  <a href="#quickstart">Quick start</a> ·
  <a href="#mcp-server-only">MCP</a> ·
  <a href="plugins/atmee/CHANGELOG.md">Changelog</a> ·
  <a href="https://discord.gg/7nkQFhedKD">Discord</a>
</p>

Atmee turns a portrait and a voice sample into a conversational avatar that
talks with people in real time over video. This toolkit gives your coding agent
the skills and the MCP server to create, test and ship those avatars, and gives
your terminal the `atmee` CLI for the same Avatar API.

This repository holds:

- **Six agent skills** that make, test and ship avatars with your coding
  agent. They install into Claude Code, Codex, Cursor and any agent the
  [skills CLI](https://www.npmjs.com/package/skills) supports, and use the
  Avatar API's remote **MCP server** (`https://api.atmanity.us/mcp`).
- **The `atmee` CLI**: one Go binary that manages avatars, API keys and
  conversations from a terminal or CI.
- **An optional `atmee` plugin** for Claude Code, Codex and Cursor that
  bundles the same skills and server.

All of them talk to the same Atmee Avatar API. The MCP server signs in as you
through atmee.ai; the CLI signs in as you or uses an API key.


## Contents

- [What you can build](#what-you-can-build)
- [Quickstart](#quickstart)
- [Skills](#skills)
- [The atmee CLI](#the-atmee-cli)
- [Requirements](#requirements)
- [Documentation](#documentation)
- [Troubleshooting](#troubleshooting)
- [Updates](#updates)
- [Contributing](#contributing)
- [License](#license)

## What you can build

- **A talking avatar or digital twin** from a name, a portrait and a voice
  sample, with its own persona and knowledge.
- **Live video conversations** in your own website or app: your backend starts
  a session, your page joins the LiveKit room, and the avatar listens and
  answers in its own voice. Avatars can call tools your page defines.
- **Your own agent with an Atmee face**: render an avatar into a LiveKit room
  you control and drive it from your own conversation logic.
- **Generated inputs**: portraits from a description, an enhanced photo, and
  voices designed from a description.
- **Short video clips**: an mp4 of 1 to 15 seconds from a picture and a
  prompt, or of a saved avatar speaking a script in its own voice.

Anyone can talk to any avatar, and video clips are on every plan, Free included (they spend
credits). Building Atmee V2 avatars, and the features built on them (candidate clips, agendas),
needs Pro and up, and Atmee V2 HD needs Growth and up. Accounts below Pro get Atmee V1 avatars. Existing V2 and V2 HD
avatars keep chatting, sharing and embedding on any plan; editing them needs Pro and up for V2 and Growth
and up for V2 HD.

## Quickstart

The quickest route is to let your agent install everything. Paste this into
Claude Code, Codex, Cursor or any other coding agent:

```text
Set up atmee for me by following INSTALL_FOR_AGENTS.md in the GitHub repo Atmanity/atmee-toolkit.
```

The agent installs the skills, adds the MCP server and opens the atmee.ai
sign-in page, then asks you for one thing: to start a new session so it loads
the server. It finishes with a check that reports "All set" and your credit
balance. The full procedure is in [INSTALL_FOR_AGENTS.md](INSTALL_FOR_AGENTS.md).

### By hand

1. Install the skills (Node.js 22.20 or newer). Use your agent's id:
   `claude-code`, `codex`, `cursor`, `gemini-cli`, `github-copilot`,
   `windsurf`, `cline`, `opencode`, …

   ```bash
   npx -y skills add Atmanity/atmee-toolkit --skill '*' --agent AGENT_ID --yes --global
   ```

2. Start a new session and say "set up atmee". The `atmee-setup` skill adds
   the MCP server (`atmee`, at `https://api.atmanity.us/mcp`), opens the
   atmee.ai sign-in page and asks for one more new session to load the
   server. Then ask your agent to make an avatar.

### Plugin (optional)

The plugin bundles the same skills and the server (named `atmee-avatars`),
updated together. Install it instead of steps 1 and 2, not as well.

| Agent | Commands |
|---|---|
| Claude Code | `claude plugin marketplace add Atmanity/atmee-toolkit`, `claude plugin install atmee@atmee`, then `/reload-plugins --force` and sign in from `/mcp` > `atmee-avatars` (local sessions only, not cloud sessions) |
| Codex | `codex plugin marketplace add https://github.com/Atmanity/atmee-toolkit.git`, `codex plugin add atmee@atmee`, restart, then `codex mcp login atmee-avatars` |
| Cursor | a team admin imports `Atmanity/atmee-toolkit` under **Plugins > Import from Repo** and members install `atmee`; on your own, see [INSTALL_FOR_AGENTS.md](INSTALL_FOR_AGENTS.md#plugin-instead). Restart, then sign in under **Settings > MCP** |


### MCP server only

Any MCP client that supports remote servers with OAuth can use the Avatar
API's MCP server directly:

```text
https://api.atmanity.us/mcp
```

For example, `claude mcp add --transport http atmee https://api.atmanity.us/mcp --scope user`,
or in claude.ai, add it as a custom connector under **Settings > Connectors**.
The first connection signs you in on atmee.ai ("Connect an app to Atmee").
The server creates and updates avatars, takes file uploads, reports your
balance, and generates portraits, voices and video clips. Generation calls are paid; the
server's `get_account` tool reports the current prices.

### claude.ai

1. Download a skill from the
   [latest release](https://github.com/Atmanity/atmee-toolkit/releases/latest), for
   example [`create-avatar.zip`](https://github.com/Atmanity/atmee-toolkit/releases/latest/download/create-avatar.zip).
   The others are `make-video.zip`, `build-avatar-app.zip`, `plan-avatar-app.zip`,
   `atmee-product.zip` and `atmee-setup.zip`.
2. Upload it under **Settings > Capabilities > Skills**.
3. Add the MCP server `https://api.atmanity.us/mcp` as a custom connector under
   **Settings > Connectors**.

### CLI

```bash
curl -fsSL https://raw.githubusercontent.com/Atmanity/atmee-toolkit/main/install.sh | sh
atmee login
atmee balance
```

See [The atmee CLI](#the-atmee-cli) below.

## Skills

| Skill | What it does |
|---|---|
| [`create-avatar`](plugins/atmee/skills/create-avatar/SKILL.md) | Builds an avatar with you: name, photo and voice, see it talk, then give it a character and test it. |
| [`make-video`](plugins/atmee/skills/make-video/SKILL.md) | Makes one short video clip from a picture (any length up to 15 seconds), silent or with a voice recording, or of a saved avatar speaking a script. |
| [`plan-avatar-app`](plugins/atmee/skills/plan-avatar-app/SKILL.md) | Shapes an app idea into a one-page brief by asking in rounds: user, flow, persona, tools, screen, limits, live test. |
| [`build-avatar-app`](plugins/atmee/skills/build-avatar-app/SKILL.md) | Helps you code an app or website around an avatar: backend sessions, joining the call, app state, events, tools, files, agendas. |
| [`atmee-setup`](plugins/atmee/skills/atmee-setup/SKILL.md) | Connects and checks what the others need: the server, sign-in, account, credits, updates and tool prompts. |
| [`atmee-product`](plugins/atmee/skills/atmee-product/SKILL.md) | Tells the agent what Atmee offers and costs (read live from your account), and which path fits an idea. |

With the plugin, the skills are namespaced, for example `/atmee:create-avatar`.

## The atmee CLI

The CLI covers the same Avatar API as the MCP server, for scripts, CI and
backends. Its source is in [`cli/`](cli).

```bash
curl -fsSL https://raw.githubusercontent.com/Atmanity/atmee-toolkit/main/install.sh | sh
```

It downloads the prebuilt binary for your machine from the
[latest release](https://github.com/Atmanity/atmee-toolkit/releases/latest),
checks it against the release's checksums and puts it in `~/.local/bin`
(`ATMEE_INSTALL_DIR=<folder>` to change that, `ATMEE_VERSION=0.10.18` to pin a
release). Run the same command to upgrade. On Windows, download
`atmee_windows_amd64.zip` from the release. With Go, you can build it instead:
`go install github.com/Atmanity/atmee-toolkit/cli/cmd/atmee@latest` (each
release is tagged `cli/vX.Y.Z`).

Sign in with your browser, or use an API key:

```bash
atmee login                  # browser sign-in; --no-browser prints the link instead
atmee login --api-key        # or save an API key (prompted, hidden)
export ATMEE_API_KEY=...     # or set one for this shell; it wins over a saved sign-in
atmee whoami
```

Create an avatar and talk to it:

```bash
atmee avatars create --name Mira --image ./mira.jpg --voice ./mira.wav \
  --who-prompt "A cheerful urban gardener." --wait
atmee avatars session chat AVATAR_ID --max-seconds 300 --open
```

`create` prints the new avatar's id; use it as `AVATAR_ID`. `--open` joins the
conversation in LiveKit Meet in your browser. Join within 60 seconds of the
start, or the session ends.

| Command | Purpose |
|---|---|
| `atmee login` / `whoami` / `logout` | Sign in, show who the commands act as, sign out |
| `atmee balance` / `account` / `earnings` | Credits, plan and prices, creator earnings |
| `atmee keys` | Create, update, rotate and revoke API keys |
| `atmee avatars create` / `list` / `get` / `wait` / `update` / `delete` | Manage avatars |
| `atmee avatars replace-appearance` / `replace-voice` / `replace-knowledge` | Replace an avatar's inputs |
| `atmee avatars candidates` | v2: compare the talking clips built for an avatar and pick one |
| `atmee avatars session chat` / `render` | Start a conversation, or render the avatar into your own LiveKit room |
| `atmee generate portraits` / `enhance` / `voice` / `clip` | Generate portraits, voices and video clips |

Every command has `--help`, and `--json` prints the API's JSON. The full
reference, including every flag, environment variable and the config file
format, is in [docs/cli.md](docs/cli.md).

## Requirements

- **An Atmee account** with credits. Sign up at [atmee.ai](https://www.atmee.ai).
- **Plugin**: a recent Claude Code, Codex or Cursor.
- **Skills CLI**: Node.js 22.20 or newer.
- **CLI**: nothing for the prebuilt binary `install.sh` fetches; Go 1.21 or
  newer only to build it from source. `atmee avatars candidates sheet` also
  needs `ffmpeg`.
- **Portrait and voice**: a front-facing photo of one person, head and
  shoulders, lips slightly parted; and 10 to 60 seconds of clear speech by one
  speaker. The skills and `atmee generate` can make either for you.

## Documentation

| Resource | Link |
|---|---|
| Atmee developer docs | [www.atmee.ai/docs](https://www.atmee.ai/docs) |
| MCP server | `https://api.atmanity.us/mcp` |
| Avatar API base URL | `https://api.atmanity.us` |
| CLI reference | [docs/cli.md](docs/cli.md) |
| Agent install guide | [INSTALL_FOR_AGENTS.md](INSTALL_FOR_AGENTS.md) |
| Changelog | [plugins/atmee/CHANGELOG.md](plugins/atmee/CHANGELOG.md) |

## Troubleshooting

| Symptom | Fix |
|---|---|
| No atmee tools in the agent | A running session doesn't load a server added from the shell: start a new one. Then `claude mcp list`, `codex mcp list` or `cursor-agent mcp list` shows the server's status: missing, add it (Quickstart, step 2); needs authentication, `claude mcp login atmee` or `codex mcp login atmee`. |
| With the plugin, Claude Code lists it but no atmee tools | Run `/reload-plugins --force`, or restart Claude Code. `/mcp` shows each server's status. |
| `npx skills` says Node.js is too old | Install Node.js 22.20 or newer, or use the plugin. In Claude Code, you can also unzip each skill from the [latest release](https://github.com/Atmanity/atmee-toolkit/releases/latest) into `~/.claude/skills`. |
| `v2_access_required` (or `insufficient_v2_credits` from an older server) | The account can't build new Atmee V2 avatars: Atmee V2 needs Pro and up. Upgrade to Pro, or make a V1 avatar. Existing V2 avatars keep working; talking to any avatar never needs it. |
| `v2hd_access_required` | The account can't build new Atmee V2 HD avatars: Atmee V2 HD needs Growth and up. Upgrade to Growth, or make a V2 avatar. |
| `v2_view_only`, `v2hd_view_only` | An edit may be refused when the account's plan doesn't cover the model; the message says which plans do. See plans in Creator Studio (Billing). |
| `402 insufficient_credits` | Top up in Creator Studio, or raise the API key's spend cap. |
| `409 avatar_not_ready` | The avatar has no ready clip yet. Check with `atmee avatars get AVATAR_ID` or wait with `atmee avatars wait AVATAR_ID`. |
| A build fails at `appearance` | Use a front-facing portrait with the whole head in frame, the face at least a third of it, and the lips slightly parted. |
| A build fails at `voice` | Use 10 to 60 seconds of one person speaking, with no music. |
| "Could not read who this is from the API" | Check `ATMEE_BASE_URL` (unset means production) and run `atmee login` again. |

In any agent, the `atmee-setup` skill runs the same checks and tells you the
one step to take for anything it can't fix.

## Updates

The skills check for a newer release once a session and print the command to
update with. Releases are listed under
[Releases](https://github.com/Atmanity/atmee-toolkit/releases), and the
[changelog](plugins/atmee/CHANGELOG.md) says what changed in each.

## Contributing

This repository is a published copy: each release replaces its contents, so
pull requests here can't be merged. Please
[open an issue](https://github.com/Atmanity/atmee-toolkit/issues) for bugs, questions
and feature requests, and we'll make the change at the source.

## License

[MIT](LICENSE)
