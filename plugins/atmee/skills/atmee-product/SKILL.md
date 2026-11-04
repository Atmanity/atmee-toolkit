---
name: atmee-product
description: What Atmee is - live video avatars and v2 video clips from a photo - features, plans, pricing, the path for an idea. Use when asked what Atmee can do or costs, or before a build. NOT for making one.
license: Proprietary
metadata:
  owner: Atmanity
---

# Atmee: the product map

You know what Atmee is, what each part of it does and how it is paid for, so
you can answer a question about the product and steer an idea to the right
**path** before anything is built. This file holds what changes slowly: what
exists and how it fits together. Numbers change often (prices, plan limits,
the balance), so they always come from a **live source** (Pricing, below),
never from this file or from memory.

## What Atmee is

Atmee (atmee.ai) makes **live video avatars**: a face, a voice, a
personality and knowledge that hold a real-time spoken conversation with a
person over video, lips in sync. In the website's words: build your own live
video avatars, with any voice, personality and knowledge, then launch them on
Atmee or integrate them anywhere. People make avatars of themselves (a
digital twin), a brand's concierge, an expert, a teacher, a companion, a
historical figure or an original character.

Atmee v2 also makes **video clips**: from a photo and a prompt, a short mp4
of that person or character moving as the prompt says, speaking a voice line
when one is given (silent without), with no avatar needed (a teaser, a social
post, an ad, a product demo). Whenever someone asks what Atmee can do, or
wants a video rather than a conversation, say that it makes clips too;
`create-avatar` makes them. Clips need Atmee V2 on the account (see
Two engines), and a server offers them only when
`get_account`'s `pricing.generation` has a clip price.

The company was called Atmanity, and the name stays in infrastructure: the
API host (`api.atmanity.us`), the GitHub organisation
and some ids. In anything a person reads, say Atmee.

## The surfaces

| Surface | For | What happens there | Where |
|---|---|---|---|
| **atmee.ai** | people who talk to avatars | explore avatars, visit creators' profiles, talk to an avatar from its share link | `https://www.atmee.ai` |
| **Atmee Studio** | creators, no code | make and edit v1 avatars, set visibility, share, analytics, API keys (Studio → API & Embeds), plans, top-ups, sponsoring and earnings (Studio → Billing) | `https://www.atmee.ai/studio`, signed in |
| **Avatar API** | developers | put an avatar in their own product: start conversations from their backend, create and manage avatars and keys | docs `https://www.atmee.ai/docs/api` |
| **Agent tooling** | anyone working through an agent | the skills `create-avatar`, `plan-avatar-app`, `build-avatar-app`, `atmee-setup` and this one, the avatars server (with generation), the `atmee` CLI | the atmee plugin |

What each surface offers in detail: `references/features.md`.

## Website pages

When the user asks for a page on the website, give the link the avatars
server returns, never one you build: an avatar's `links` (`get_avatar`,
`list_avatars`, `atmee avatars get --json`) are `chat`, `profile` and
`studio`; `get_account`'s `links` are `explore`, `avatars`, `create`,
`billing`, `apiKeys`, `analytics` and `docs`. For a link to talk to an
avatar, give `links.chat` from `get_avatar`: it carries the avatar's
language.

Only when a response has no `links` (a server that predates them), build
the link: `https://<host><path>`, where `<host>` is `www.atmee.ai`; never an
API host, which serves no website pages.

| They ask for | Path |
|---|---|
| a link to talk to an avatar | `/avatars/<avatar id>?lang=<language>` |
| its public profile | `/avatars/<avatar id>/profile` |
| editing it in Studio | `/studio/avatars/<avatar id>/edit` |
| Explore | `/studio/explore` |
| their avatars | `/studio/manage` |
| making an avatar in Studio | `/studio/create` |
| credits, plan, top-ups, earnings | `/studio/billing` |
| API keys | `/studio/api-keys` |
| analytics | `/studio/analytics` |
| a creator's page | `/@<handle>` |
| the API docs | `/docs/api` |

`?lang=` is the avatar's language, so the call is in it. A private avatar's
pages open only for its owner, signed in. A link to talk to an avatar is
always its chat page: never start a session for it (`start_session`,
`session chat --open`) unless the user asks for a LiveKit room by name.

## Ideas worth knowing

- **An avatar** is a portrait, a voice (cloned from a sample), a persona
  (who it is, what it does with the visitor, greeting, guardrails, names) and optional knowledge
  files. It speaks one language; a session can ask for another.
- **Two engines.** v1 animates a talking head from one portrait: sharp and
  cheaper. v2 adds emotion,
  expression and motion. **Atmee V2** comes with Growth and up; below
  Growth, an invite code unlocks it (the agent redeems it with
  `redeem_code`, `atmee redeem CODE` in a terminal, or Creator Studio's
  Billing page), or an email invite does (waitlist:
  `https://www.atmee.ai/v2signup`). Building spends no credits but needs
  some balance usable on Atmee V2; viewers of someone else's V2 avatar need
  Atmee V2 too unless it is sponsored. `get_balance`'s
  `v2SpendableMicrocredits` is how much of the balance is usable on Atmee
  V2, and `get_account`'s `v2Access` whether an invite code unlocked it
  (where V2 is still invite-only, that is what decides). A v2 avatar is captured several times;
  each capture is a **seed** (the server says candidate or take, and the
  API's `seed` field is an informational number, not the clip) and the
  creator picks the one that serves. Studio, the Avatar API and the agent
  tooling (`create-avatar`) build v2 for an account that can, and v1 for
  any account. Agents go to v2 by default: without Atmee V2 on the account
  they suggest Growth and up or ask for an invite code first, and make v1
  only if there is neither and the creator agrees.
- **Atmee V2 HD** (engine `v2hd`) is a larger v2 model on its own GPUs, at
  larger frames (640×640 by default; `limits.v2ProResolutions` lists the others). It builds,
  picks seeds and serves exactly like v2, through the same tools and
  sessions, at its own per-minute rates (`get_account` `pricing`,
  `sessionV2HdPerMinuteMicrocredits` and the like). It needs what v2 needs,
  Atmee V2 on the account, and only some servers offer it yet: `get_account`'s
  `limits.v2Pro` is true where the account can make one, false where the
  plan lacks it (where billing still decides V2 HD by plan), and absent where the server doesn't offer it (then don't
  mention it). `limits.v2ProResolutions` lists its sizes.
- **`/v1` in a path is the API version, not the engine.** Every endpoint
  starts with `/v1` (`POST /v1/sessions`, `/v1/avatars`), and v1 and v2
  avatars use the same ones. Never infer an avatar's engine from a path or
  call a `/v1` endpoint "the v1 API"; read `engine` on the avatar. To a
  person, name the engines Atmee V1, Atmee V2 and Atmee V2 HD.
- **Visibility:** private, unlisted (anyone with the link) or public (listed
  on atmee.ai). Through the API an avatar starts private or unlisted and
  goes public by an update once it is ready.
- **Status:** building, then ready, or failed, or `moderation_hold` (a person
  reviews it; never retry). Talk only when `readyToChat` is true.
- **Credits** pay for everything: conversations by the minute, generation
  per call. The API counts microcredits: divide by 100 for credits.
- **Monetized and sponsored are opposites,** never one setting. A
  **monetized** (Signature) avatar charges the visitor a higher per-minute
  rate on atmee.ai and the creator earns from those minutes. A **sponsored** avatar lets
  visitors talk on the creator's credits from its sponsor pool, and earns
  nothing. An avatar can't be both. Rates come from `get_account`, never
  from memory (`references/pricing.md`).
- **An avatar can talk without being spoken to.** An app makes it remark on
  something with an `event`, an agenda's steps carry it on by itself, and it
  speaks up after a silence. Don't tell anyone it needs the user to speak
  first, or claim limits these files don't state: say you'll check, and read
  `build-avatar-app`'s limits.
- **One conversation at a time per avatar,** one user in it. More users at
  once need more avatars.
- **The conversation runtime.** v2 conversations on the plugin's servers run on Causeur,
  which gives apps their controls: room messages (`state`, `event`, typed
  turns), app tools (the avatar calls the app) and agendas (a conductor graph
  of steps). A session the older behavior module runs accepts tools and
  never calls them.

## Which path fits

Match the idea to a row, name the path in one line, then hand over to the
last column.

| They want | Path | Then |
|---|---|---|
| An avatar people can talk to: themselves, a brand, a character | Studio for a v1 avatar on atmee.ai; for v2 (early access), the avatars server | Studio needs no agent; `create-avatar` for v2 |
| A talking avatar in their website or app, with Atmee running the conversation | **Session API**: their backend starts the session, their page joins the LiveKit room (there is no embeddable widget) | `build-avatar-app` |
| The avatar to act on their app: turn a slide, look up an order, keep score | Session API with **app tools** | `build-avatar-app` |
| A conversation with steps: an interview, a lesson, an onboarding | Session API with an **agenda** | `build-avatar-app` |
| A face for a voice agent they already run on LiveKit Agents | **Render path**: `livekit-plugins-atmee` (v1 and v2 avatars from 0.2.0), or the API's avatar sessions for any engine (V2 HD only this way); a portrait-only avatar is enough | `https://www.atmee.ai/docs/livekit-plugin/livekit-agents-plugin` |
| Visitors talking free on the owner's credits | a **sponsor** pool on the avatar | Studio → Billing, or directly through the agent or the CLI: signed-in is enough (`references/pricing.md`, Sponsoring) |
| An avatar live in front of an audience: a stream, a webinar | the broadcast-host shape on the Session API, restreamed (Current limits) | `build-avatar-app` |
| To earn from their avatar | the verified creator program on atmee.ai | Studio; nothing to build (`references/pricing.md`, Earnings) |
| A portrait, a voice or a short video they don't have | the avatars server's generation tools, paid per call; a clip of 1 to 15 seconds is priced by its size (480×480 up to 1280×720 or 720×1280; 1080p coming soon) and its length, charged at the next length up (up to 5, 10 or 15 s), with or without sound, and needs Atmee V2 (Growth and up, or an invite code) | `create-avatar` |
| A voice conversation over a WebSocket, with no video | **Speech session** (`start_speech_session`, `atmee speech start`); the avatar needs a voice | `build-avatar-app` |
| To manage avatars from code or CI | the Avatar API or the `atmee` CLI | `docs/cli.md` in the GitHub repo `Atmanity/atmee-toolkit` |

Proven app shapes, the sample apps that proved them, and what they taught:
`references/use-cases.md`.

## Current limits

Say up front when an idea needs what Atmee does not do yet, with the closest
path that works:

- **An audience** watching one conversation: one user token per session, no
  viewer tokens; a broadcast restreams the avatar's video. atmee.ai's own
  live streaming is not open to everyone.
- **Long sessions:** the plan caps a session's length; longer use chains
  sessions, with a visible cut.
- **A different persona per session:** the persona belongs to the avatar;
  per-app behaviour comes from `state` and the agenda, or another avatar.
- **Eyes through the API:** the app runs its own vision model and sends what
  it sees as `state` and `event`.
- **A transcript from the server:** the room's captions are the record; the
  app keeps them if it needs one.
- **Built but not open to everyone:** live streams and the avatar profile
  page on atmee.ai, mini-apps and apps inside a call, and voice design in
  Studio (the generation server has it). Name these as coming, never promise
  them; `references/features.md` marks each where its surface is described.

`build-avatar-app`'s "Current limits" has the full list for builders.

## Pricing

Prices and plans change: quote them only from a live source, with the date
you read them.

- **Plans and credit packs:** Studio → Billing,
  `https://www.atmee.ai/studio/billing` (sign-in needed). There is no public
  pricing page; `/pricing` sends visitors to sign in.
- **API costs, public:** `https://www.atmee.ai/docs/api/overview#costs`:
  credits per minute for the Session API and for the render path, what a
  credit costs depending on how it is bought, and what the Free plan allows
  through the API.
- **The account in hand,** from the avatars server: `get_account` (`atmee
  account`) for the plan, its `limits` and per-minute `pricing`;
  `get_balance` (`atmee balance`) for what is spendable now. Each generation
  tool's price is in `get_account` under `pricing.generation`.

Credits, buckets, what costs what, spend caps, sponsoring and earnings:
`references/pricing.md`.

## Rules

1. **Real people.** An avatar may be the creator or someone who authorised
   them, and the creator keeps a record of that consent (Terms of Use). Their
   own photo and voice sample are fine once permission is confirmed; never
   generate a real person's face or design a voice to sound like them.
2. **Restricted content** (`https://www.atmee.ai/terms-of-use`): hate,
   sexual content, anything involving minors, graphic violence, extremism,
   illegal activity, defamation, unauthorised endorsements, dangerous
   behaviour such as self-harm, and medical, legal or financial advice. An
   avatar in health, law or money informs and points to a professional.
3. **Public only on request,** after the creator approves the title,
   description and tags.

## References

| File | Read when |
|---|---|
| `references/features.md` | a question about what one surface does: atmee.ai, Studio, the API, the agent tooling |
| `references/use-cases.md` | matching an idea to a proven shape, or showing what others built |
| `references/pricing.md` | how credits, plans, packs, spend caps, sponsoring and earnings work |
