---
name: build-avatar-app
description: Code an app or website around an Atmee avatar - backend sessions, joining the call, app state, events, tools, files, agendas. Use when building, embedding or debugging one. NOT for making the avatar.
license: Proprietary
metadata:
  owner: Atmanity
allowed-tools: Bash(*/update-check*)
---

# Build an app around an Atmee avatar

You help a developer put a live Atmee avatar inside their own product: a page
where the avatar talks with their user and knows what the app knows. An app
has five parts:

1. **Their backend** starts the avatar's session with an API key and hands the
   page a LiveKit room and token (`references/server.md`).
2. **Their page** joins that room as the user, shows the avatar and its
   captions, and takes the user's voice, typing or files
   (`references/room.md`).
3. **The app steers the avatar** through the room: `state` (what is true
   now), `note`, `event` (something to remark on), `mute`, and typed turns
   (`references/room.md`).
4. **The avatar acts on the app** through app tools: functions the backend
   declares for the session and the page runs when the avatar calls them
   (`atmee.tool`, `references/server.md` and `references/room.md`).
5. **An agenda**, when the conversation has steps: a conductor graph on the
   avatar (`references/conductor.md`).

Steering and tools go opposite ways. `state`, `note` and `event` are the app
telling the avatar something; a tool is the avatar making the app do
something (turn the slide, open the cart) or fetch something it needs to
answer (the order's status, the user's score).

Build from the starter in `templates/`: `server.mjs` (the backend's two
routes), `avatar-room.js` with `floor.mjs` (the page's half), `try.html` (a
test page) and `app-tools.example.json` (two sample tools `try.html` answers).
They were proven in live sessions, including the timing rules in
`references/room.md` that are easy to get wrong.

## Rules that always apply

1. **The API key stays on the server.** The page receives only `serverUrl`,
   `userToken` and `sessionId`. Never put the key in client code, a URL, a
   log or the chat; create it straight into a file (`references/server.md`,
   Keys).
2. **Start on a click, join at once.** The user must be in the room within 60
   seconds of the avatar or the session fails (`no_user_joined`), and
   browsers play sound only after a gesture (when the start took long, one
   more tap: `references/room.md`, Joining).
3. **One live conversation per avatar.** A second start answers `409
   avatar_busy`. End every session (`POST /v1/session/{id}/end`, also when
   the page closes); several users at once need several avatars.
4. **Test sessions cost credits.** Before the first one, read the price per
   minute for the avatar's engine (`get_account`:
   `pricing.sessionPerMinuteMicrocredits` for v1,
   `pricing.sessionV2PerMinuteMicrocredits` for v2, the first where the second
   is absent) and the
   balance (`get_balance`: `spendableMicrocredits`), each divided by 100 for
   credits, say both, and ask. One yes covers the short tests of this piece
   of work, since building an app takes many; ask again before a long test or
   when the balance runs low. Keep tests short with `maxDurationSeconds`.
   A link for someone who only wants to talk to the avatar is not a test of
   the app: give its chat page, `links.chat` from `get_avatar` (the API's
   response, `GET /v1/avatars/{id}`), which costs no session here. Start a
   session for a link only when they ask for a LiveKit room by name. Other
   website pages (Studio, API keys, billing, …) are `links` on the avatar
   and on `get_account`: the `atmee-product` skill's Website pages.
5. **Room messages, files, tools and agendas need Causeur**, the runtime
   that serves v2 sessions on the plugin's server. A session the
   behavior module runs accepts tools and never calls them. Anywhere else,
   the live test below shows whether `state`, `event` and tool calls reach
   the app before you build on them.
6. **Done means seen live.** A part works when you watched it work in a real
   session (its captions, the session's end state), not when it compiles.
   Quote what the avatar said when you report.

Ask the developer only what the code and the request can't tell you, in your
question tool, your recommendation first.

## Environment

Avatars, keys and sign-ins exist on one server only: a key or avatar sent to
another answers `401` or `404`. Every host the app, the CLI and your tests use
is the one below.

| Variable | Value | Read by |
|---|---|---|
| `ATMEE_BASE_URL` | `https://api.atmanity.us` | the app's backend (the starter defaults to it) and the `atmee` CLI: export it before `atmee login` and every `atmee` command |
| `ATMEE_API_KEY` | written by `atmee keys create --out .env`, never typed or shown | the backend only |
| `AVATAR_ID` | the avatar from step 1 | the backend |
| `MAX_SECONDS`, `APP_TOOLS_FILE`, `PORT` | optional: 300, none, 3000 | the starter backend |

When porting the backend, read `ATMEE_BASE_URL` from the environment like the
starter does, with that value as the fallback, and put it in the developer's
`.env` (and `.env.example`) next to the key. The atmee MCP server is
`https://api.atmanity.us/mcp`. Website pages (chat, Studio, API keys) are
the `links` the API returns; never build them from the API host, which serves
no pages.

## Flow

Once per session, first run the update check in the `atmee-setup` skill (its
step 0), and go on whatever it says.

### 1. Pick the avatar

Call `list_avatars` (the atmee avatars server; run `atmee-setup` if its tools
are missing). The app needs one that is `ready`, `readyToChat`, engine `v2`.
None fits: make one with the `create-avatar` skill first. v2 is invite-only for
now: when `get_account`'s `v2Access` is `invite_required`, the account needs an
invite code before it can make one: ask the user for it and redeem it with
`redeem_code`.

The persona is the avatar's: it greets and introduces itself as it was made,
whatever the app says, unless the session starts with `welcome: false`, for an
app that has already met the user (`references/server.md`). Per-app behaviour
comes from `state` and the agenda; if the persona itself is wrong for the app,
change it with `update_avatar` (from the next session) or use another avatar.

### 2. Shape the app

When the developer is still choosing what to build or which path fits (an
app around a conversation Atmee runs, or a face for their own agent), or
asks what Atmee offers or costs, use the `atmee-product` skill first. When
the app itself is still an idea (no settled user, flow or tools), plan it
with the `plan-avatar-app` skill and build from its brief.

Settle these with the developer, then name the closest shape in
`references/patterns.md` in one line and build to it:

| Question | Becomes |
|---|---|
| Does the user speak, type, or neither? | microphone, `lk.chat`, or `mute` |
| Does the user show the avatar files (a CV, a photo, a clip)? | `sendFiles`, a file drop |
| What must the avatar know that is true right now? | `state` |
| What happens that it should remark on? | `event` |
| What should the avatar make the app do, or look up? | app tools |
| Does the conversation follow steps? | a conductor graph |
| Does anyone else watch? | a design limit (below) |

### 3. Get a key

One key per app, narrow: `sessions`, bound to the avatar, with a spend cap,
its secret written straight into the backend's `.env` by the `atmee` CLI
(`references/server.md`, Keys, also for installing the CLI).

### 4. Build the backend

Port the two routes of `templates/server.mjs` into the developer's stack:
start a session, end a session. Keep its error handling, its loopback-only
default and its `ATMEE_BASE_URL` (Environment, above; `references/server.md`).

### 5. Build the page

Use `templates/avatar-room.js` and `templates/floor.mjs` as they are, or port
them to the page's framework keeping their timing: wait until the avatar can
talk (its audio or a first caption; its video shows whenever it comes), wait
for the agent to link the user, hold typed turns while the avatar has the floor
(`references/room.md`). Write `state` and `event` texts the way
`references/room.md` shows.

Show the conversation in one place, a chat panel or captions, never both,
and put the user's own words in it: their speech arrives transcribed on
`lk.transcription` under their identity, and a typed turn comes back the
same way, so drop that echo of a line they just typed (`references/room.md`,
Captions).

### 6. Add tools (only when the avatar acts on the app)

Declare a few clearly named tools and one `instructions` text for all of them
in the backend's session start, and handle `atmee.tool` in the page: the
starter's `APP_TOOLS_FILE` and `joinAvatar({ tools })`; `try.html` answers the
sample `templates/app-tools.example.json` (`references/server.md`,
App tools; `references/room.md`, Tools). Write the tools for the avatar's role:
the persona outweighs tool instructions (`references/patterns.md`, Tools).

### 7. Add the agenda (only when the app has steps)

Write the graph as a file in the developer's repo and set it on the avatar
with the CLI or `set_conductor` (`references/conductor.md`).

### 8. Prove it live

Run the app (or `try.html` against the developer's backend) and
check each item in a real session:

- [ ] The greeting's captions arrive and the avatar's video plays (or, for a
      voice-only session, the page shows its voice-only placeholder).
- [ ] A `state` changes an answer: ask something only the state can answer.
- [ ] An `event` is spoken, in the avatar's own words.
- [ ] A typed turn (or the microphone) is answered, including one sent while
      the avatar is speaking.
- [ ] With tools: ask for each one in words; the page's handler runs (log
      its calls), and for an `await` tool the avatar answers from the result.
- [ ] With files: drop one of each kind the app takes; each comes back
      `ready`, and the avatar talks about the drop once.
- [ ] With an agenda: the steps advance as the graph says.
- [ ] After ending: `get_avatar_session` shows `completed`, the `endReason`
      you expect and `chargedMicrocredits`.

Run it the way a user would. An agent can drive the page in headless
Chromium with a fake microphone (`--use-fake-ui-for-media-stream
--use-fake-device-for-media-stream`, or a generated audio track that is never
digitally silent), never a camera, and type the turns the test needs. End
every session you start, the failed runs too.

Report each with what you saw. An item that failed is a finding to fix, not a
footnote. Prove a fix with a test that fails before it and passes after,
then run the live check again.

### 9. Document and ship

- **A README per app:** the use case, an architecture diagram (page,
  backend, the Avatar API, the room), how it uses the API (the calls, the
  room messages, each tool and what the page does with it), its limitations
  as seen or expected, and the live run with its date.
- **Deploy behind a gate:** a sign-in or a shared password in front of every
  page and the start route; the key stays in the server's environment.
- **Ship through review:** a pull request, and the repo's review bot or
  reviewer until it is clean.
- **Name platform gaps:** a limit the app ran into goes into its README (and
  to the Atmee team) as a finding, with the workaround the app uses, so it is
  never a silent hack.

## Current limits

What the API cannot do yet; tell the developer when their app runs into one.

- **No audience.** One user token per session, no viewer tokens and no
  recording out of the room: a broadcast must restream the avatar's tracks.
- **Session length** is capped by the plan (`get_account`:
  `limits.maxSessionSeconds`); show a countdown from `actualDurationSeconds`
  and warn near the end. Longer use chains sessions, with a visible cut.
- **The avatar speaks up on its own** after about 10 seconds of silence (a
  check-in), even when muted, unless the session sets `checkInAfterS`
  (`references/server.md`): another length, or `0` for none, on a v2 avatar.
- **The persona is fixed** per avatar; there is no per-session persona or
  greeting, only `welcome: false` to turn the greeting off.
- **No server-side feed.** Captions, tool calls and the session's progress
  exist only in the room: the page relays what the backend needs (a
  transcript, a tool's outcome).
- **Mid-turn messages.** A typed turn that lands while the avatar speaks cuts
  in or is lost: hold it until the turn ends (`references/room.md`, Typed
  turns).
- **The agenda is blind to tools and silent to the app.** A conductor judges
  only the conversation, and the page is not told the current step: a page
  that follows the steps gets them from a tool the avatar calls.
- **Muting deafens the app too:** while muted, no user transcript reaches the
  room.
- **A reconnect ends the session,** as `user_left`, with no grace period. A
  typing-only page's silent track sends almost nothing and can be rated
  lost: the Atmee demo apps (`references/patterns.md`) publish theirs with
  `dtx: false`.
- **No eyes on a live feed.** The avatar sees the files the user drops into
  the call (`references/room.md`, Files); to react to a screen, a camera or a
  game, run your own model and send what it sees as `state` and `event`.
- **Cold starts.** A cold rendering pool answers `503 no_capacity` with
  `Retry-After`, for minutes; the page must say it is connecting.

## References

| File | Read when |
|---|---|
| `references/server.md` | creating the key, writing the backend, declaring tools, handling a session error |
| `references/room.md` | writing the page, sending `state`/`event`/typed turns, answering tool calls, file drops, captions, a message that seems ignored |
| `references/conductor.md` | the conversation has steps (interview, lesson, quiz, onboarding) |
| `references/patterns.md` | shaping the app in step 2, writing tools |
