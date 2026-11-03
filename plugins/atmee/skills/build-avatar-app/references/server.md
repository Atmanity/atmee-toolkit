# The backend

## Contents

- Keys
- Start a session: `POST /v1/session`
- App tools
- End and read a session
- Protect the start route
- A face for your own agent
- Speech sessions

The backend holds the API key, starts the avatar's session, gives the page
what it needs to join, and ends the session. `templates/server.mjs` does all
of it in one file with no packages; its two `/api` routes are what to port.

## Keys

Only a signed-in owner or admin of the account creates keys, and the secret
exists once, in the answer to create or rotate. Get it straight into the
backend's environment file so it never passes through the chat, with the
`atmee` CLI. When `atmee --version` fails, install it with Go (its source is
in `Atmanity/atmee-toolkit`) and sign in against the server the plugin uses:

```bash
go install github.com/Atmanity/atmee-toolkit/cli/cmd/atmee@latest
export ATMEE_BASE_URL=https://api.atmanity.us
atmee login        # opens the browser; the same account as the avatar
```

A sign-in or key works only against the server it was made for, so keep
`ATMEE_BASE_URL` set in every shell that runs `atmee` (the skill's Environment section); `atmee whoami` names the
server. Then:

```bash
atmee keys create --name "<app name>" --avatar <avatarId> --spend-cap <credits> --out .env
```

`--out` writes `ATMEE_API_KEY=...` into the file and prints no secret. Add the
rest of the backend's environment to the same file:

```bash
ATMEE_BASE_URL=https://api.atmanity.us
AVATAR_ID=<avatarId>
``` With
only the MCP tools, `create_api_key` makes the key but withholds the secret:
give the developer `atmee keys rotate <keyId> --out .env`, or the API keys page
in Atmee Studio. Check that `.env` is in `.gitignore`; in production the key
goes into the platform's secret store.

Keep the key narrow:

- **Scopes:** `sessions` (start and end conversations) is all an app needs.
  Add `avatars:read` only if the backend reads avatar status; `avatars:write`
  only if it sets the agenda itself.
- **`--avatar`** binds the key to the app's avatar; others answer `403`.
- **Caps:** `--spend-cap` (and `--spend-cap-window`), `--max-session`,
  `--max-concurrent`. They are the backstop when the app's own limits fail.
- **`--visitor-memory`** only if the app sends `visitorId` (below).

## Start a session: `POST /v1/session`

`X-Api-Key: <key>`, JSON body:

| Field | Notes |
|---|---|
| `avatarId` | required; on the key's allow-list |
| `maxDurationSeconds` | 1 to 14400; cut to the key's, the plan's and the balance's limits, never refused for being long |
| `language` | BCP-47 (`en`, `zh-TW`, `ja` are routed; other tags fall back to detection); omit for the avatar's own |
| `visitorId` | 16 to 256 characters, only with a visitor-memory key: it unlocks that visitor's earlier conversations, so derive it on the server (for example an HMAC of the app's user id) and never take it from the page |
| `metadata` | up to 50 properties, stored with the session (the app's name, its own user or order id) |
| `appTools` | `{ "instructions": "...", "tools": [...] }`: functions the avatar may call in this session (App tools, below) |
| `welcome` | `false` starts without the avatar's self-introduction or welcome back, for an app that has already met the user: it waits for them to speak, though a long silence still gets its check-in (`checkInAfterS`, next). Honoured by a v2 avatar on the default runtime; any other (v1, or one with its own launch URL) ignores it and speaks first, with no error. A speech session pins it instead: `"settings": {"welcome": false}` |
| `checkInAfterS` | seconds of silence before the avatar speaks up, 0 to 3600; `0` for no check-in, for an app that drives the talk itself; a second check-in waits twice as long, then it stays quiet until the user speaks. Absent, the default, about 10 seconds. Honoured by a v2 avatar on the default runtime; any other ignores it and keeps its usual check-in, with no error, billed the same. A speech session pins it instead: `"settings": {"checkin_after_s": 0}` |

`201` answers once the avatar is in the room: about 15 to 20 seconds in
our tests. The page should show that it is connecting meanwhile.

| Response field | Use |
|---|---|
| `sessionId` | to end it and read it later |
| `serverUrl`, `userToken` | for the page: `room.connect(serverUrl, userToken)` |
| `roomName` | informational; the token already picks the room |
| `actualDurationSeconds` | the granted length, counted from when both the user and the avatar are in the room: show a countdown and warn near the end (it then ends `max_duration`); absent for an uncapped (metered) session, so show no countdown then |
| `durationCapped` | `true` when the limits cut what you asked for |
| `billingMode` | `reserve` (the whole length held at start, the rest returned) or `metered` (per minute) |

Errors (`{"error": "<code>", "message": "..."}`):

| Status | Codes | What the app does |
|---|---|---|
| `503` | `no_capacity` (a cold or full rendering pool) | wait `Retry-After` seconds and start again; the starter's page keeps trying for up to six minutes |
| `503` | no body | embeds are turned off for the deployment: don't retry; tell the operator |
| `409` | `avatar_busy` | the avatar is in another live session (`liveSessionId` when it is an API one): end that one, or tell the user it is busy |
| `409` | `avatar_not_ready` | no seed is serving yet: wait for `readyToChat` |
| `402` | `insufficient_credits`, `api_key_spend_cap_reached`, `concurrent_session_limit`, ... | don't retry; tell the operator, not the end user, which limit it was (the starter logs the message and sends the page only the code) |
| `403` | | the avatar is not on the key's allow-list |
| `401` | | a wrong or revoked key |
| `400` | `invalid_tools` | a tool breaks a rule (App tools, below); the message names it: fix the declaration, don't retry |
| `422` | | a field is out of range, or `appTools` has a field other than `instructions` and `tools`; the body names it |

A seed is one captured talking clip of a v2 avatar; the server's "candidate" and "take" mean a seed, and the API's `seed` field is an informational number, not the clip.

## App tools

`appTools` declares what the avatar may make the app do during this session.
Tools are per session: each start carries the set it needs, and a session
without `appTools` has none.

```json
"appTools": {
  "instructions": "Put each slide up before you teach it. When the trainee asks for their score, get it.",
  "tools": [
    { "type": "function", "name": "show_slide",
      "description": "Put lesson slide n on the trainee's screen.",
      "parameters": { "type": "object", "properties": { "n": { "type": "integer" } }, "required": ["n"] } },
    { "type": "function", "name": "get_score",
      "description": "Get the trainee's quiz score so far.",
      "parameters": { "type": "object", "properties": {} },
      "x-atmee": { "mode": "await", "timeoutSeconds": 8 } }
  ]
}
```

- **A tool** is OpenAI's function shape, `type` `function`, `name`,
  `description`, `parameters`, plus an optional `x-atmee`: `mode` `notify`
  (the default: the avatar goes on at once) or `await` (it waits for the
  page's answer), and `timeoutSeconds`, 1 to 28 (default 8), for `await`.
  OpenAI's `strict` is accepted and ignored: arguments are checked against
  `parameters` either way, and a call that doesn't fit never reaches the page.
- **`parameters`** is a JSON Schema 2020-12 object (top-level `"type":
  "object"`). A `$schema` naming another dialect is refused, and `$ref`,
  `$dynamicRef` and `$recursiveRef` may point only inside the schema (`#...`).
- **`instructions`** is one text for all the tools, at most 4000 characters:
  when to call which.
- **Limits:** 1 to 30 tools; names unique, `^[a-z][a-z0-9_]{0,47}$`, and not
  `web_search` or `lookup` (Causeur's own); `description` 1 to 500
  characters; `parameters` at most 4096 bytes; all tools together at most
  24576 bytes. A tool set that would push the session over the runtime's
  payload limit together with the avatar's own config (its agenda included)
  is refused as well.
- **Every call goes to the page** in the room, as the LiveKit RPC
  `atmee.tool` (`references/room.md`, Tools). When the answer needs server
  data, the page's handler calls its own backend; nothing calls the backend
  directly.
- **Only Causeur calls tools.** A session the behavior module runs (every v1
  avatar, and a v2 avatar not yet moved to Causeur) accepts them and never
  calls them.

A broken rule answers `400 invalid_tools`, its message naming the tool and the
rule (`tools/0/parameters/$schema: only JSON Schema 2020-12 ... is accepted`).

## End and read a session

- `POST /v1/session/{id}/end`: the room closes and billing stops. Ending twice
  answers `200` with `alreadyEnded: true`, so `503 session_unavailable` and
  `500 finalize_failed` are retried. End only sessions the backend started for this user: the starter
  keeps a set of its own; a real app ties each session to its own user.
- `GET /v1/avatar_sessions/{id}` (also `get_avatar_session`): `status`
  (`pending`, `active`, `completed`, `failed`), `endReason` (`user_left`,
  `max_duration`, `ended_by_api`, `no_user_joined`, `balance_exhausted`, ...),
  `billedSeconds` and `chargedMicrocredits`. Billing runs only while both the
  user and the avatar are in the room.

## Protect the start route

Every call starts a billed session. Put it behind the app's own sign-in and a
rate limit; the key's spend cap and concurrency cap catch what slips through.
The starter answers only this machine (`127.0.0.1`) and only its own pages
(a Host and an Origin check), for that reason; anything else is the
developer's own backend.

## A face for your own agent

When the developer already runs their own voice agent (speech, model, voice)
in their own LiveKit room and wants only the avatar's face, the call is
`POST /v1/avatars/{avatarId}/avatar_sessions` with their room's URL and a
token they mint, not `/v1/session`. Nothing in `references/room.md` applies
then: their agent does the talking. `atmee avatars session render --help`
shows the fields. `livekit-plugins-atmee` wraps this for LiveKit Agents:
from 0.2.0 it renders v1 and v2 avatars (`avatar_version="v2"` for a v2
one). It does not take V2 Pro: for a V2 Pro avatar, call the route directly.
A render-only
session with an Atmee V2 Pro avatar (engine `v2pro`) is the same call and
the same fields, billed at the v2 render rate (`renderV2PerMinuteMicrocredits`);
its video comes at the avatar's own size (640×640 unless it was given
another).

## Speech sessions

A voice conversation with no video: `POST /v1/speech_sessions` (MCP
`start_speech_session`, `atmee speech start`) returns a WebSocket url and a
token for the developer's client, which connects within 60 seconds. The
avatar needs a voice, and the owner's account is billed by the minute.
