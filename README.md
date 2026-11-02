# atmee toolkit

Make [Atmee](https://www.atmee.ai) talking avatars by talking to your coding
agent, then build apps around them. This repo holds the **atmee plugin** for
Claude Code, Codex and Cursor (its skills work in any agent through the skills
CLI) and the **`atmee` CLI**.

- `create-avatar` builds an avatar with you: give it a name, a photo and a
  voice, see it talk, then give it a character and test it.
- `plan-avatar-app` shapes an app idea into a one-page brief.
- `build-avatar-app` helps you code an app or website around an avatar: a
  backend that starts sessions, a page that joins the call, the app's state
  and events, an agenda.
- `atmee-setup` connects and checks everything the others need.
- `atmee-product` tells the agent what Atmee offers and costs, and which path
  fits an idea.

The plugin also adds one server, `atmee-avatars`, which creates and updates
avatars, takes uploads, reports your balance and generates portraits and
voices (paid per call).

## Install

Paste this into Claude Code, Codex, Cursor or any other coding agent:

```text
Set up atmee for me by following INSTALL_FOR_AGENTS.md in the GitHub repo Atmanity/toolkit.
```

The agent works out which agent it is, installs the plugin or the skills, and
stops only for what needs you: loading the plugin and signing in on atmee.ai.
It finishes with a check that says "All set" and your credit balance.

**Or install it yourself:**

| Agent | Commands |
|---|---|
| Claude Code | `claude plugin marketplace add Atmanity/toolkit`, then `claude plugin install atmee@atmee`, then `/reload-plugins --force` in Claude Code and sign in with `/mcp` |
| Codex | `codex plugin marketplace add https://github.com/Atmanity/toolkit.git`, then `codex plugin add atmee@atmee`, then restart and `codex mcp login atmee-avatars` |
| Cursor | Plugins > Import from Repo with `Atmanity/toolkit`, or see [INSTALL_FOR_AGENTS.md](INSTALL_FOR_AGENTS.md#2c-cursor) |
| Any agent | `npx -y skills add Atmanity/toolkit --skill '*' --agent <agent-id> --yes --global` (Node.js 22.20 or newer) |

Then ask your agent to make an avatar, or run `atmee-setup` first
(`/atmee:atmee-setup` with the plugin).

## The `atmee` CLI

The CLI manages avatars, API keys and sessions from a terminal or CI. Its
source is in [`cli/`](cli). Install it with Go 1.21 or later (Go fetches the
version the CLI needs):

```bash
go install github.com/Atmanity/toolkit/cli/cmd/atmee@latest
atmee login
```

The binary lands in `$(go env GOPATH)/bin` (usually `~/go/bin`), which must be on your
`PATH`. Run `go install` again to upgrade.
Every command and flag: [docs/cli.md](docs/cli.md).

## Updates

The skills check for a newer release once a session and print the command to
update with. Each release is listed under
[Releases](https://github.com/Atmanity/toolkit/releases), and
[CHANGELOG.md](plugins/atmee/CHANGELOG.md) says what changed.

## About this repo

This repo is published automatically with each release. Changes made here are
overwritten by the next one, so please open issues rather than pull requests.

## License

[MIT](LICENSE)
