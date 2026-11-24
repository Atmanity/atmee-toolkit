# What each surface offers

## Contents

- atmee.ai: talking to avatars
- Atmee Studio: making avatars without code
- Avatar API: Atmee in a developer's product
- Agent tooling: the atmee plugin

What a person can do on each surface today. **Not open to everyone** marks
a feature that is built but switched on only for some accounts: name it as
coming, never promise it.

## atmee.ai: talking to avatars

- **Explore:** public avatars, featured and new, by channel, with search.
  Signature avatars (a star) cost more per minute and take paid credits;
  verified creators (identity checked) carry a badge.
- **A call** is a live video conversation in the browser; no app to install.
  The visitor signs in and talks on their own credits, unless the avatar's
  owner sponsors visitors (`references/pricing.md`, Sponsoring). A sponsored avatar that
  allows visitors without an account, and is public or unlisted, can be
  talked to from its share link without signing in.
- **Coming back:** recent conversations and favourites open the call again.
- **Creator profiles** at `https://www.atmee.ai/@<handle>`.
- **Not open to everyone:** live streams of avatars on the home page, the
  avatar profile page before a call, mini-apps inside a call.

## Atmee Studio: making avatars without code

The editor has five steps: Appearance, Voice, Persona, Knowledge, Finalize.
Studio builds all three models: **Atmee V1** on any plan, **Atmee V2** on Pro
and up, **Atmee V2 HD** on Growth and up (the same rules as the Avatar API and
the agent tooling). Avatars an account already has keep chatting, sharing and embedding on any
plan; editing V2 ones needs Pro and up, and V2 HD ones Growth and up.

- **Appearance:** upload or take a photo, or generate a portrait from a
  prompt (generation costs credits).
- **Voice:** record or upload a sample to clone, or pick a premade voice.
  Voice design from a description is not open to everyone in Studio; the
  avatars server has it.
- **Persona:** a personality preset or a custom one, a role (assistant,
  sales guide, mentor and coach, brand ambassador, expert advisor and more),
  greeting, guardrails, conversational habits, names; it can be drafted from
  the creator's social accounts.
- **Knowledge:** files, free text, a questionnaire about the person, and
  feeds that keep the avatar current from social accounts and sites (the
  list of sources is in Studio).
- **Finalize:** visibility and, for eligible creators, earnings. After the
  first publish, edits go to a draft until published again.
- **Test call** before publishing.
- **Manage:** every avatar with its build status, share link, embed steps and
  sponsor status.
- **Analytics:** conversations and their length, peak hours, top avatars and
  regions, who talks (demographics, segments), and the topics and questions
  people bring.
- **API & Embeds:** API keys with scopes and caps, usage per key and avatar,
  and the embed quickstart. There is no embeddable widget: an avatar on
  another site goes through a backend with a key (the Session API).
- **Billing:** the plan and upgrades, credit packs, usage and credit history,
  automatic recharge, sponsored access, earnings and payouts, identity
  verification.
- **Not open to everyone:** live streaming an avatar to an audience, apps
  inside a call.

## Avatar API: Atmee in a developer's product

Docs: `https://www.atmee.ai/docs/api`. A backend sends `X-Api-Key` (the API
refuses browser calls); the CLI and the MCP servers act as a signed-in user
instead. The API is at `https://api.atmanity.us`.

- **Conversations** (`POST /v1/session`): Atmee runs speech recognition, the
  persona and model, the voice and the video. The answer comes once the
  avatar is in the room, with the LiveKit `serverUrl` and a one-room
  `userToken` for the page; the user joins within 60 seconds. Options: the
  longest the session may run, a language for this session, a visitor id for
  memory across visits (a key with visitor memory), metadata, app tools. End
  with `POST /v1/session/{id}/end`; `GET /v1/avatar_sessions/{id}` gives the
  end state, the reason and the cost.
- **In the room** (Causeur sessions): `state`, `note`, `event`, `mute` and
  typed turns from the page; captions from the avatar; app tools the avatar
  calls on the page (`atmee.tool`), either telling the app or waiting for its
  answer.
- **Render path** (`POST /v1/avatars/{id}/avatar_sessions`): the avatar joins
  the developer's own LiveKit room and gives their agent a face.
  `livekit-plugins-atmee` wraps it for LiveKit Agents, for v1 and v2 avatars
  (0.2.0 and later; not V2 HD, which takes the route directly).
- **Speech sessions** (`POST /v1/speech_sessions`, `start_speech_session`,
  `atmee speech start`): speech-to-speech over a WebSocket, with no video.
  The avatar needs a voice; a client connects with the token within 60
  seconds.
- **Avatars:** create from a portrait, a voice sample, a persona and
  knowledge files (local files through uploads, or https links); list, read,
  update the persona and settings, replace the portrait, voice or knowledge,
  delete. Duplicate one (`duplicate_avatar`, `atmee avatars duplicate`): a
  private copy with its persona, knowledge, portrait and voice, on any model
  (an Atmee V1 avatar onto Atmee V2, or a V2 one back onto V1), without its
  sharing, earnings, sponsor pool, agenda or seeds. v2 seeds: list, capture more, select, remove an older version. An
  agenda (conductor) per v2 avatar. A v2 avatar's video is square (512×512)
  by default, or another size the server offers (`get_account`
  `limits.v2Resolutions`): 672×384 (16:9), 384×672 (9:16), 576×448 (4:3)
  or 448×576 (3:4), set with `resolution`; a new size makes new seeds. An Atmee V2 HD avatar
  (engine `v2hd`, for Growth and up: `get_account` `limits.v2Pro`) is a v2 avatar on a larger model: the same seeds, agenda and sessions, at
  640×640 by default or another of `limits.v2ProResolutions` (for example
  864×480).
- **Account:** the balance, the plan with its limits, per-minute prices and
  generation prices, and who the caller is.
- **Generation:** portraits from a description, an enhanced or restyled
  version of a photo, voice previews from a description (each paid per call); the outputs
  go straight into an avatar's create or replace (a key needs
  `generation:write` with `avatars:write`). Where offered, a **clip**: an
  mp4 that starts on any picture (a person, character, animal, product or
  scene) and moves as a prompt says, silent, or with an audio file (speech
  drives the speaker's lips; other sound plays back as given), with a prompt
  of up to 1,024 model tokens (a token limit, not a character one: about
  5,000 characters of English), 1 to 15 seconds at one of the clip sizes: 480×480 (the
  default), 640×480, 480×640, 832×468, 468×832, 720×720, 960×720, 720×960,
  1280×720 or 720×1280. 1080p (1080×1080, 1920×1080, 1080×1920, 1440×1080,
  1080×1440) is coming soon. No avatar is made; it
  is on every plan, Free included, with no Atmee V2 access
  needed (`get_account` `limits.videoGeneration` false means video generation
  was turned off for the account: contact support), and each size has a price per
  clip for up to 5, 10 and 15 seconds, charged at the next length up
  (`pricing.generation.clipPrices`). A clip is not avatar editing: a key
  needs only `generation:write`, which also covers the clip's uploads and
  reading it back. An **avatar clip** (`avatarId` with `transcript`) stars a
  saved V1 or V2 avatar instead: its saved picture is the first frame and its
  Fish Audio voice speaks the transcript, lip-synced; the length follows the
  speech (at most 15 seconds), it is charged at its actual length's tier but
  the balance must cover the size's 15-second price first, and a key also
  needs `avatars:read`. A clip is kept 30 days, a generated portrait or voice
  preview 7; its link works until `urlExpiresAt`, and reading a clip
  again (`get_clip`) gives a new one. The account can delete one sooner (`DELETE
  /v1/uploads/{uploadId}`, MCP `delete_output`; a whole generation with `DELETE
  /v1/generation/{requestId}`, MCP `delete_generation`).
- **Generation history:** every clip, portrait and voice call of the account,
  newest first, with who made it, its inputs, its charge and fresh download
  links (`GET /v1/generation`, MCP `list_generations`, CLI `atmee generate
  list`), and counts by kind and maker (`GET /v1/generation/summary`, MCP
  `get_generation_summary`, CLI `atmee generate summary`). Any member sees all
  of the account's generations; a clip shows a small poster after it expires.
- **API keys:** list, create, rotate, revoke (signed in as any member of
  the account). Scopes `sessions`, `avatars:read`, `avatars:write`,
  `billing:write`, `generation:write`; optionally one avatar only, a spend cap, a longest
  session, sessions at once, visitor memory.
- **Sponsoring:** the avatar's pool and its settings, moving credits in and
  out (signed in, or a key with `billing:write`; on a shared account, signed
  in as its owner or an admin).
- **Languages:** an avatar takes one of the codes `update_avatar` accepts; a
  session may name another. The API reference lists which are spoken end to
  end.
- **Studio only, not in the API:** knowledge feeds from social accounts,
  premade voices, analytics, billing and earnings. An avatar can't be
  created public; it goes public by an update once it is ready.

## Agent tooling: the atmee plugin

- **Skills:** `create-avatar` (a v2 avatar from a name, a photo and a voice:
  see it talk, then give it a character and test it), `make-video` (one
  short video clip from a picture, silent or with a recording, or of a saved
  avatar speaking a script),
  `plan-avatar-app` (an
  app idea shaped into a brief), `build-avatar-app` (an app or website
  around an avatar), `atmee-setup` (connects and checks everything) and
  `atmee-product` (this one).
- **The avatars server** (`atmee-avatars`, the Avatar API's own MCP server):
  avatars, seeds, uploads, test conversations, keys, agendas, sponsoring, the
  account, balance and prices, and generation: portraits from a description,
  portrait enhancement or restyling and voice design (each per call),
  and clips (`generate_clip` or `atmee generate clip`, a price per clip by its size and length, with Atmee V2, where offered).
- **The `atmee` CLI:** every Avatar API task from a terminal or CI, as a
  signed-in user or with a key (`docs/cli.md` in the GitHub repo
  `Atmanity/atmee-toolkit`).
- Installing all of it: the `README.md` of the same repo.
