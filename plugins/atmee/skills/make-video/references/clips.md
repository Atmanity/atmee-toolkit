# Clips

## Contents

- What a clip is
- Avatar clips
- Inputs
- Sizes and prices
- Price and balance
- Asking and sending
- Waiting and errors
- Planning a good clip
- A picture to start from
- Uploading the creator's files
- Saving and handing over
- Coaching a clip

## What a clip is

`generate_clip` renders an mp4 of 1 to 15 seconds (25 fps) that starts on
`image` and moves as `prompt` says, with sound if audio is given. `image` and
`prompt` are required for this picture clip; an avatar clip takes
`avatarId` and `transcript` instead (Avatar clips). Any picture works: a person, a drawn or anime
character, an animal, a product, a room or a landscape; nothing checks for a
face. The creator decides what to try. Give them the guidance below as tips
from real renders, not as limits, and never refuse an idea because it might
not work: say what tends to happen and let them choose. No avatar is made.

Clips are on every plan, Free included, and need no Atmee V2 access: make one
on any plan, without checking `v2Access`. Credits pay for it, at the price
per clip `get_account`'s `pricing.generation` lists. Where `get_account`
`limits.videoGeneration` is false (absent counts as allowed) the call
answers `video_generation_not_in_plan`: video generation was turned off for
this account, so say so and send the creator to contact us through the
Studio help center (the origin of `links.billing` plus
`/studio/help-center`); don't call it, and don't suggest changing plans.

In a shell, `atmee generate clip` does the same (`--image`, `--prompt`,
`--audio`, `--duration`, `--resolution`, `--seed`; `--avatar` and
`--transcript` for an avatar clip), and uploads local files first; it waits up to 10 minutes, and `atmee generate clip-status REQUEST_ID
--wait` follows a clip it stopped waiting for. Like the other `atmee
generate` commands it does not ask first, so confirm the price with the
creator before running it.

## Avatar clips

Send `avatarId` and `transcript` together, with an optional `prompt`,
`resolution` and `seed`. Never send `image`, `audio` or `durationSeconds`
with them: the call refuses them. The avatar's saved picture is the first
frame, its own Fish Audio voice speaks the transcript, and the lips follow.
V1 and V2 avatars both work.

- **When to offer it:** the creator asks for their avatar in a clip ("my
  avatar saying …"), or has saved avatars and wants a talking clip. It needs
  no picture and no recording, and the avatar looks and sounds the same in
  every clip: the way to keep one character across a series.
- **Finding the avatar:** `list_avatars` lists the creator's avatars by name
  with their `status` and `engine`. Offer the `ready` ones; a draft or a
  failed build may lack the picture or the voice. `get_avatar` reads one by
  id.
- **`transcript`:** the words the avatar says, 1 to 1,000 characters.
  Draft it with the creator. It is moderated with the prompt.
- **`prompt`** (optional): gestures, expression or a camera move, written as
  a shot list (Planning a good clip). Without it the avatar speaks to
  camera.
- **Length:** the speech plus 0.4 seconds of silence before and after,
  rounded up to whole seconds, at most 15. Speech past 14.2 seconds is cut,
  so keep each transcript to about 14 seconds of speech. The result's
  `clip.durationSeconds` is the actual length.
- **Size:** any clip size (Sizes and prices). The picture is cropped to the
  clip's shape, so match the avatar's own frame (`list_avatars` shows it): a
  v1 avatar is square.
- **Price:** a range, since the length is known only once the speech is
  made (Price and balance).
- **A longer script:** split it yourself at sentence breaks into pieces of
  about 14 seconds of speech each, one clip per piece, all at the same size
  and with the same avatar. Unlike a recording, text can be split here.
- **Errors:** `avatar_not_found` means the id is wrong or not this
  account's: list the avatars again. `invalid_input` naming `avatarId`
  (the avatar has no usable Fish Audio voice, or no readable picture) costs
  nothing: offer a picture clip instead, from the avatar's photo with a
  recording, or the `create-avatar` skill to give it a voice.
  `generation_unavailable` can also mean voice synthesis is off here; offer
  to try later.
- **API key:** with an API key, an avatar clip also needs `avatars:read`
  next to `generation:write`.

## Inputs

These are a picture clip's inputs; an avatar clip takes only `prompt`,
`resolution` and `seed` of them.


- **`image`:** any picture as `upload:<uploadId>` (create it with kind
  `portrait`, the name of the image kind; any subject is fine, and it stays
  usable afterwards) or a public https link; a generated portrait or an
  enhanced photo works too. Never a real person other than the creator
  without that person's permission (SKILL.md rule 4). The image and `prompt`
  are moderated.
- **`prompt` is limited in tokens, not characters:** the model reads at
  most 1,024 tokens of it (its own tokenizer's count, which you can't run
  here). Characters are only an estimate: about 5,000 of plain English,
  about 1,400 of Chinese, fewer for text full of symbols, numbers or URLs.
  A longer prompt answers `invalid_input` naming the prompt and costs
  nothing: shorten it and send it as a new request. 8,000 characters is a
  hard cap on top (an older server stops at 2,000 characters).
- **`audio`** (optional): a voice upload or https link, any length up to
  the clip's (the 10-second minimum of an avatar's voice sample doesn't
  apply). It is what is said on screen: the words and the voice are kept as
  recorded, and the clip moves the speaker's lips and face to them. Then
  `durationSeconds` is required, in whole seconds; the server doesn't read
  it from the file: set it to the audio's length, rounded up (4.5 s: 5), at
  most 15; longer audio is cut at the end.
  A silent clip carries a silent audio track.
- **Size:** `resolution` `{width, height}`, one of the clip sizes
  `get_account` lists in `pricing.generation.clipSizes`: 480×480 (the
  default), 640×480 and 480×640 (4:3), 832×468 and 468×832 (16:9), 720×720,
  960×720, 720×960, 1280×720, 720×1280. Match the shape the creator wants
  (square, standard or wide; tall for a phone). 1080p (1080×1080,
  1920×1080, 1080×1920, 1440×1080, 1080×1440) is coming soon, with no price
  yet: say so, and offer 720×720 (square) or 1280×720 / 720×1280 (wide or
  tall) instead. Any other size answers `invalid_input` with the list.
- **Length:** `durationSeconds` is any whole number of seconds from 1 to
  15, at any size. Ask the creator how long it should be (with audio, the
  audio's length decides) and always pass it: without it a silent clip runs
  8 seconds. The price comes in three tiers, up to 5, 10 or 15 seconds, and
  a clip pays the tier its length falls in (7 seconds pays the 10-second
  price), so when a length is just over a tier, say a little shorter costs
  less. Pick the size and length before stating the price.
- **`seed`:** defaults to 0; another seed gives other motion, as a
  new request.

## Sizes and prices

Read both from `get_account` before planning, never from memory: they
differ by server and change over time. `pricing.generation.clipPrices` lists
each size served here (`width`, `height`) with its `prices`, shortest
first: `upToSeconds` 5, 10 and 15, each with `microcredits` (divide by 100
for credits). Group the sizes by shape when you offer them:

| Shape | Sizes |
|---|---|
| Square | 480×480, 720×720 |
| Standard (4:3), wide or tall | 640×480, 960×720; 480×640, 720×960 |
| Wide (16:9) or tall for a phone | 832×468, 1280×720; 468×832, 720×1280 |

Offer only the sizes `clipPrices` lists, and show each with its price for
the length the creator has in mind, for example "720×1280: <price> up to
10 seconds". A larger size costs more; 480×480 is the cheapest and good
for trying a look before the final size. In a shell, `atmee account` prints
the same table.

## Price and balance

A clip costs the same with or without sound; its price is set by the size
and the length: in that size's `pricing.generation.clipPrices` `prices`, the
entry with the smallest `upToSeconds` (5, 10 or 15) at or above
`durationSeconds` (a 7-second clip pays the 10-second price). Divide by 100
for credits; never assume 1. Pick the size and length, then ask with that
price.

- Without `clipPrices` (an older server) a size costs its `clipSizes`
  `microcredits` for any length.
- Only on an older server without `clipSizes`:
  `clipPer15SecondsMicrocredits` pro rata to `durationSeconds`, rounded up
  (`ceil(durationSeconds × price / 15)`), or `clipPerSecondMicrocredits`
  times `durationSeconds`.
- Only if `get_account` has none of these are clips not offered here: say
  so and stop.

An avatar clip is charged at the tier its actual length falls in, but its
length is known only once the speech is made, and the balance must cover
the size's 15-second price before that. So quote its size's range, from the
up-to-5-seconds price to the 15-second price ("<price> to <price>, by its
length"), and check the balance against the 15-second price: below it, say
so before asking, as the call would answer `insufficient_credits`. After the
result, say its actual length and `microcreditsCharged`.

A paid plan's free generation allowance (`pricing.generation.freeAllowance`)
never pays for clips: credits do (the balance: SKILL.md rule 6). A result's
`microcreditsCharged` is what credits paid.

A picture made first is a paid call of its own: `pricing.generation` has
`portraitsMicrocredits` and `enhancementsMicrocredits`, each the price of one
call (a portrait call costs the same whether one portrait comes back or
two). The free generation allowance's `remainingMicrocredits` (what is left
this cycle) pays for portraits and enhancements before credits do: quote
what the call costs in credits after it (never below 0) and say the free
part ("covered by your plan's free generation"). A result's
`microcreditsFree` is what the allowance paid.

## Asking and sending

1. Ask once for the round: every paid call it needs (a picture and the clip,
   for example), each with its price, then the total and the balance: "Make
   a 10-second 720×1280 clip (<price>)? You have <balance> left." Wait for a
   clear yes in the creator's next message; it covers only the calls you
   named. Never add a call to it after the yes.
2. For each call, make up a new `requestId` (a UUID).
3. Every paid call, a picture as much as the clip, is tracked the same way.
   Before the call, write the tool (`generate_clip`, `generate_portraits` or
   `enhance_portrait`), the `requestId` and the exact inputs (for an avatar
   clip, `avatarId` and `transcript` with the rest) to
   `atmee-avatars/clips/pending.json`. Then send the call with that
   `requestId` and `confirm: true`. The server refuses the call without
   either, so `confirm: true` goes only after the yes, never before.
4. If the call times out or returns `request_in_progress` or `rate_limited`,
   wait the seconds it gives (`retryAfter`, or "retry in N s" in its
   message), then follow a clip with `get_clip` (Waiting and errors), or
   send the **same** call again (the only way for a picture): the same `requestId`, the same inputs and `confirm:
   true`. Don't ask again: the same id never charges twice, and a call still
   running answers with its one result once it finishes. Tell the creator in
   one line that it is still being made. A wait of minutes means the charge
   is still being confirmed; `pending.json` keeps the id safe if the turn
   ends first.
5. Delete `pending.json` once the result is saved (the clip, or every
   picture option), the replay answers
   `unavailable`, or the call answers `moderation_flagged`,
   `provider_failed`, `invalid_input`, `insufficient_credits`,
   `api_key_spend_cap_reached` or `generation_unavailable`, after which a
   further try is a new request. After a timeout or any other lost answer,
   keep the file. While the file exists, make no new paid call: read a clip
   with `get_clip` (its `requestId`), or retry the call with the same tool,
   `requestId` and
   inputs, without asking the creator again.

Reuse a `requestId` only for a retry of identical inputs; any change is a new
request with a new id. A result with `replayed: true` is an earlier result
returned again, charged nothing this time (`microcreditsCharged` 0): the
first call paid. Before using one of its outputs, check `unavailable`: an
unavailable output was already used or has expired and can't be used.

Without a filesystem, keep the `requestId` and inputs in a short summary in
chat before the call, retry a pending one from an earlier summary the same
way before any new clip, and give the creator `clip.url` to download.

## Waiting and errors

- **Waiting:** a cold render takes minutes, and past about 45 seconds the
  call answers `request_in_progress` while the render goes on. Follow it
  with `get_clip` and the same `requestId`: it charges nothing, needs no
  `confirm`, and answers `rendering` (ask again after `retryAfterSeconds`),
  `ready` (`clip.url`, signed again on each read), `expired` (made, but its
  30 days are over or it was deleted; not made again) or `stalled` (the call stopped: send
  the same `generate_clip` again, same `requestId` and inputs, which resumes
  it and charges once). `clip_not_found` means the call ended without a clip
  and charged nothing: the same `generate_clip` again says why. Where
  `get_clip` is not listed (an older server), wait `retryAfter` and send the
  same call again, as step 4 of Asking and sending says.
- **`invalid_input`** naming `input_failure image` (the file can't be read)
  or `audio` needs another file; `input_failure resolution` ("the render
  pool does not serve WxH") needs another size, not another image;
  `input_failure prompt` needs a shorter prompt.
- **One clip at a time:** a few calls in parallel answer `429
  rate_limited`. Send clips one after another.
- **Other codes** (`insufficient_credits`, `api_key_spend_cap_reached`,
  `moderation_flagged`, …): SKILL.md rule 7.

## Planning a good clip

Plan the clip with the creator before the price question: what they want to
see, then the tips that help it come out well. These come from real renders;
the model often does more than they say, so when the creator wants to try
something unusual, try it.

- **Any subject, any motion.** People, drawn and anime characters, animals,
  objects and whole scenes all animate. Big motion works: a person stands up
  and walks across the room, a dog runs at the camera, a new person walks
  into the shot. Whatever the picture didn't show (a back, legs, the rest of
  a room) is invented, so the look can drift as the subject moves away from
  the picture; that is a trade-off to mention, not a reason to hold back.
- **Write the prompt as a shot list.** The camera, then concrete actions in
  order: "medium shot, static camera; she looks up and smiles, then points
  at the chart, then laughs and takes a sip of water". Each action is
  followed in turn, and motion settles after the last one, so give enough
  actions for the whole length; a 15-second clip with five actions stays
  busy to the end. "She talks to the camera" alone barely moves.
- **For speech, give a recording.** Speech drives the lips and face of a
  person or a drawn character, in any language, and adds nods and gestures;
  silent, the mouth stays mostly still. Music, rain and other sound play
  back but don't move the lips. If the creator has only a script, they can
  record it (a phone is fine; the voice is kept as recorded). Music,
  ambience and a voice-over can also go on in an editor afterwards.
- **Lively, clear speech moves the face most;** quiet or flat audio less.
- **The picture is the first frame.** The clip starts on it exactly and is
  cropped to the clip's shape (a square picture in 720×1280 loses its
  sides), so a picture already in the clip's shape keeps everything. Only
  one image is taken: to bring in another look, outfit or setting, restyle
  or make the picture first (A picture to start from), let the creator
  pick, then animate it.
- **Landscapes and scenes with no one in them** animate well when the
  prompt says what moves: falling water, breaking waves, drifting mist,
  rolling clouds, changing light. A time-lapse works. Weather is partial
  (asked for snow, a scene can turn snowy without visible flakes).
- **Some things come out less reliably:** a product spinning on a
  turntable (the object tends to morph), exact liquids or colours (poured
  water may change colour), and camera moves that carry the shot (a drone
  flight or a tilt can barely move, an orbit can jump, and a camera path in
  several stages may come back as cuts between shots). Worth trying when
  the creator wants them; set expectations and suggest a second seed.
- **Renders vary a lot by `seed`.** The same inputs gave one clip that
  talks well and one that barely moves. When a clip matters, say a second
  render may help and price it too. A big fixed smile in the picture can
  also stay fixed while it talks.
- **A longer video is several clips,** joined in an editor: up to 15
  seconds each, all at the same size. With a saved avatar, avatar clips keep
  the look and the voice the same, and you split the script yourself
  (Avatar clips). Otherwise keep the characters the same by
  starting every clip from the same picture, or from pictures made from it
  (`enhance_portrait` with a prompt for a new pose or setting), never from a
  new description each time. A voice-over longer than 15 seconds needs one
  recording per clip, each at most 15 seconds and ending at a sentence
  break: you can't cut audio here, so ask the creator to record the pieces
  separately (or split the file in an audio editor) and upload each one.
  Set each clip's `durationSeconds` to its piece's length rounded up: audio
  longer than the clip is cut, so a whole voice-over in one clip loses its
  end. This skill makes one clip at a time: make each piece as its own clip.
- **Size and time:** a warm 480×480 clip of 5 to 8 seconds came back in
  about 15 to 30 seconds; 720×1280 took about 80 to 110 seconds. Trying the
  look at 480×480 (cheapest) and making the final at the larger size is a
  good way to explore.

## A picture to start from

When the creator has no picture, or wants another look for the one they
have, make it first, as a paid call asked like the clip (Asking and
sending). Never generate a real person's likeness from a description.

- **`generate_portraits`** takes a `description` and returns up to two
  `portraits`, one from Gemini and one from Grok. Each has an `upload`
  (`upload:<uploadId>`), a `url` to the full-size image that works until
  `urlExpiresAt`, and its `provider`. Each portrait's small preview arrives as an image after the
  JSON, labelled `Portrait N (upload:<uploadId>):`; match each image to its
  portrait by that upload, not by position. It draws people, characters,
  animals, scenes and products. Be specific: age, gender, expression,
  clothing, style and framing for a person; for a talking subject, keep the
  face front-facing with the lips slightly parted. "A friendly avatar" is
  too vague: ask what style they want first.
- **`enhance_portrait`** takes `image`, the creator's photo as it is, and
  returns up to two square versions the same way. Without `prompt` it
  enhances a real photo that is close but flawed (poor lighting, odd crop,
  low resolution). With `prompt` it restyles: the same person in a new
  look, scene or style ("as a 1920s detective, film noir lighting"). The
  person is kept unless the prompt asks otherwise, so write any change to
  the person itself into the prompt on purpose. Up to 2,000 characters; a
  blank prompt only enhances. A restyle costs the same as an enhancement and
  is moderated like any input: on `moderation_flagged`, rephrase the prompt
  with the creator and use a new `requestId`. Pass the photo as
  `upload:<uploadId>` (Uploading the creator's files) or a public https link
  the creator gave.
- **The engine changes the picture.** A portrait is made for an avatar
  engine. For v1, enhancing turns the face to the camera, frames it from the
  shoulders up and blurs the background, and a generated portrait is framed
  from the shoulders up; left out, the server makes a v1 portrait. For v2,
  enhancing keeps the photo's pose, gaze, hands, framing and background and
  only improves its quality, and a generated portrait is framed half-body,
  unless the description sets another framing. A clip wants the v2 picture
  (the pose, the setting, the hands), so always pass `engine: v2` for a clip
  picture, on every plan: portraits need no Atmee V2 access, only avatars
  do.
- **Is it still the same person?** Before recommending an enhanced or
  restyled picture, compare it with the original: face shape, features,
  skin tone, apparent age and hair. Ignore what the prompt asked to change.
  If anything else changed enough that someone who knows them would notice,
  say so plainly and don't recommend it.

Save every option from its `url` with `scripts/media.sh save` (kind
`faces`, in `atmee-avatars/clips`, named `face_1`, …), delete
`pending.json`, show them with `scripts/media.sh show`, then let the creator
pick. Pass the chosen output's
`upload` as it is as the clip's `image`. Never download it and upload it
again. It expires 7 days after it was made, and its `url` sooner, at `urlExpiresAt`.

## Uploading the creator's files

The creator's own picture and recording go to the server as uploads.

- **Save it first.** In an agent with a shell, tell the creator they can
  paste an image or drag a file (a photo, a recording) straight into the
  chat; you receive its path. Never ask them to move files or type paths.
  Copy it into `atmee-avatars/clips/sources/` yourself.
- **Upload it just before the call that uses it,** never earlier: an upload
  expires after an hour. Call `create_upload` with `kind` (`portrait` for a
  picture, `voice` for a recording), `fileName` and `contentType`. Declare
  the type `upload-file.sh` sends for the extension, because the upload
  takes only the declared one: its usual image type, `audio/mpeg` for mp3,
  `audio/mp4` for m4a, `audio/opus` for opus.
- **With a shell,** from the project folder run this skill's
  `scripts/upload-file.sh` by its full path with the `uploadUrl` and the
  copy in `sources/`. Once `get_upload` reads `uploaded`, pass
  `upload:<uploadId>`.
- **Without one** (claude.ai, cloud sessions), give the creator the
  `uploadPageUrl` in a message: they choose the file there, or record a
  voice in the browser. The link works for 15 minutes; make a new upload if
  it lapses. In the same turn, right after that message, call
  `wait_for_upload`, and call it again while `status` stays `pending`, so
  the creator doesn't have to say it's done. If they seem away, end the
  turn and ask them to say when it's uploaded. A server without
  `wait_for_upload` takes `get_upload` instead.
- **When the file arrives from the page,** tell the creator it's uploaded
  and ask whether to use it for the clip now. Only after they agree, pass
  `upload:<uploadId>`. With a shell the creator already handed you the file,
  so don't ask again.
- Never write `uploadUrl` or `uploadPageUrl` to any file.

## Saving and handing over

The result's `clip.upload` lasts 30 days and `clip.url` until `urlExpiresAt`: save the clip
at once with `scripts/media.sh save` (kind `seeds`, in `atmee-avatars/clips`,
named `clip_1`, `clip_2`, …) and show it with `scripts/media.sh show`. You
can't watch it, so the creator judges it. When they say it looks off, coach
them (Coaching a clip). If `media.sh save` refuses or fails on a link, give
`clip.url` itself and say it works until `urlExpiresAt`. A clip link that stops working (a page
saying `InvalidJWT` or `"exp" claim timestamp check failed`) has only
expired: read the clip again with `get_clip` (free) for a new link while the
clip lasts, and save it at once.

When the creator wants a clip (kept 30 days) or a picture (kept 7) gone sooner,
confirm once (it can't be undone, and nothing is refunded), then call
`delete_output` with its `upload:<uploadId>` and `confirm: true`: the file
is deleted at once and its link stops working. Never delete one unasked.

When the creator asks for a clip made earlier (in another chat, by a team
member or through an API key), call `list_generations` (`kind: clip`; it is
free): each item has its `requestId`, who made it, its prompt or script and,
while it lasts, a fresh `url`. An expired one keeps its `inputs`, so it can
be made again with a new `requestId` (when `remake` says an image or audio
upload is required, ask the creator for the file again). `delete_generation`
with the `requestId` deletes all of a generation's outputs, with the same
confirmation as `delete_output`.

## Coaching a clip

A clip from `generate_clip` is one render of the picture, prompt and audio,
on Atmee V2. When the creator says it looks off (too still, lips not moving
while it speaks, a warped subject, odd motion), offer fixes as options; the
creator keeps whatever they like, however unusual.

- **Every try is a new paid clip** at the clip's price for its size and
  length, asked first (SKILL.md rule 1). Say so in each choice; there is no
  free re-render.
- **Another render:** the same inputs with another `seed` give other motion,
  as a new request. Renders vary a lot: from the same inputs one clip
  talked well and one barely moved.
- **Too still, or the motion stops early:** add concrete actions in order
  for the whole length ("waves, then picks up the cup, then laughs"); the
  model settles after the last action it was given.
- **The audio:** for a talking clip, a speech recording makes the
  mouth move; silent, it stays mostly still.
  Otherwise a livelier recording or a different voice; flat or quiet audio,
  and non-speech audio such as rain or music, move the lips less or not at
  all.
- **The subject changes as it moves away:** parts the picture never showed
  are invented. A picture that already shows more (a wider shot, the
  outfit, the place) keeps it closer; or keep it and enjoy the surprise.
- **The picture:** a picture in the clip's shape isn't cropped, and a
  retaken, cropped or restyled one is free or cheap before the new render.
  For a talking person: chest-up or half body works best; keep the face
  turned to the camera and the lips visible, not closed, covered or behind a
  filter (a slight smile with some teeth showing works well; avoid a wide
  grin or an open mouth); even, soft light on the face. A big fixed smile
  can stay fixed.
- **Last,** a different picture altogether.
