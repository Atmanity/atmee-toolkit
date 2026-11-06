# Credits and generation

## Contents

- When to offer generation
- Before each round of paid calls
- How much to spend
- The engine a portrait is for
- Portraits
- Enhancing or restyling the creator's photo
- Voices
- Clips
- Using generated outputs

The server makes portraits (`generate_portraits`), enhances or restyles the
creator's photo (`enhance_portrait`), designs voices (`design_voice`) and
renders short video clips (`generate_clip`). Each is a paid
call, charged only when results come back. Its price comes from
`get_account`, and the balance only from `get_balance` (SKILL.md rule 12).

## When to offer generation

- **The creator has no portrait or voice** in step 1, which needs both before
  it can create the avatar.
- **A pair is failing** and you've diagnosed which side (the candidates reference).

Don't generate up front "to have options". Don't generate a real person's
likeness from a description, ever (the people-and-publishing reference).

## Before each round of paid calls

1. Read the price from `get_account`: `pricing.generation` has
   `portraitsMicrocredits`, `enhancementsMicrocredits` and
   `voicesMicrocredits`, each the price of one call (a portrait call costs
   the same whether one portrait comes back or two), and
   `clipPrices` each size's prices by clip length (where clips are
   offered; an older server reports `clipPer15SecondsMicrocredits` instead,
   see Clips), each divided by 100 for credits; never assume 1.
   A paid plan's free generation comes first: where `pricing.generation`
   has `freeAllowance`, its `remainingMicrocredits` (what is left this
   cycle) pays for portraits, enhancements and voices before credits do
   (never for clips). Quote what the round costs in credits after it: the
   prices of those calls, less what is left of it (never below 0), and say
   the free part ("covered by your plan's free generation"). A result's
   `microcreditsCharged` is what credits paid, `microcreditsFree` what the
   allowance did.
   Read the balance from `get_balance`: `spendableMicrocredits` / 100.
   For a clip, `v2SpendableMicrocredits` / 100 (where present) is what pays:
   the part of the balance usable on Atmee V2.
2. Ask once for the whole round: every paid call the step needs (portraits
   and a voice together, for example), each with its price, then the total
   and the balance: "Make two portraits (<portrait price>) and voice previews
   (<price>), <total> in all? You have <balance> left." With one call, give its
   price and the balance. Wait for a clear yes in the creator's next message;
   it covers only the calls you named. Never split a round into one question
   per call, and never add a call to it after the yes.
3. For each call in the round, make up a new `requestId` (a UUID). Before sending,
   add a pending entry to `paid_calls` in the avatar's latest iteration file
   (the iterations reference; with no file yet, start `000-create.md` now):
   the tool, the date, the `request_id`, the exact `inputs` you will send and
   `status: pending`. In claude.ai, put the pending id and inputs in the chat
   summary instead. Then send the call with that `requestId` and
   `confirm: true`. The server refuses the call without either, so
   `confirm: true` goes only after the yes, never before.
4. If the call times out or returns `request_in_progress` or `rate_limited`,
   wait the seconds it gives (`retryAfter`, or "retry in N s" in its
   message), then send the **same** call again:
   the same `requestId`, the same inputs and `confirm: true`. Don't ask
   again: the same id never charges twice, and a call still running answers
   with its one result once it finishes. Tell the creator in one line that
   it is still being made. A wait of minutes means the charge is still being
   confirmed; the pending entry keeps the id safe if the turn ends first.
5. When the result comes back, mark the entry `status: done` with the
   credits charged (`microcreditsCharged` / 100). On an error that charged
   nothing, mark it `failed`; on the `invalid_input` that says the key was
   already used and charged, mark it `lost` (SKILL.md rule 8).

After each paid result, read `get_balance` again and mention what's left.

**A pending entry left behind** (a Resume, an interrupted turn, or a call
that never answered): before offering any new paid call, send it again with
its `request_id`, its exact `inputs` and `confirm: true`, without asking the
creator again, and handle the result as in step 5. Reuse a `requestId` only
for this retry of identical inputs; any change is a new request with a new id.

A result with `replayed: true` is an earlier result returned again, charged
nothing this time. Before using one of its outputs, check `unavailable`: an
unavailable output was already used or has expired and can't be used.

## How much to spend

Count the avatar's charged paid calls (`done` and `lost`) across its iteration
files. At 10, and every 10 after, stop before the next paid call and ask as a choice (SKILL.md rule 9):
"That's 10 paid generations for Mira so far. Keep going?" with "Keep going"
and "Stop here". On "Keep going", carry on to the next 10. Rule 1 still asks
before each round; this is a check-in on the total.

**On `insufficient_credits`:** say so, give the top-up link from the error's
message (or the billing page on atmee.ai), and stop. Don't retry, and don't
loop asking. **On `api_key_spend_cap_reached`:** the API key's spend cap is
used up; say so and stop the same way.

## The engine a portrait is for

A portrait is made for one engine, and the two want different pictures. v1
needs a frontal head-and-shoulders face, so for v1 enhancing turns the face
to the camera, frames it from the shoulders up and blurs the background, and
a generated portrait is framed from the shoulders up. v2 animates whatever
the portrait shows, so for v2 enhancing keeps the photo's pose, gaze, hands,
framing and background and only improves its quality, and a generated
portrait is framed half-body, from the waist up with both hands visible,
unless the description sets another framing. A new portrait for an existing
Atmee V2 HD avatar (`engine: v2hd`, the candidates reference, Model) is made
as for v2; never make one for a new avatar.

- **Settle the engine before the first paid portrait call,** in step 1:
  v2 when `get_balance` reports `v2SpendableMicrocredits` above 0; when it
  leaves that field out (billing still decides v2 by invite), when
  `get_account`'s `v2Access` is `granted`. A reported 0 means no v2,
  whatever `v2Access` says. Otherwise ask for an invite code
  (or suggest Growth and up) as the candidates reference's
  Create says, and use v1 only when the creator has neither and agrees. Use v1 too when the creator asks for
  it.
- **Pass it as `engine`** on every `generate_portraits` and
  `enhance_portrait` call for a new avatar, and write it into the pending
  `paid_calls` entry's `inputs`. Left out, the server makes a v1 portrait.
  Create the avatar with the same engine.
- **A new portrait for an existing avatar** (for `replace_appearance`):
  `enhance_portrait` takes its `avatarId`; leave `engine` out, so the server
  uses the avatar's own engine (an `engine` that disagrees with it is
  `invalid_input`). `generate_portraits` takes no `avatarId`: pass `engine`
  as the avatar's own, from `get_avatar`.
- **A restyle** (`enhance_portrait` with `prompt`) is the same for both
  engines; pass `engine` anyway.

**Falling back to v1 after a v2 portrait.** When the portrait was made for
v2 but the avatar is now made as v1 (no Atmee V2 on the account), that portrait may keep
a pose, hands or a background v1 animates badly. Ask before going on, as a
choice: for an enhanced photo, offer the creator's original photo first (it
costs nothing), then a v1 enhancement at its price; for a generated
portrait, offer to use it as it is, then a v1 portrait at its price. A v1
call is a new paid request: a new `requestId`, asked first like any round.

## Portraits

`generate_portraits` takes a `description` and returns up to two
`portraits`, one from Gemini and one from Grok. Each has an `upload` (`upload:<uploadId>`), a
short-lived `url` to the full-size image and its `provider`. Each portrait's
small preview arrives as an image after the JSON, labelled
`Portrait N (upload:<uploadId>):`; match each image to its portrait by that
upload, not by position. It is for invented characters only.

**Writing the description:** step 1 has no character yet, so ask one style
question first and build the description from the answer and the name. Put
the free-text hint in the question, not in an option: "What should <name>
look like? Pick a style, or type age, gender, look and style" with the styles
as options (SKILL.md, How to ask the creator).

Don't ask about framing and don't add one of your own: the server frames a
v2 portrait half-body and a v1 portrait from the shoulders up when the
description sets no framing (The engine a portrait is for, above). When the
creator asks for another framing, a close-up or a full-body shot, or for
gestures or movement, write it into the description; a v2 avatar moves what
the portrait shows.

- Be specific: age, gender, expression, clothing, style, and the framing
  when the creator gave one.
- Keep the face front-facing with the lips slightly parted, and the whole
  figure inside the avatar's shape: the engine keeps only the centre crop of
  its frame, a square unless the creator chose a portrait or landscape
  resolution (the candidates reference, Create). In a full body shot the
  face gets small, so check each option against the portrait checks'
  fifth-of-the-crop rule.
- Photorealistic, half-body: "A professional woman in her 30s, navy blazer,
  warm smile, lips slightly parted, half-body from the waist up, hands
  visible, neutral background."
- Stylised, full body: "A friendly cartoon character with big eyes,
  colourful outfit, cheerful expression, standing, full body, facing the
  camera."
- Too vague: "a friendly avatar". Ask the creator what style they want first.

**Judging:** save every option from its `url` with `scripts/media.sh save`
(kind `faces`) and show them with `scripts/media.sh show`, then run the
portrait checks (the portrait-checks reference) on each, generated or
enhanced. Recommend one with reasons, and let the creator pick.

**One change to a chosen portrait.** When the creator likes a generated
portrait but wants one thing different ("lighter background", "close the
collar"), generate again with that change written into the same description.
It is a new paid call, asked first. Keep the earlier options beside the new
ones, and let the creator pick which to use.

## Enhancing or restyling the creator's photo

`enhance_portrait` takes `image`, the creator's photo as it is, and returns
up to two square versions the same way as portraits. If the photo shows a
real person other than the creator, rule 4 applies first.

- **Without `prompt`** it enhances: use it when a real photo is close but
  flawed (poor lighting, odd crop, low resolution).
- **With `prompt`** it restyles: the same person in a new look, scene or
  style ("as a 1920s detective, film noir lighting", "in a chef's whites in
  a bright kitchen", "as a watercolour painting"). Offer it when the creator
  has a photo and wants a different look for that person, instead of
  describing a stranger with `generate_portraits`. The person is kept unless
  the prompt asks otherwise, so write any change to the person itself (age,
  hair, a cartoon animal) into the prompt on purpose. Up to 2,000
  characters. A blank prompt only enhances.

A restyle costs the same as an enhancement and is moderated like any input:
on `moderation_flagged`, rephrase the prompt with the creator and use a new
`requestId`. A changed prompt always needs a new `requestId`.

Pass the photo as `upload:<uploadId>`: upload the `sources/` copy with
`create_upload` (kind `portrait`, SKILL.md, Files and uploads) just before the
call, or pass a public https link the creator gave. Enhancing doesn't use the
upload up, so if the creator picks the original after all, pass the same
`upload:<uploadId>` to the create, within its hour.

**Is it still the same person?** Before recommending an enhanced or
restyled portrait, compare it with the original: face shape, features, skin
tone, apparent age and hair. Ignore what the prompt asked to change (a
restyle that was asked to age them is supposed to look older). If anything
else changed enough that someone who knows them would notice, say so plainly
("this one doesn't look like the same person: the jaw and eyes changed") and
don't recommend it. Offer to try again or to use the original.

A restyle may also leave the face turned away, covered or too small, or no
longer a face at all; run the portrait checks on each option as for any
portrait, and say which ones won't talk well.

## Voices

`design_voice` takes `voiceDescription` (20 to 1,000 characters), the
avatar's `language` and its name (`avatarName`), and returns `voices`. Step 1
has no character yet, so ask the creator how the voice should sound (age,
warmth, pace) and build the description from that.

**Writing the description:** "<Gender>, <age>. Persona: <2–5 words>.
Emotion: <2–3 adjectives>. <1–2 sentences about timbre, pacing, delivery>."
For example: "Female, 30–35. Persona: warm friendly guide. Emotion:
welcoming, cheerful. Smooth, natural timbre with gentle intonation and a
moderate pace." Ask for an expressive, energetic delivery: it animates better.

**Writing the sample text (`sampleText`):** three or four sentences the
avatar would say to a visitor, in its language, drawn from what the creator
has said so far, so the creator hears a person rather than a generic
greeting. Keep it within these lengths (220–600 characters for Latin and
Cyrillic scripts, 120–300 for Chinese, Japanese and Korean, 180–450 for
Hindi): that is what makes the sample run 10–60 seconds. No real person's
name. Without `sampleText`, the server writes the sample itself.

**Judging:** you can't hear audio. Save each preview from its `url` with
`scripts/media.sh save` (kind `voices`) and show it with `scripts/media.sh
show`, so the creator can play it. Each preview comes with:
- `durationSeconds`: every preview the server returns runs 10–60 seconds;
  `null` means the provider didn't report the length
- `spokenText`, the text it actually speaks, and its `provider`
- a `url` the creator plays

Say plainly that the creator judges how it sounds, then ask them to listen
and choose. Mention a preview whose length is unknown as such. If none
suits the creator, offer one new design at the stated price, with a changed
description or `sampleText`: a "slower delivery" in the description doesn't
make the sample longer.

## Clips

`generate_clip` renders an mp4 of 1 to 15 seconds (25 fps) of the person in
`image` moving as `prompt` says, with sound if audio is given. `image` and
`prompt` are required. No avatar is made: offer it only when the creator asks
for a clip (a teaser, a post), not as part of the five steps. Clips need
Atmee V2 (Atmee V2 comes with Growth and up; below Growth, an invite code unlocks it): when `get_balance`'s
`v2SpendableMicrocredits` doesn't cover the price the call answers
`insufficient_v2_credits`, so suggest Growth and up or ask for an invite
code (`redeem_code`); there is no v1 clip. Where billing still
decides clips by invitation and plan, an account no invite code unlocked gets
`v2_access_required` (ask for an invite code), and where `get_account`
`limits.videoGeneration` is false (absent counts as allowed) the call answers
`video_generation_not_in_plan`, so say the plan doesn't include video clips
and point to Studio → Billing (`links.billing`) to change plans; don't call
it. In a shell, `atmee generate clip` does the same (`--image`, `--prompt`,
`--audio`, `--duration`, `--resolution`, `--seed`), and uploads local files
first; it waits up to 10 minutes, and `atmee generate clip-status
REQUEST_ID --wait` follows a clip it stopped waiting for. Like the other `atmee generate`
commands it does not ask first, so confirm the price with the creator before
running it.

- **`image`:** a portrait as `upload:<uploadId>` (kind `portrait`; it stays
  usable afterwards, and a generated portrait works too) or a public https
  link. Rule 4 applies: never a real person other than the creator without
  their permission. The image and `prompt` (the shot and the motion, up to
  2,000 characters) are moderated.
- **`audio`** (optional; with sound if audio is given): a voice upload or
  https link. Then `durationSeconds` is required: set it to the audio's
  length; the server doesn't read it from the file. A silent clip defaults
  to 8.
- **Size:** `resolution` `{width, height}`, one of the clip sizes
  `get_account` lists in `pricing.generation.clipSizes`: 480×480 (the
  default), 640×480 and 480×640 (4:3), 832×468 and 468×832 (16:9), 720×720,
  960×720, 720×960, 1280×720, 720×1280. Match the shape the creator wants
  (square, standard or wide; tall for a phone). 1080p (1080×1080,
  1920×1080, 1080×1920, 1440×1080, 1080×1440) is coming soon, with no price
  yet: say so, and offer 720×720 (square) or 1280×720 / 720×1280 (wide or
  tall) instead. Any other size answers `invalid_input` with the list.
- **Length:** `durationSeconds` 1 to 15, at any size. It sets the price
  tier (up to 5, 10 or 15 seconds): a clip just over a tier pays the next
  one, so suggest 5 or 10 seconds when a little shorter would do. Pick the
  size and length before stating the price.
- **`seed`:** defaults to 0; another seed gives other motion, as a
  new request.
- **Price:** with or without sound, set by the size and the length: in
  that size's `clipPrices` `prices`, the entry with the smallest
  `upToSeconds` (5, 10 or 15) at or above `durationSeconds` (a 7-second
  clip pays the 10-second price; divide by 100 for credits). Pick the size
  and length, then ask with that price. Without `clipPrices` (an older
  server) a size costs its `clipSizes` `microcredits` for any length. Only on an older server
  without `clipSizes`: `clipPer15SecondsMicrocredits` pro
  rata to `durationSeconds`, rounded up (`ceil(durationSeconds × price /
  15)`), or `clipPerSecondMicrocredits` times `durationSeconds`. Only if
  `get_account` has none of these are clips not offered here: say so and
  stop.
- **Waiting:** a cold render takes minutes, and past about 45 seconds the call
  answers `request_in_progress` while the render goes on. Follow it with
  `get_clip` and the same `requestId`: it charges nothing, needs no
  `confirm`, and answers `rendering` (ask again after `retryAfterSeconds`),
  `ready` (`clip.url`, signed again on each read), `expired` (made, but its
  24 hours are over; not made again) or `stalled` (the call stopped: send
  the same `generate_clip` again, same `requestId` and inputs, which resumes
  it and charges once). `clip_not_found` means the call ended without a clip
  and charged nothing: the same `generate_clip` again says why. Where
  `get_clip` is not listed (an older server), wait `retryAfter` and send the
  same call again, as step 4 above says.
- **`invalid_input`** naming `input_failure image` (no face, unreadable) or
  `audio` needs another file; `input_failure resolution` ("the render pool
  does not serve WxH") needs another size, not another image.

The result's `clip.upload` lasts 24 hours and `clip.url` less: save the clip
at once with `scripts/media.sh save` (kind `seeds`, named `clip_1`, …) and
show it. You can't watch it, so the creator judges it.

A clip with no avatar has no folder, so save it in `atmee-avatars/clips`.
Before the call, make that folder and write the `requestId` and exact inputs
to `atmee-avatars/clips/pending.json`. Delete the file once the clip is saved,
the replay answers `unavailable`, or the call answers `moderation_flagged`,
`provider_failed`, `invalid_input`, `insufficient_credits`,
`api_key_spend_cap_reached` or `generation_unavailable`, after which a further
try is a new request. After a timeout or any other lost answer, keep the file. While the file exists, make no new clip: read that one with
`get_clip` (its `requestId`), or retry it with the same `requestId` and inputs.

Without a filesystem, keep the `requestId` and inputs in the chat summary
(`references/iterations.md`, Without a filesystem) and give the creator
`clip.url` to download.

## Using generated outputs

Pass the chosen output's `upload` as it is: a portrait as `assets.image.url`
to `create_avatar` or as `url` to `replace_appearance`, a voice as
`assets.voice.url` to `create_avatar` or as `url` to `replace_voice`. Never
download it and upload it again. Each works for one avatar and expires 24
hours after it was made: after that, or for a second avatar, generate again
(a new paid call, asked first). Its `url` expires sooner, so save the copy
you show as soon as the result comes back.
