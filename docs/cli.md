# CLI Reference

`atmee` is a single Go binary, released with the plugin as source that
`go install` builds. Avatar commands use the
public Avatar API as a signed-in user (`atmee login`) or with an API key. To install it
and get started, see the [README](../README.md#install). For building an app around an
avatar (sessions, app tools, agendas), see the `build-avatar-app` skill's
[references](../plugins/atmee/skills/build-avatar-app/references/). Every command also has
`--help`.

## Configuration

| Variable | Default | Used by |
|---|---|---|
| `ATMEE_BASE_URL` | `https://api.atmanity.us` (local mock `http://localhost:8091`) | every command that calls the API, and `atmee login` |
| `ATMEE_API_KEY` | the key saved by `atmee login --api-key` | every command that calls the API |
| `ATMEE_ACCOUNT_ID` | the account chosen with `atmee login --account` | a signed-in user who belongs to several accounts |
| `ATMEE_HOME` | `~/.atmee` | where the saved sign-in or key lives |
| `ATMEE_NO_BROWSER` | unset | print the sign-in and candidate picker addresses instead of opening a browser |

The saved sign-in or key lives in `~/.atmee/credentials.json` (mode 600); saving one
replaces the other. `ATMEE_API_KEY`, when set, wins over both.

## Auth

```bash
atmee login [--account <id>] [--no-browser] [--json]   # sign in with the browser
atmee login --api-key [--json]  # or: prompt (hidden) for an Avatar API key and save it
atmee whoami [--json]          # who the commands act as, the server, and from the API: the user and account, or the key's name, scopes, avatar, spend cap
atmee logout                   # remove the saved sign-in or key
```

`atmee login` prints `Signed in as <email> -> <base URL>` and the account id. It is the
same sign-in as the MCP server's: it finds the Avatar API's
authorization server (`/.well-known/oauth-protected-resource`), registers the CLI there
for a `127.0.0.1` redirect, opens the sign-in page with PKCE, and saves the tokens. The
commands then act as you, on your account (`Authorization: Bearer`); the access token is
renewed before it expires. A login that belongs to several accounts picks one with
`--account` or `ATMEE_ACCOUNT_ID` (sent as `X-Account-Id`). A sign-in is for the
`ATMEE_BASE_URL` it was made against: set `ATMEE_BASE_URL` first, and sign in again to use
another API. The sign-in must finish within 5 minutes. `--no-browser` prints the link
instead of opening it; the page redirects to `127.0.0.1` on the machine running `atmee`,
so open it in a browser on that machine (or forward the port).

`whoami` reads the caller from `GET /v1/api_keys/current`. When the API cannot be reached
(offline, a wrong `ATMEE_BASE_URL`) or refuses the credentials, it prints the saved ones and
the server and says why on stderr; it still exits 0. `--json` adds `key` (the API's answer)
or `key_error`.

Local files go up in the create or replace request itself, so the CLI does not use
`POST /v1/uploads`, which is for clients that cannot send a file in the same request.

## Account

```bash
atmee balance [--json]         # credits left on your account
atmee account [--json]         # plan, limits, prices per minute, v2 and V2 HD sizes, capture quota, payout status, v2 access
atmee redeem CODE [--json]     # redeem an invite or credit code (needs atmee login, not an API key)
```

Both work for a signed-in user, and for a key with `sessions` or `avatars:read` (an
avatar-scoped key works too); any other key gets `403 insufficient_scope`. Amounts show in credits (100 microcredits = 1
credit); `--json` is the API's answer, in microcredits.

```
$ atmee balance
Spendable  50,000.00 credits
  Plan     1,000.00
  Top-up   49,000.00
  Coupon   0.00
Usable on Atmee V2  50,000.00
Next expiry  1,000.00 plan credits on 2026-11-01

$ atmee account
Account 11111111-1111-4111-8111-111111111111
Plan         Scale (scale) · 1,000.00 credits a cycle · cycle ends 2026-11-01
API access   on
Limits       session length no plan cap · 7 at once · 50 API avatars
Per minute   conversation v1 1.00/min, v2 2.00/min · avatar session v1 0.50/min, v2 1.00/min
Preview clip 0.00 credits
Generation   portraits 1.00 · enhance 1.00 · voice 1.00 credits
Clip sizes   480x480 0.10/0.30/0.50, 640x480 0.10/0.30/0.50, 480x640 0.10/0.30/0.50, 832x468 0.30/0.70/1.00, 468x832 0.30/0.70/1.00, 720x720 0.50/1.00/1.50, 960x720 0.50/1.00/1.50, 720x960 0.50/1.00/1.50, 1280x720 0.80/1.40/2.00, 720x1280 0.80/1.40/2.00 credits a clip up to 5/10/15 s, charged at the next length up (--resolution)
v2 sizes     512x512, 672x384, 384x672, 576x448, 448x576 (--resolution)
V2 HD        Enterprise only, on custom plans: contact us through Creator Studio's help center (https://www.atmee.ai/studio/help-center)
V2 HD sizes  640x640, 864x480, 480x864, 736x544, 544x736 (--resolution)
Captures     0 of 10 queued or running
Payouts      verified since 2026-09-01
```

**Spendable** is what a session can be started on. A metered key spends plan and top-up
credits only, so its coupon line says it is not spent. **Usable on Atmee V2**
(`v2SpendableMicrocredits`, shown where the server reports it) is the part that pays for Atmee
V2 sessions (V2 HD avatars' included) and video clips, and building a V2 avatar needs it above 0. Atmee V2
comes with Growth and up; below Growth, an invite code unlocks it. Credits held by running sessions
are already out of the buckets; what a session does not use comes back when it ends.
**Payouts** is the status of the account's payout account (`not started`, `unverified`,
`pending`, `verified` or `suspended`); monetizing an avatar needs it verified, and payouts
are set up in Creator Studio. **Per minute** shows a conversation rate for each engine; a server
that doesn't report the v2 rate shows one `conversation` rate, which then applies to both.
Atmee V2 HD's rates are not shown: it is for Enterprise customers on custom plans
(`--json` carries what the server reports).
**Clip sizes** are the sizes `generate clip --resolution` takes on this server, each with its
price per clip for up to 5, 10 and 15 seconds: a clip pays the next length up, so a 7-second
clip pays the 10-second price (`pricing.generation.clipPrices` in `--json`). A server without `clipPrices`, which priced
each size the same for any length, shows one price per size.
An older server without them shows one clip price per 15 seconds on the **Generation** line
instead. **v2 sizes** (shown when the deployment offers only some of the sizes the rule
allows) are the frame sizes an avatar's `--resolution` takes (`limits.v2Resolutions`; on an
older server, `pricing.generation.clipResolutions`). **Video clips** appears only when the
account may not make clips (`limits.videoGeneration` false, where billing still decides clips
by plan). **V2 HD** appears only where the server reports Atmee V2 HD (`limits.v2Pro`, true or false):
it is for Enterprise customers on custom plans, so the line says to contact us through Creator
Studio's help center, on the website the account's `links` point to (named without an address
when the server reports no links). **v2 avatars** says whether an invite code
unlocked Atmee V2; on Growth and up it comes with the plan.
**V2 HD sizes** are the frame sizes a V2 HD avatar's `--resolution` takes on this server
(`limits.v2ProResolutions`, 640x640 first); the example above is one server's list.

## API keys

```bash
atmee keys list [--all] [--json]                 # the account's keys; secrets are never shown
atmee keys create --name NAME [--scopes S,...] [--avatar AVATAR_ID] [--max-session 15m] \
  [--max-concurrent N] [--spend-cap CREDITS] [--spend-cap-window daily|monthly|total] \
  [--visitor-memory] [--out FILE [--env-var NAME]] [--json]
atmee keys update KEY_ID [--name NAME] [--scopes S,...] [--avatar AVATAR_ID | --any-avatar] \
  [--max-session 15m] [--max-concurrent N] [--spend-cap CREDITS] [--spend-cap-window daily|monthly|total] \
  [--visitor-memory[=false]] [--json]
atmee keys rotate KEY_ID [--out FILE [--env-var NAME]] [--json]
atmee keys revoke KEY_ID [--yes] [--json]
```

Only a signed-in user (`atmee login`) who is the account's owner or an admin manages
keys; with an API key in use (`ATMEE_API_KEY`, or one saved by `atmee login --api-key`)
the commands stop before sending anything (the API answers a key `403 login_required`,
so a key cannot mint keys), and a member gets `403 owner_or_admin_required`. A secret in
`--out .env` is not picked up by `atmee` itself: keep `ATMEE_API_KEY` out of the shell
that manages keys.

- **A secret exists once**, in the answer to `create` or `rotate`. With `--out FILE` the
  CLI writes it to the file as `ATMEE_API_KEY=...` (`--env-var` names another variable),
  replacing that variable's line and keeping every other one, readable by you only, and
  never prints it; a symlinked file keeps its link. `--out` is checked before anything is
  sent. Without `--out` it is printed once (also with `--json`), with a warning.
- **Keep keys narrow**: `sessions` (start and end conversations) is always included; add
  `avatars:read`, `avatars:write`, `billing:write` or `generation:write` only when the app needs them, bind a
  key that serves one avatar with `--avatar`, and set a spend cap. Unset caps take the
  database's defaults (the plan is the ceiling).
- **Update** changes only the flags you pass and keeps the secret. `--scopes` replaces the
  whole set, `--any-avatar` widens a bound key to the account. The plan's ceilings apply
  again on every edit, also to limits you did not pass, so read the limits from the answer.
  A revoked key is `409 api_key_revoked`. An edit that fails partway is safe to send again.
- **Rotate** keeps the key's id, name, scopes and caps and issues a new secret; the old one
  stops working within about 30 seconds. **Revoke** is for good (again is no error);
  running sessions keep going. `list` hides revoked keys unless `--all`.

```
$ atmee keys create --name "stream app" --avatar 11111111-2222-4333-8444-555555555555 --spend-cap 50 --out .env
Created key "stream app" (aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee); its secret is in .env as ATMEE_API_KEY.
stream app  aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee
  Secret      sk_atmee_01234567…89ab
  Scopes      sessions
  Avatars     only 11111111-2222-4333-8444-555555555555
  Limits      50.00 credits a month · sessions up to 1h0m0s · 3 at once
  Created     2026-10-22 · never used
```

The MCP server's `create_api_key` tool keeps the secret out of the conversation: rotate
the new key here with `--out` to get one.

## Avatars

```bash
atmee avatars create --name NAME [--image IMG] [--voice SAMPLE] [flags] [--wait] [--json]   # without one: a draft
atmee avatars list [--limit 50] [--before CURSOR] [--all] [--json]
atmee avatars get ID [--json]                    # status, and what was set: persona, language, facts and knowledge summary, public page
atmee avatars get-knowledge ID [--json]          # the --facts text, questionnaire answers, knowledge files with links
atmee avatars questionnaire ID [--json]          # Studio's knowledge questions, numbered, with the answers
atmee avatars get-appearance ID [--out FILE] [--json]   # the portrait last given (a link valid for an hour, or saved)
atmee avatars get-voice ID [--out FILE] [--json]        # the voice sample last given
atmee avatars wait ID [--timeout 900] [--json]   # until ready, a v2 pick is needed, or it is a draft
atmee avatars update ID [field flags] [--json]
atmee avatars replace-appearance ID FILE        # also completes a draft
atmee avatars replace-voice ID FILE             # also completes a draft
atmee avatars replace-knowledge ID [FILE...]     # no files clears the set (the --facts text keeps its own file)
atmee avatars thumbnail set ID IMAGE [--json]    # the Explore thumbnail: jpg, png or webp, at most 10 MB
atmee avatars thumbnail remove ID [--yes] [--json]   # asks first; Explore shows the portrait instead
atmee avatars monetization ID [--json]           # monetized, 30-day eligibility, the plan's cap, earnings
atmee avatars delete ID [--yes]
```

`--image` is the portrait: front-facing, the face at least a third of the frame, lips
slightly parted as if about to speak (a closed mouth makes the first second of speech
fail; the build refuses a clearly bad portrait with a `portrait_*` reason). The same
goes for `replace-appearance`.

`--image` and `--voice` are optional. Without one the avatar is a **draft**
(`status: draft`, with `missingInputs`): its persona and knowledge build, and
`replace-appearance` / `replace-voice` complete it, in either order (`--wait` returns
the draft at once). A v1 avatar with only `--image` is render-only.

**Runtime seed (v2).** Two seeds shape a v2 avatar: the candidate's seed picks
the take (its look and lip sync; you choose it with `candidates`), and the
runtime seed varies the motion while it talks in a conversation. Most avatars
are fine on the default (42); if the motion during conversations looks off
while the take itself is good, try another with `atmee avatars update ID
--runtime-seed 7`. It applies from the next session and rebuilds nothing;
`avatars get` shows the one in use, and `--runtime-seed 42` goes back to the
default. It is an API setting only: Creator Studio doesn't show it.

**Resolution (v2).** `--resolution WIDTHxHEIGHT` sets the frame size an avatar's
candidates, previews and conversations render at: 512x512 (1:1, the default), 672x384
(16:9), 384x672 (9:16), 576x448 (4:3) or 448x576 (3:4), as the server offers them. A size
the deployment does not offer is `400 invalid_input`, and the message lists the ones it does
(`atmee account` shows them as `v2 sizes`); avatars already built at an older size keep
it. The portrait is
framed at that size, so on `update` a new size builds a new version with new candidates, as
a new portrait does, while the current candidate keeps serving; the current size sent again
builds nothing. v1 avatars refuse it. `avatars get` shows it, e.g. `Resolution: 672x384`.
A V2 HD avatar has its own sizes, the ones the server lists as `V2 HD sizes` in
`atmee account`: 640x640 (1:1, the default), 864x480 (16:9), 480x864 (9:16), 736x544 (4:3)
or 544x736 (3:4); 512x512 and the other v2 sizes are not V2 HD sizes. The CLI checks the size against the engine's rule
and, where `atmee account` lists sizes, against that list before sending; the server has the
last word.

**Atmee V2 HD (`--engine v2hd`) is for Enterprise customers on custom plans.** To get it,
contact us through Creator Studio's help center (`atmee account` gives its address). A larger
v2 model, on its own GPUs: it builds, previews and serves exactly like v2 (candidates,
versions, the conductor, the runtime seed) at larger frame sizes. Existing V2 HD avatars keep
working: `avatars list` and `get` show their engine as `v2hd`, and they take the same
commands as v2 ones. `--engine v2pro`, its first name, still works. A V2 HD create, capture or
rebuild the account may not make answers `403 v2_pro_not_in_plan`; a server without V2 HD
answers `503 engine_unavailable`; the CLI adds what to do. Video clips are the same whatever
engine you use.

`create` flags: `--engine v2|v1|v2hd` (default v2; v1 for an account that cannot build v2: Atmee V2 not on its plan and not unlocked by an invite code (`atmee balance` shows nothing usable on Atmee V2), or a v2 create refused with `insufficient_v2_credits` or `v2_access_required`; v2hd for Atmee V2 HD, for Enterprise customers, above), `--candidates 1-10`
(default 1 on v2), `--seed-selection auto|manual` (default auto on v2: the first candidate
finished is used; both need `--image` and `--voice`, a v2 draft takes one candidate,
used automatically, once completed), `--preview-message` (v2: at most 300
characters), `--runtime-seed` (v2: 0-2147483647, default 42; also on `update`, see below), `--resolution WxH` (v2: default
512x512; also on `update`, see above), `--who-prompt` (Studio's "Who are you"), `--what-prompt` ("What are we doing"), `--greeting`, `--self-intro`,
`--guardrail` (repeatable), `--facts`/`--facts-file` (text it knows, up to 32,000
characters, saved as Studio's "Facts with context"), `--knowledge FILE` (repeatable; v1
conversations use knowledge, v2 ones do not yet),
`--alias`, `--language`, `--visibility`, `--previews`/`--no-previews`,
`--sponsor-microcredits N` and `--sponsor-max-per-session N` (fund the sponsor pool on
create; needs `billing:write`), `--config FILE|-`.

Image, voice and knowledge are all https URLs or all local files; local files are sent as
one zip bundle. A create is sent once: after a network failure, check `atmee avatars list`
before running it again.

`--personality`/`--who` and `--scenario`/`--what` are older names of `--who-prompt` and `--what-prompt`, on
`create` and `update`; they still work but are hidden from `--help`.

`update` flags: `--name`, `--description`, `--visibility` (private, unlisted or public), `--language`, `--who-prompt`,
`--what-prompt`, `--greeting`, `--self-intro`, `--guardrail`, `--alias`, `--spoken-name` (how the name is pronounced),
`--known-alias` (repeatable; other names people call it, no commas),
`--habit NAME=LEVEL` (repeatable; Studio's conversational style sliders: engagement, wordiness, delivery, inquiry, focus = low|medium|high; the others keep their level), `--preview-message`,
`--facts`/`--facts-file` (replaces the text; other knowledge files stay),
`--answer QUESTION=ANSWER` (repeatable; a question of Studio's knowledge questionnaire, its text or its number in
`atmee avatars questionnaire`; the other answers stay, an empty answer removes that one),
`--runtime-seed` (v2: 0-2147483647; `42` goes back to the default; applies from the next session),
`--resolution WxH` (v2: a new size builds new candidates),
the Explore profile (below), `--monetize on|off` and `--confirm-sponsor-withdrawal`,
and `--clear FIELD` (repeatable) to remove a value. Fields you leave out stay unchanged.
`--clear` sends the field empty and takes `description`, `greeting`, `alias`, `spoken-name`, `known-aliases`, `habits`,
`guardrails`, `facts`, `questionnaire`, `title`, `subtitle`, `tags`, `socials`, `statement` (resets it) and
`preview-message` (back to the default sentence); the who and what prompts and self-introduction cannot be
emptied. `list` takes `--limit`, `--before` and `--all`.

**Explore profile.** What Creator Studio's "Share to community" card edits, on either engine; nothing is
rebuilt: `--title` (at most 100 characters), `--subtitle` (200), `--description` (280), `--tag` (repeatable,
the whole set: at most 5, case-insensitive duplicates dropped), `--social TYPE=URL` (repeatable, the whole set:
instagram, tiktok, facebook, twitter or youtube, each once, with an https URL), and the thumbnail
(`atmee avatars thumbnail set ID IMAGE`). `--visibility public` needs a title, a tag, a subtitle, a description
and a thumbnail; the API answers `409 public_requirements_missing` with what is missing, and the CLI names the
flags that give it. `--statement self|representative|fictional` records who the avatar depicts (self and
representative are verified in Creator Studio; changing it clears the verification and is refused while the
avatar is monetized).

**Monetization.** `atmee avatars monetization ID` shows whether it is on, the last 30 days (your own
conversations left out) against the requirements, the plan's cap on monetized avatars and the earnings in
credits. `update --monetize on` needs a statement (`fictional`, or a verified self or representative one),
verified payouts (`atmee account`), the 30-day requirements and room under the cap; each refusal (`409
statement_required`, `identity_not_verified`, `payouts_not_ready`, `monetization_eligibility_not_met`,
`monetization_limit_reached`, `avatar_is_earning`) is printed with what to do. When the sponsor pool holds
credits, monetizing withdraws them to the account: the CLI asks first (like `delete`), and
`--confirm-sponsor-withdrawal` answers yes. `--monetize off` always works.

```
$ atmee avatars monetization 11111111-2222-4333-8444-555555555555
Monetized    no
Eligible     no
Last 30 days ✓ 12 of 10 conversations · ✗ 40.50 of 60 minutes · ✗ 3 of 5 people
Account      1 monetized · plan cap 3
Earnings     2.50 credits this cycle (1 session) · 1,234.56 credits all time (40 sessions)
```

### Config file

`--config` takes a JSON object (or `-` for stdin) with the create fields in snake_case.
Flags win over it, and its `name` is ignored (`--name` is the name). An unknown field is
an error, so a typo never creates an avatar with defaults. `personality` and `scenario` are still read as the
older names of `who_prompt` and `what_prompt` (giving both names of one field is an error). `resolution` takes `"672x384"` or the
API's `{"width": 672, "height": 384}`. Relative file paths are read
from the directory you run `atmee` in.

```bash
# set "image" and "voice" in the file to your own portrait and voice sample first
atmee avatars create --name Coach --config examples/create_with_config.json --wait
```


```json
{
  "image": "https://example.com/face.png",
  "voice": "https://example.com/voice.wav",
  "engine": "v2",
  "seed_selection": "auto",
  "seed_previews": 1,
  "preview_message": "Hi! Want to talk plants?",
  "who_prompt": "A cheerful gardener.",
  "what_prompt": "Helps visitors choose houseplants.",
  "guardrails": ["No medical advice."],
  "facts": "Favourite plant: basil.",
  "language": "en",
  "sponsor_microcredits": 1000,
  "sponsor_max_per_session": 200,
  "runtime_seed": 7,
  "resolution": "512x512"
}
```

## v2 candidates

```bash
atmee avatars candidates list ID [--all] [--json]         # with preview links and quota; --all adds older versions
atmee avatars candidates pick ID [--more 3] [--timeout 300]  # side by side in the browser; waits until the pick serves
atmee avatars candidates add ID [--count 1] [--wait]      # sent once; list before re-running after a network failure
atmee avatars candidates select ID CANDIDATE_ID [--wait=false] [--timeout 300] [--json]  # waits until the pick serves
atmee avatars candidates sheet ID [--out candidates.png]  # needs ffmpeg
```

A new portrait, voice sample or preview message starts a new version; older versions'
candidates are kept. Selecting a completed one of an older version (`list --all`) goes back
to that version: nothing is captured again or charged, and the persona, greeting,
guardrails, knowledge and name stay as they are.

The contact sheet has one row per completed candidate: twelve mouth crops spread over the
preview clip. A row whose lips never part is a dead candidate.

## v2 conductor

```bash
atmee avatars conductor set ID --graph FILE.json [--speak-after SECONDS] [--json]  # FILE may be - (stdin)
atmee avatars conductor get ID [--json] [--out FILE.json]
atmee avatars conductor delete ID [--yes]                 # asks first
```

A conductor is a fixed agenda for a v2 avatar's conversations, a graph of steps run by
Causeur ([conductor.md](../plugins/atmee/skills/build-avatar-app/references/conductor.md)
for the format and limits). `--graph` takes the graph itself (`{"start", "nodes"}`) or a
whole request body (`{"graph", "speakAfterS"}`, what `get --out` saves); a top-level
`graph` key marks the second. `--speak-after` (0.1-10 s) replaces the file's
`speakAfterS`. `set` and `get` print the node and edge counts, the start node and each
step's edges; `avatars get` shows `Conductor: N nodes` on a v2 avatar.

It applies from the next session and rebuilds nothing. A v1 avatar is refused
(`409 not_v2_avatar`); a refused graph names the field (`400 invalid_conductor` at
`/graph/nodes/0/edges/1/weight: ...`, with the path in a bare-graph file too); `get` with
none set is `404 conductor_not_set`.

## Earnings

```bash
atmee earnings [--json]                         # the creator wallet: earned, paid out, balance
```

`earnings` is what monetized avatars earned, not the credits sessions spend (`atmee
balance`); it works signed in or with an account-wide key with `avatars:read`.

## Generate

```bash
atmee generate portraits --description TEXT [--engine v1|v2|v2hd] [--idempotency-key K] [--json]
atmee generate enhance --image PATH|URL|upload:ID [--prompt TEXT] [--engine v1|v2|v2hd | --avatar ID] \
  [--idempotency-key K] [--json]
atmee generate voice --description TEXT [--name NAME] [--language LANG] [--text SAMPLE] \
  [--idempotency-key K] [--json]
atmee generate clip --image PATH|URL|upload:ID --prompt TEXT [--audio PATH|URL|upload:ID] \
  [--duration SECONDS] [--resolution WxH] [--seed N] [--idempotency-key K] [--json]
atmee generate clip-status REQUEST_ID [--wait] [--json]
```

Each answers with `upload:<uploadId>` references and short-lived links to look or listen.
`avatars create --image/--voice` and `avatars replace-appearance/replace-voice` take a
reference as they are, as they do an https URL (with `create`, not mixed with local
files). A reference works once and lasts 24 hours.

- **`portraits`** is for invented characters only. For a real person, with their
  permission, use `enhance` on their own photo.
- **Prices:** `portraits`, `enhance` and `voice` each cost one price per call (`atmee account`),
  charged when something comes back. `portraits` and `enhance` make up to two portraits, one from Gemini and one from
  Grok, and one costs the same as two. A paid plan's free generation allowance pays first.
- **`enhance`** keeps the subject's identity; with `--prompt` it restyles the photo. A local
  file (jpg, png or webp, at most 10 MB) goes up through `POST /v1/uploads` first.
- **`--engine`** is the engine the avatar will be created with; pass it on both. Without it the
  API uses v1, which turns the face to the camera, frames it from the shoulders up and, in
  `enhance`, blurs the background. v2 keeps the photo's pose, hands, framing and background
  and only improves quality; a v2 `portraits` call frames the character half-body unless the
  description sets another framing. With `--prompt` the restyle is the same for both.
- **`--avatar`** is for a new photo of an existing avatar: the avatar's own engine is used, so
  leave `--engine` out (a different one is refused as `invalid_input`).
- **`voice`** makes previews of 10 to 60 seconds. `--description` is 20 to 1000 characters;
  `--text` is what every preview says, in `--language`.
- **`clip`** renders an mp4 from an image and a prompt, silent or speaking
  `--audio`; no avatar is made or needed. `--duration` is required with `--audio`, set to the
  audio's length, because it is not read from the file, and defaults to 8 for a silent clip.
  A clip lasts at least 1 second, up to the server's limit: 15 seconds on current servers
  (an older server allowed more); a longer `--duration` is refused by the server, which names
  its limit. `--resolution` is one of the sizes the server serves: `atmee account` lists them
  (`Clip sizes`), and on a current server each is priced by the clip's length, charged at the
  next length up (a 7-second clip pays the 10-second price), the same with or without audio.
  For example, a current server serves:

  | Size | Up to 5 s | Up to 10 s | Up to 15 s |
  |---|---|---|---|
  | 480x480 (the default), 640x480, 480x640 | 0.10 | 0.30 | 0.50 |
  | 832x468, 468x832 | 0.30 | 0.70 | 1.00 |
  | 720x720, 960x720, 720x960 | 0.50 | 1.00 | 1.50 |
  | 1280x720, 720x1280 | 0.80 | 1.40 | 2.00 |

  1080p sizes (1080x1080 and the wider 1080p shapes) are coming soon: a current server refuses
  them with `400 invalid_input`, so pick 1280x720 or 720x1280 for the largest clip.

  The CLI checks only that `--resolution` is two even numbers; a size the server doesn't
  serve is `400 invalid_input`, and the message lists the ones it does. Without
  `--resolution` the server's own default size is used. The renderer works at each side rounded up to a multiple of 32 and crops the
  centre. Local files go up through `POST /v1/uploads` first. It needs Atmee V2
  (Growth and up; below Growth, an invite code unlocks it): otherwise it is `402
  insufficient_v2_credits`. Where billing still
  decides clips by invitation and plan, it needs v2 access (`403 v2_access_required`) and a
  plan with video generation (`403 video_generation_not_in_plan`). It comes back as an `upload:<uploadId>` with a download
  link for 24 hours.
- **`clip-status`** reads a clip by its request id, the `--idempotency-key` a `clip` call was
  given or printed when it stopped waiting, through `GET /v1/generation/clips/{requestId}`. It
  charges nothing and never starts a render, so it works from another shell or machine with
  the same account. It prints `rendering` (`--wait` keeps reading, up to 10 minutes), the clip
  and a newly signed download link when `ready`, the upload when `expired` (made, but its
  24 hours are over; it is not made again), or, for `stalled` (the call stopped without
  finishing), how to resume it: run `clip` again with the same inputs and
  `--idempotency-key`, charged at most once, and it exits non-zero. A request id with no
  clip is `404 clip_not_found`: the call ended without a clip and charged nothing. It needs
  `avatars:read` or `avatars:write`.

Like the other generation commands, none of these asks first.

The price is in `atmee account`, charged only when something comes back. A call still
running is waited for with the same key, up to 10 minutes. After a network failure nothing
is sent again: the error prints the key, and `--idempotency-key` retries it without
charging twice. They work signed in, or with a key holding `generation:write` and
`avatars:write`.

## Sponsor and sessions

```bash
atmee avatars sponsor show ID
atmee avatars sponsor update ID [--max-per-session N] [--when-exhausted pause|spend_balance]
                                [--anonymous|--no-anonymous] [--anonymous-daily-seconds N]
                                [--clear-max-per-session] [--clear-anonymous-daily-seconds]
atmee avatars sponsor grant ID MICROCREDITS [--idempotency-key K]   # 100 microcredits = 1 credit
atmee avatars sponsor withdraw ID [--microcredits N]                 # all unreserved by default

atmee avatars session chat ID [--open] [--max-seconds N] [--visitor-id ID] [--language L]
                              [--no-welcome] [--check-in-after SECONDS]
                              [--tools FILE] [--tool-instructions FILE|-]
atmee avatars session render ID --livekit-url URL --livekit-token TOKEN [--agent-identity A]
                                [--max-seconds N] [--wait-joined]
atmee avatars session status SESSION_ID
atmee avatars session end SESSION_ID
```

`sponsor` and `session` commands always print JSON; `--json` is accepted for consistency.

Avatar-only starts (`session render`) of a v2 avatar retry `503 avatar_warming` (a
rendering node is fetching the newly published avatar) after `Retry-After` seconds, for up
to 10 minutes, and say so on stderr. v1 avatars and full sessions (`session chat`) report
it at once, like every other error, `no_capacity` included. On a cold v2 pool the start
itself can take minutes: pass `--wait-joined`, with which each attempt waits up to 10 minutes
for the join (on top of any warm-up retries); without it each attempt gives up after the
60 s request timeout.

`session chat` is for an app's own client, or for testing one: to give someone a link to
talk to an avatar, send its chat page, `links.chat` from `atmee avatars get ID --json`,
which needs no session or token. Besides this session's LiveKit Meet link, `session chat` prints the avatar's website chat page on stderr
from the start's `links.chat`, when the API returns it. `session chat` returns once the avatar is in the room. Join with `serverUrl` and
`userToken` within 60 seconds, or the session ends and the room is deleted. Leave the room
to end it; `--max-seconds` caps it. `session end` and `session status` are for
avatar-only sessions (`session render`).

`--visitor-id` unlocks that visitor's memory across sessions: use an unguessable id your
server derives (16-256 characters), never one from the browser.

`--no-welcome` starts without the avatar's self-introduction or welcome back, for an app
that has already met the user: the avatar waits for them to speak, though a long silence
still gets its check-in (`--check-in-after` sets it or turns it off). Only a v2 avatar on
the default conversation runtime (every avatar built through the API) honours it; any other
(a v1 avatar, or one with a launch URL of its own) ignores it, starts as usual and speaks
first. A speech session does the same with
`{"welcome": false}` in `--settings`. `session render` has no welcome: the avatar says only
what your agent sends.

`--check-in-after SECONDS` sets how long a silence lasts before the avatar speaks up, 0 to
3600, `0` for no check-in; a second check-in waits twice as long, then it stays quiet until
the user speaks. Left out, it keeps the default, about 10 seconds. Only a v2 avatar on the
default conversation runtime honours it; any other ignores it and keeps its usual check-in,
billed the same either way. A speech session does the same
with `{"checkin_after_s": 0}` in `--settings`. `session render` has no check-in.

### App tools on `session chat`

`--tools FILE` declares functions the avatar may call in this conversation: a JSON array
of 1 to 30 function tools (`type`, `name`, `description`, `parameters` as JSON Schema
2020-12, optional `strict` (ignored) and `x-atmee` `{mode, timeoutSeconds}`; see
[server.md](../plugins/atmee/skills/build-avatar-app/references/server.md#app-tools)). `--tool-instructions
FILE` (`-` reads stdin) is one text for all the tools, saying how and when to use them;
it needs `--tools`. The CLI sends both as `appTools: {tools, instructions}`. It only
checks the file is a JSON array of objects with a string `name` and sends it as written;
the API checks the limits (count, names, sizes, values) and answers `400 invalid_tools`,
naming the rule.

Every call reaches the visitor's LiveKit client as the RPC `atmee.tool`, which the page
that joins the room answers; `x-atmee.mode` `await` (with `timeoutSeconds`, 1 to 28) makes
the avatar wait for the answer. That page must be your own LiveKit client: LiveKit Meet
(`--open`, or the link the command prints) answers no `atmee.tool` call, so `await` calls
fail and `notify` calls are lost there. With `--tools` the command warns about it on
stderr.


## Speech sessions

```bash
atmee speech start --avatar ID [--max-seconds N] [--language L] [--settings FILE] [--instructions FILE] [--json]
atmee speech get SESSION_ID [--json]
atmee speech end SESSION_ID [--json]
```

A speech session is a speech-to-speech conversation over a WebSocket, without video, with
an avatar: it uses the avatar's persona and always its voice (`--avatar` is required; an
avatar without a voice answers `avatar_has_no_voice`). `start` reserves one and prints its
id, WebSocket `url`, when the connection token expires (60 seconds), the length granted
(and, when you asked for one, whether the key, plan or balance capped it) and the billing mode;
the token itself is printed only with `--json`, for the client that connects.
`--max-seconds` takes 60 to 3600 (default 600).

`--settings FILE` (`-` reads stdin) pins `session.start` fields from your backend: a JSON
object with any of `instructions`, `persona`, `facts`, `scenario`, `agenda`, `directive`,
`avatar_name`, `user_name`, `self_introduction`, `input`, `delegation`, `avatar_timezone`
and `user_timezone`. The client cannot set a pinned field, and pinned persona fields
override the avatar's. `--instructions FILE` pins the instructions alone (with
`--settings`, the file must not pin them too). The CLI refuses an unknown field; the API
checks the limits (`422 invalid_settings` names the field).

`get` shows the state, when billing started, `endsAt`, the end reason and cost, and, while
a connection is open after an `end`, when the end was requested. `end` settles a session
with no connection at once; with a connection open the session ends at its next usage
report, within 30 seconds, and is billed until then (`end` answers `active` and `get`
reads it `completed` once it settled). It is safe to repeat. A session nobody connects to
costs nothing and closes by itself (`failed`, `no_user_joined`) on the account's next read
or start once its token has been expired 30 seconds, otherwise within about 5 minutes. The
CLI streams no audio.

## Output and exit codes

- `--json` prints the Avatar API's own JSON (camelCase fields, e.g. `avatarId`,
  `selectionRequired`, `candidateId`). `create --wait --json` prints the settled avatar.
- Progress and notes go to stderr, so `--json` output can be piped.
- Exit 0 success, 1 failure (API error, failed build), 2 bad usage.
- Errors read `Error: <status> <code>: <message>`; a 429 adds `(retry in Ns)`.
