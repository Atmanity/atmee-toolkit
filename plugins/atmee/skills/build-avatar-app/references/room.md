# The room

## Contents

- Who is in the room
- Joining
- Topics
- `atmee.app` messages
- Tools: `atmee.tool`
- Files: `atmee.file`
- Typed turns: hold them
- Captions
- Speaking indicator
- Ending

The page joins the session's LiveKit room as the user, with `livekit-client`
2.x. `templates/avatar-room.js` does everything below; read this to port it,
to change it, or when something seems ignored.

## Who is in the room

- **You**, the user: the participant the agent listens to.
- **The agent** (`lk.agent.name` attribute): the conversation itself; it
  publishes no tracks. Its `lk.agent.state` attribute is not a usable signal:
  it stayed `speaking` through a whole test session.
- **The renderer**: the avatar's audio and video, published on the agent's
  behalf. Attach its tracks to a `<video>` and an `<audio>` element.

Both are agent participants (`participant.isAgent`). Only the renderer has
`lk.publish_on_behalf`, set to the agent's identity: read the agent's
identity from it, or, with no renderer, take the agent participant without
it (the starter's `agentIn`).

## Joining

1. `room.connect(serverUrl, userToken)`, then `room.startAudio()`. The click
   that started the session may be too long ago by then for the browser to
   allow sound (Safari): LiveKit's `AudioPlaybackStatusChanged` fires and
   `room.canPlaybackAudio` is false, so show a "tap to hear" button that calls
   `room.startAudio()` (the starter's `onAudioBlocked`).
2. Publish the microphone, or for a typing-only app a silent audio track (how
   the starter and the demo apps ran typing-only sessions).
3. **The avatar has arrived when it can talk:** its audio track is subscribed,
   or its first caption stream arrives, usually a few seconds after
   connecting. Give it about 90 seconds before calling it failed
   (`arrival_timeout`). Its video is not a gate: attach and play it whenever it
   comes, and show a voice-only placeholder until it plays (the starter's
   `onVideo(on)`). A session can have no video at all, from a voice-only
   runtime, and still talks, calls tools and answers `state`. An app that must
   not start without the face passes `waitForVideo: true`, which goes live
   only once the video plays. The video has the avatar's own shape, square
   (512×512) by default or portrait or landscape: size its box from the
   video (`aspect-ratio: auto 1 / 1` with `object-fit: contain`, as the
   starter's `try.html` does), never a fixed square that crops it.
4. **Wait about 2.5 seconds more before the first `atmee.app` message or
   file drop.** The agent hears `atmee.app` only from the participant it has
   linked, and messages sent before that are dropped without a word. Queue
   what the app wants to say until then.
5. For about its first 5 seconds the avatar hears the user as silence
   (Causeur's join mute, so a user still talking to someone else isn't
   answered): words spoken that early are not heard.

## Topics

| Topic | Direction | Carries |
|---|---|---|
| `lk.chat` | page → avatar | a typed user turn; the avatar answers it |
| `atmee.app` | page → avatar | one JSON object per message: the app's state, notes, events, mute |
| `lk.transcription` | avatar → page | the avatar's words, as text streams |
| `atmee.file` | page → avatar | a drop of files and its caption, to the agent alone (Files, below) |
| `atmee.file.status` | avatar → page | one JSON object per file and status (Files, below) |

App tool calls come as an RPC, not a topic (Tools, below).

Send with `room.localParticipant.sendText(text, { topic })`. Nothing is
answered on `atmee.app`: a malformed message is logged by the agent and
ignored, so the only proof a message worked is what the avatar does next.

## `atmee.app` messages

| Message | What it does | Limits |
|---|---|---|
| `{"type": "state", "text": "..."}` | what is true now; replaces the last state, `""` clears it | 3,858 characters |
| `{"type": "note", "text": "..."}` | context to keep; added to earlier notes, never replaced | 2,000 characters |
| `{"type": "event", "text": "...", "ttl_s": 10}` | something to speak about in the avatar's own words, once nobody has the floor; dropped if it waits longer than `ttl_s` | 2,000 characters; `ttl_s` above 0, at most 60, default 10; 16 waiting at most (a 17th drops the oldest) |
| `{"type": "mute"}`, `{"type": "unmute"}` | the avatar hears the user as silence, or hears them again; the user stays in the call | silence still counts towards a check-in |

One message is at most 16,000 characters as sent.

### Writing them

- **`state`** is what the user sees now, in plain sentences, with the facts
  the avatar needs to answer: "The user is on the pricing page. Plans: Leaf
  costs 9 dollars a month, Grove costs 29." Asked "How much is the Grove
  plan?", the avatar answered from that state alone. Re-send it whenever the
  screen changes; the starter skips a state identical to the last.
- **`event`** is a fact that just happened, plus how to react when it
  matters: "The player found the key. React in one vivid sentence." Short
  `ttl_s` for things that go stale (a game move: about 12), longer for a
  digest (30).
- **`note`** is for facts that hold for the whole session, the user's name,
  their plan, that should not be lost when `state` changes.
- **`lk.chat`** is for the user's own words. Relaying someone else's words as
  a turn works when it is framed ("Mia asks: ..."), as a live stream does with
  its viewers' questions.

## Tools: `atmee.tool`

When the session declared app tools (`references/server.md`, App tools), each
call the avatar makes arrives as a LiveKit RPC, method `atmee.tool`, sent to
the user's participant, the one the agent listens to. Register the handler on
the room before connecting, so no call finds the page without one:

```js
room.registerRpcMethod('atmee.tool', async ({ payload }) => {
  const { id, name, arguments: args } = JSON.parse(payload);
  if (name === 'show_slide') { showSlide(args.n); return 'ok'; }
  if (name === 'get_score') return JSON.stringify(await fetch('/api/score').then((r) => r.json()));
  throw new RpcError(1500, `no tool ${name}`);
});
```

(`room.localParticipant.registerRpcMethod` is the older, deprecated name for
the same.) The starter does this in `joinAvatar({ tools })`: one function per
tool name, and a returned object is sent as JSON.

- **The payload** is `{"id", "name", "arguments"}`; `arguments` already fit
  the tool's `parameters`. `id` is the call's own.
- **`notify`:** the avatar was told `ok` before the call reached the page, so
  return quickly; the answer is ignored.
- **`await`:** the string the handler returns is what the avatar is given, to
  answer from. Return it within the tool's `timeoutSeconds`; the wait is cut at
  about 27 seconds whatever the tool says. Keep it short and plain: the avatar
  speaks from it.
- **Size:** the call (its `arguments` with the id and name) and the reply are
  each at most 15 KiB.
- **Server data:** the handler calls the app's own backend, as above; the key
  and anything the browser must not see stay there.

What the avatar is given instead of an answer, each a text starting `error: `,
which it reads like any other result:

| When | It is given |
|---|---|
| no answer in time | `error: timeout` |
| the handler throws `new RpcError(code, message)` | `error: <message>` |
| the handler throws anything else | `error: Application error in method handler` |
| no handler is registered (or LiveKit Meet joined) | `error: the app does not handle atmee.tool` |
| the reply is over 15 KiB | `error: reply too large (limit 15 KiB)` |
| the call is over 15 KiB (never sent) | `error: arguments too large` |
| the arguments don't fit `parameters` (never sent) | `error: invalid arguments: ...` |
| no user is in the room | `error: no user in the room` |

Once a `notify` call is sent, its failure is only logged by the agent: the
avatar was already given `ok`.
While an `await` call runs the avatar says at most one short filler line.
LiveKit Meet (the CLI's `session chat --open`) registers no handler, so test
tools from the app's own page or `try.html`: start `server.mjs` with
`APP_TOOLS_FILE=app-tools.example.json` and ask the avatar to show slide 3 or
for your score. The page shows the slide, lists each call with its arguments,
and answers `get_score` with `{"score": 4173, "outOf": 5000}`.

## Files: `atmee.file`

The user can drop files into the call for the avatar to read and talk about:
a CV to go through, a photo, a recording. The starter's
`avatar.sendFiles(files, { caption, onStatus })` sends one drop (1 to 10
files and an optional caption of at most 500 characters; a longer one is
cut) and resolves, once every file is ready or refused, with one
`{ name, status, reason }` per file, in order. It is there once `joinAvatar`
resolves, after the agent has linked the user: a drop sent before the link
is lost, like an `atmee.app` message.

| Kind | Types | Each file at most |
|---|---|---|
| Image | PNG, JPEG, WebP | 20 MB |
| Image | HEIC, HEIF | 5 MB |
| PDF | | 20 MB and 100 pages |
| Text | any UTF-8 text: txt, md, csv, json, html, xml, srt, code | 500 KB |
| Audio | AAC, FLAC, MP3, M4A, Ogg, WAV, WebM | 20 MB |
| Video | MP4, MOV, MPEG, WebM, WMV, 3GP, FLV | 25 MB |

A drop is at most 10 files and 50 MB; a call, 20 files and 100 MB. The agent
checks types and sizes; the starter checks only the count (more than 10
rejects with `too_many_files`, before anything is sent). Give the file input
`multiple` and
`accept="image/*,application/pdf,text/*,audio/*,video/*,.json,.srt,.md,.csv"`
(some browsers file JSON, subtitles, Markdown or CSV under other types).

- **Only for the call.** Files are never stored: the avatar reads them during
  the call, and they are gone when it ends.
- **One reaction per drop.** A drop is read at once, even while the avatar
  speaks, and talked about at the next free floor, the way an event waits.
  The avatar reacts once to the whole drop, so send files that belong
  together as one drop.
- **An agenda step wins over a drop.** While a conductor step asks for
  something else, the avatar may only say it got the file and go back to the
  step, even when asked about it. If a step expects a file (a CV in a
  screening), say so in that step's instructions.
- **The support check.** An agent that takes files sets its attribute
  `atmee.files` to `"1"`; avatars on the older runtime never set it. Offer a
  drop only when `avatar.acceptsFiles()` is true. Otherwise `sendFiles`
  rejects at once with `files_unsupported` (`no_agent` when the agent is not
  in the room, as during a reconnect).

A drop is one `sendText` with the files attached, to the agent alone:

```js
const info = await room.localParticipant.sendText(JSON.stringify({ caption }), {
  topic: 'atmee.file', attachments: files, destinationIdentities: [agentIdentity],
});
```

LiveKit sends the caption as a text stream that lists the files, then each
file as a byte stream on the same topic, with its name and type. Send to the
agent only (Who is in the room, above): sent to everyone, the files would
stream to the renderer too. Never send files on `lk.chat`: the agent drops
attachments there.

### Answers: `atmee.file.status`

The agent answers each file on the topic `atmee.file.status`, one JSON object
per status:

```json
{"batchId": "...", "index": 1, "name": "clip.mp4", "status": "refused", "reason": "too_large"}
```

`batchId` is the `id` of the info `sendText` resolves with, and `index` the
file's place in the drop. Take answers only from the agent. `sendText`
resolves once every file is out, so the first answers can come before the
page knows the `batchId`: keep them until it does, as the starter does.

`status` is `received` or `reading` on the way, then `ready` (the avatar can
talk about it) or `refused`, with a `reason`: `too_large` (over a cap above),
`unsupported_type`, `unsafe`, `too_many_files` (over the drop's or the call's
count), `unreadable` or `failed`. The starter's `onStatus` gets each answer,
and the refusals the starter makes itself:

- **No answer for 120 seconds** once the drop is out: every file not yet
  ready or refused is `refused`, `failed`. Each answer starts the wait again,
  since the agent reads files in turn (a few at a time per call and per
  server) and a large drop can take minutes.
- **A full reconnect** (`RoomEvent.Reconnecting`) cuts off the files still
  streaming: a drop still uploading (`sendText` not yet resolved) is
  `refused`, `unreadable`. A drop already out waits on; the agent keeps
  reading it.
- **The room disconnects or the session ends:** every file not yet ready or
  refused is `refused`, `unreadable`.

`try.html` has a file picker: drop a few files, with a caption, and its log
shows each answer and the outcome.

## Typed turns: hold them

A typed turn that reaches the avatar while it speaks cuts it off, and in the
sample apps was sometimes lost. One that lands just as it starts an event's
reply was lost in testing: Causeur sends a waiting event 0.3 seconds after the
avatar's words end, ahead of any turn waiting on the page. So the page holds
typed turns until:

- the avatar has been quiet for 1.5 seconds (gaps inside one reply are under a
  second),
- every event the app sent has been started on: each new avatar utterance
  (a new caption stream) counts for the oldest waiting event, and an event
  counts as dropped once its `ttl_s` has passed, and
- the previous typed turn has been answered, or 10 seconds have passed
  without the avatar starting to.

Turns typed meanwhile go out together, as one turn. Show a held turn as sent
in the page. The microphone needs none of this: the agent handles barge-in on
speech.

The starter keeps all of this in `templates/floor.mjs`, apart from the rest:
it works around today's platform, and goes once Causeur makes typed turns
wait for the floor the way it makes events wait.

## Captions

The avatar's words arrive on `lk.transcription` as text streams, one per
utterance. Read the chunks as they come (a live caption); a stream that has
finished is a finished utterance (its final attribute never reads true, so
don't wait for it). Streams sent as your own identity are the user's speech,
transcribed. A reply that is cut off can end its stream early while the audio
goes on, so captions are not a complete transcript.

## Speaking indicator

`RoomEvent.ActiveSpeakersChanged` with any participant other than you means the
avatar is speaking. Expect gaps under a second inside one reply; treat 1.5
seconds of quiet as the end of its turn.

## Ending

`avatar.end()` in the starter: the backend's end route, then
`room.disconnect()`. On `pagehide`, `navigator.sendBeacon` to the end route
as well: a user who just leaves ends the session too (`user_left`), but only
once LiveKit notices, and until then the avatar is busy (`409 avatar_busy`).
`RoomEvent.Disconnected` tells the page the session is over, whoever ended
it.
