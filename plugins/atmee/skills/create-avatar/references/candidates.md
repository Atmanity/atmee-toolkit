# Create the avatar and check its seed

## Contents

- Create
- Waiting
- Check the seed
- When no seed is good
- Limits

## Create

Step 1 creates the avatar from its name, portrait, voice and language alone,
so the creator sees it talk before shaping its character. Create through the
avatars server with:

- **`engine`:** the one any paid portrait was made for (the
  credits-and-generation reference, The engine a portrait is for), normally
  `v2`, or `v2hd` when the creator chose Atmee V2 HD (Model, below). Use
  v1 only if the creator asks, or when the account
  can't build v2. Atmee V2 comes with Growth and up; below Growth, an invite code unlocks it. The
  account can build v2 when `get_balance` reports `v2SpendableMicrocredits`
  above 0; when `get_balance` leaves that field out (billing still decides v2
  by invite), when `get_account`'s `v2Access` is `granted`. A reported 0
  means no v2, whatever `v2Access` says. Otherwise (or when a
  v2 call answers `insufficient_v2_credits` or `v2_access_required`) ask the
  creator for an invite code and redeem it with `redeem_code`,
  or suggest Growth and up. Make v2 only once that holds; make v1 only if
  they have neither and agree.
- **`seedSelection`:** `auto`. The first seed that finishes goes live, so
  the avatar is ready without anyone choosing.
- **`seedPreviews`:** `{"count": 1}`.
- **name** and **language** from step 1. The language matters: with no
  preview line set, the seed speaks a default sentence in it.
- **`visibility`:** `private`. Create never makes an avatar public; unlisted is a
  change later (the grill's layer 3, or at hand-over).
- **no `previewMessage`, `persona` or `knowledge`.** The default sentence is
  the preview line, and the character comes in step 3 with `update_avatar`
  (Apply the character in the drafting reference). Never send
  `knowledge.facts`.
- **portrait and voice:** `upload:<uploadId>`, for the creator's own file
  (`SKILL.md`, Files and uploads) or a generated portrait or voice as it came
  back (the credits-and-generation reference), or a public https link the
  creator gave.
- **`resolution`:** leave it out (512×512, square) unless the creator
  wants a tall or wide video, then for example `{width: 384, height: 672}`
  (9:16) or `{width: 672, height: 384}` (16:9), if the server offers it
  (`get_account` `limits.v2Resolutions`; absent means every size the rule
  allows): a size it doesn't offer answers `invalid_input` listing the ones
  it does. The portrait is cropped to that shape at its centre, so
  check the face against it. A later change on `update_avatar` makes new
  seeds, like a new portrait. v1 refuses it: leave it out of a v1 create,
  and tell the creator the video stays square. A V2 HD avatar has its own
  sizes: leave it out for 640×640 (square), or pick one of
  `limits.v2ProResolutions`, the only list to trust (for example a tall
  `{width: 480, height: 864}` or a wide `{width: 864, height: 480}`, if it
  lists them); the v2 sizes (512×512, 672×384,
  384×672, 576×448, 448×576) are not V2 HD sizes.

### Model

There are three models. Name them Atmee V1, Atmee V2 and Atmee V2 HD to the
creator; the API's `engine` is `v1`, `v2` and `v2hd`.

- **Atmee V2** is the default, and what this skill makes.
- **Atmee V2 HD** is a larger V2 model on its own GPUs, at larger frames
  (640×640 by default; `limits.v2ProResolutions` lists the others). It is a V2 avatar in every
  other way: seeds, picking another, the character, the preview line and its
  links all work as this skill says; its conversations have their own per-minute rates
  (`get_account` `pricing.sessionV2HdPerMinuteMicrocredits`).
  Offer it when the creator wants a bigger video or asks for the best model.
- **Atmee V1** is the older model, for an account without Atmee V2.

Whether the account has V2 HD is in `get_account`: `limits.v2Pro: true`, on
an account that can build v2 (above), means yes. `limits.v2Pro: false` means
the plan lacks it (where billing still decides V2 HD by plan): mention it only
if the creator asks for it or for a bigger video, and point to Billing. No `limits.v2Pro` at all means this server
doesn't offer it: never mention V2 HD. Only when the account has it, add one
question to step 1's name and language call ("Which model?", with "Atmee V2
(Recommended)" and "Atmee V2 HD: a larger model, bigger video"), and save
the answer with the `create` iteration. Every later call on the avatar works
the same either way; `get_avatar` reads `engine: v2hd` for a V2 HD one.

The avatars server's create tool may default to manual selection or several
seeds: pass the selection and count above every time.

## Waiting

Building takes several minutes, and a silent wait looks stuck. Keep the
creator posted in plain words about the avatar, never about your tools:

1. Before the create (or a replace), say what is starting and roughly how
   long it takes: "Creating Manny now. It takes about 5 minutes; I'll tell you
   as each part finishes."
2. Follow it with the avatars server's `wait_for_avatar` on that avatar's id,
   with `timeoutSeconds` 25, and call it again while it hasn't settled. For a
   new batch on an avatar that already serves, follow `list_candidates` the
   same way.
3. After each call, say one short line when something changed: a stage
   finished, the avatar started making seeds, or another seed is done.
   Name the stage as the creator would: "Manny's voice is ready. Making his
   seed now." "Manny's seed is done." (for a batch: "2 of 3 seeds are done").
   When nothing changed for 2 minutes, say so in one line: "Still making
   Manny's seed, about 4 minutes in."
4. While the seed builds, step 3's conversation can start (SKILL.md, Step
   1 offers it). A question round holds your turn, so between rounds call `get_avatar`
   once, and as soon as the seed serves, show it before the next round: the
   seed comes first.

If it still hasn't settled after about 20 minutes, stop waiting: tell the
creator it's still building and that resuming later picks it up, and save the
iteration. If the creator leaves, the build carries on; resuming later finds
it (Resume in the iterations reference).

`wait_for_avatar` counts an avatar that is already serving as settled, so
after adding seeds to one (`add_candidates`), follow the new captures with
`list_candidates` until none is pending or processing.

**Never start a second build while one runs.** After a timeout or a network
error on create, replace or add-candidates, look before acting: list the
creator's avatars (or the avatar's seeds with `list_candidates`). If the build or capture is
there, wait on it. Only when it isn't there, try again, under rule 8's
two-failures limit. The server doesn't yet de-duplicate a repeated create, so
this check is what stops a duplicate avatar.

`wait_for_avatar` settles on one of these:

- **`ready`:** the first seed serves: check it (below).
- **`awaiting_selection`:** an avatar made with manual selection waits for a
  choice: judge its seeds and select on the creator's choice (below).
- **`failed`:** relay `latestFailure.userFeedback` as given and suggest the
  fix it names.
- **`stalled`:** waiting longer won't help. `stall.reason` says what to
  replace; guide that replacement (below).
- **`moderation_hold`:** a person has to review it; don't retry.
  `readyToChat` says whether the account can still chat with it meanwhile.

## Check the seed

Each seed is a short talking clip of the avatar saying the preview
message. Whether its lip sync and motion are good can't be predicted from
the inputs, so check the one that serves, and compare when there are more.

**From the contact sheet.** The server returns only preview links, so build
the sheet yourself with `scripts/contact-sheet.sh` (with a shell and
`ffmpeg` installed). Pass each seed's label and preview link in list
order, then read the image it writes:

- one row per completed seed, in list order
- twelve crops of the mouth area across each clip
- a `motion=<n>` number per row: the mean frame-to-frame change of the mouth
  crop, 0.0 for a mouth that never moves

To read it:

- A row at or near `motion=0.0`, whose lips never part, is a dead seed.
- Otherwise look at the crops to confirm the mouth clearly opens and closes.
  With several seeds, prefer the higher numbers; the number compares rows of
  the same avatar only: head movement and lighting raise it too, so it is
  not a threshold.

Tell the creator what you see.

**Show every seed**, with or without a contact sheet: in an agent with a
shell, save each completed seed with `scripts/media.sh save` (kind `seeds`)
and show it with `scripts/media.sh show` (SKILL.md, Showing results).
Preview links expire in about an hour, so save before showing. Without a shell, give the preview link.

**Without a contact sheet** (no `ffmpeg`, or no shell): say plainly
that you can't judge the lip sync from here, show the seed as above, and ask
whether it looks right. Don't invent a verdict.

**Another seed.** When the serving seed is dead, or the creator asks for
another, make one more (`add_candidates` with `count` 1, within the free
slots under Limits), check it the same way and show both. Adding seeds never
changes the serving one.

**Selecting.** Ask which to use at the end of the reply that shows the
seeds, in plain text with no question tool in that turn (SKILL.md, How to ask
the creator, the exception): your pick first, the serving one included, each
by its saved file ("seed_4ea8dc02.mp4 has the clearest mouth; say
seed_9b1c… otherwise"). Select only the seed the
creator chooses, or pick the best surviving one yourself if they ask you to.

## When no seed is good

Seeds are the lever, so tell the creator; otherwise they may conclude the
model doesn't work. Capture another (or up to 3 at once, within the free
slots under Limits) and judge again.

Diagnose the pair instead of capturing more when either holds:

- two batches in a row have no usable seed, or
- the server gives a warning that the pair is the problem.

Then try the levers in this order:

- **A more expressive preview line:** the line every seed speaks, a default
  sentence in the avatar's language until you set one. A livelier sentence
  often moves the lips more. Write it in the avatar's language, in 60 code
  points or fewer. Setting it replaces the default sentence and starts a new
  version, which captures new seeds; judge them the same way.

If that doesn't help, name the likely side:

- **Lips barely move in every seed:** usually a flat or quiet voice
  sample. Suggest a more expressive recording or a designed voice.
- **Mouth or face distorted, or motion odd:** usually the portrait (side
  view, hands near the face, no teeth showing). Suggest a front-facing photo
  with a slight smile, or an enhanced or generated portrait.

Guide the replacement through the avatars server's replace-portrait or
replace-voice. That captures new seeds; judge them the same way. Offer
generation here, and only here (the credits-and-generation reference).

## Limits

Captures count against one limit: 10 queued or running per account. Before
adding seeds, read `captureQuota` from `get_account`: the free slots are
`pendingLimit` minus `pending`. Ask only for what fits, at most 10 in one
batch; if none is free, say so and wait for running captures to finish. A
replace isn't counted in advance, since its capture count isn't published.

On `rate_limited`, wait the `Retry-After` the server gives, tell the creator
when they can try again, and offer to select one of the existing seeds
meanwhile. If adding seeds fails on the network, list them (`list_candidates`)
before adding again.
