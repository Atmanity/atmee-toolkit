---
name: make-video
description: Make a short Atmee video clip (1 to 15 s) from a picture, or of a saved avatar speaking a script. Use when someone wants a clip, teaser or post. NOT for building avatars or editing.
license: Proprietary
metadata:
  owner: Atmanity
allowed-tools: Bash(*/scripts/media.sh *), Bash(*/scripts/update-check*), Bash(*/scripts/detect-host*), mcp__plugin_atmee_atmee-avatars__get_api_key, mcp__plugin_atmee_atmee-avatars__get_account, mcp__plugin_atmee_atmee-avatars__get_balance, mcp__plugin_atmee_atmee-avatars__list_avatars, mcp__plugin_atmee_atmee-avatars__get_avatar, mcp__plugin_atmee_atmee-avatars__create_upload, mcp__plugin_atmee_atmee-avatars__get_upload, mcp__plugin_atmee_atmee-avatars__wait_for_upload, mcp__plugin_atmee_atmee-avatars__generate_portraits, mcp__plugin_atmee_atmee-avatars__enhance_portrait, mcp__plugin_atmee_atmee-avatars__generate_clip, mcp__plugin_atmee_atmee-avatars__get_clip, mcp__plugin_atmee_atmee-avatars__delete_output, mcp__atmee__get_api_key, mcp__atmee__get_account, mcp__atmee__get_balance, mcp__atmee__list_avatars, mcp__atmee__get_avatar, mcp__atmee__create_upload, mcp__atmee__get_upload, mcp__atmee__wait_for_upload, mcp__atmee__generate_portraits, mcp__atmee__enhance_portrait, mcp__atmee__generate_clip, mcp__atmee__get_clip, mcp__atmee__delete_output
---

# Make an Atmee video

You help a creator make one short video clip, an mp4 of 1 to 15 seconds,
in one of two ways:

- **A picture clip** starts on a picture they choose and moves as a prompt
  says, silent or speaking a recording they give.
- **An avatar clip** stars one of their saved avatars: its look is the first
  frame and its own voice speaks a script you write with them. No picture or
  recording is needed, and the avatar looks and sounds the same in every
  clip.

One `generate_clip` call makes one clip. No avatar is made. The clip
reference (`references/clips.md`) has every input, the price lookup, the
waiting and the tips from real renders; read it before planning.

A creator who wants to build or change a talking avatar, not a video, needs
the `create-avatar` skill instead.

## How to ask the creator

**Every question to the creator goes through your question tool**
(`AskUserQuestion` in Claude Code), with one exception: when you show
pictures or the finished clip, end your turn with them as your reply's text
and a short question in plain text, and call no question tool in that turn.
Your thinking is not your reply.

- One call per round: up to 4 questions that don't depend on each other, each
  with 2–4 short options. Your recommended option comes first, marked
  "(Recommended)".
- **Free text** (what should happen in the clip): offer your best guesses as
  options and say in the question what to type instead. Never add an option
  that only means "let me type".
- **Files:** "I'll drag them into the chat now (Recommended)" and "Go ahead
  without them". On the first, wait for the files.
- **Paid calls:** Yes and No, with the price and the balance in the question.
- **Never ask twice.** Drop every question the conversation or the creator's
  files already answer. If an answer is unclear, state how you read it in one
  line and go on.

Without a question tool (claude.ai, some clients), ask the same round in chat,
in this shape, so the creator can answer by number:

```
1. <question>
   ➡️ <your recommended answer>
2. <question>
   ➡️ <your recommended answer>
```

## Rules that always apply

1. **Ask once before each round of paid calls.** Where your agent honours
   this skill's pre-approved tools, the paid tools run without a permission
   prompt, so this question is the creator's only confirmation: never skip
   it. Read each price from `get_account` (`pricing.generation`) and the
   balance from `get_balance` (rule 6), then ask one question with every
   call's price, the total and what the creator has left. Wait for a clear
   yes, then pass `confirm: true` on each call in that round. Each new paid
   request gets a fresh `requestId` (a UUID you make up); a retry of the same
   request keeps its id and `confirm: true`, isn't asked again, and doesn't
   charge twice (`references/clips.md`, Asking and sending).
2. **One paid call at a time.** Never send calls in parallel, and make no
   new paid call (a clip or a picture) while
   `atmee-avatars/clips/pending.json` exists.
3. **A picture clip always takes `durationSeconds`:** the length the
   creator chose, any whole number of seconds from 1 to 15, or the
   recording's length rounded up. Without it a silent clip runs 8 seconds.
   An avatar clip takes `avatarId` and `transcript` and never `image`,
   `audio` or `durationSeconds`: its speech sets the length.
4. **Real people.** If the picture or the recording is of a real person other
   than the creator, confirm they have that person's permission before any
   upload or paid call, and never generate a real person's likeness from a
   description.
5. **Don't fake a judgment.** You can't watch the clip or hear audio: the
   creator judges it.
6. **The balance comes only from `get_balance`:** `spendableMicrocredits`
   divided by 100 is the credits left. After a paid call, read it again.
7. **Error codes decide retries:**
   - `insufficient_credits`: say so, give the top-up link its message carries
     (or the billing page on atmee.ai), and don't retry.
   - `api_key_spend_cap_reached`: the API key's spend cap is used up; say so
     and don't retry.
   - `video_generation_not_in_plan`: video generation was turned off for this
     account; send the creator to the Studio help center
     (`references/clips.md`, What a clip is). Don't suggest changing plans.
   - `moderation_flagged`: nothing was charged; offer to rephrase the prompt
     or use another picture.
   - `provider_failed`: nothing was charged; offer to try again (a new
     request).
   - `generation_unavailable`: generation is briefly unavailable or off;
     nothing was charged; offer to try again later.
   - `rate_limited` or `request_in_progress`: wait the time it gives
     (`retryAfter`), then follow the clip with `get_clip` or send the same
     call again, same `requestId` and `confirm: true`.
   - `invalid_input`: fix the input it names (a shorter prompt, another size,
     another file); don't repeat it unchanged. If it says the key was already
     used and charged and its result can't be recovered, tell the creator
     they were charged for a result that was lost, and ask before a new paid
     call.
   - `account_ambiguous`: run setup's account question, then call again with
     the `accountId`.

   **Two identical failures stop a retry:** the same tool with the same
   inputs failing with the same error twice. Change the input or ask the
   creator; never try a third time unchanged.
8. **Talk about the video, not your work.** Say what happens next for the
   creator in plain words ("Making your clip now; it can take a few
   minutes"). Keep tool names, ids and checks that passed out of the chat.
9. **Talk in the creator's language.**

## The server

One server does everything: `atmee-avatars` with the atmee plugin, `atmee`
without it. When `atmee-avatars/settings.json` has an `accountId`, pass it on
every call.

| Tool | Use |
|---|---|
| `get_account` | the plan, `limits.videoGeneration`, and the prices in `pricing.generation` (`clipPrices`, `clipSizes`) |
| `get_balance` | the credits left, the only source of them |
| `list_avatars`, `get_avatar` | the creator's saved avatars, for an avatar clip |
| `create_upload`, `get_upload`, `wait_for_upload` | the creator's own picture or recording |
| `generate_portraits` | a picture from a description, when the creator has none (paid) |
| `enhance_portrait` | a cleaned-up or restyled version of the creator's photo (paid) |
| `generate_clip` | the clip itself (paid, by size and length) |
| `get_clip` | a clip's status by its `requestId`, free: follow a render and get its download link |
| `delete_output` | deletes a clip or picture the creator no longer wants, only when they ask and confirm (it can't be undone or refunded) |

In claude.ai the server is a connector the creator adds; the atmee plugin
adds it for its agents; anywhere else setup adds it (`references/setup.md`).

## Flow

### Start

1. Run the setup in `references/setup.md` (the same steps as the
   `atmee-setup` skill): it adds the atmee server if it is missing, checks
   sign-in, the account and credits in one pass, then comes back here.
2. Make `atmee-avatars/clips/` (and `sources/` in it) if it is missing.
   Everything this skill saves goes there.
3. If `atmee-avatars/clips/pending.json` exists, finish that call first,
   before any other check: it was asked and may be paid already. Go by the
   tool it names, without asking again (`references/clips.md`, Asking and
   sending):
   - `generate_clip`: read it with `get_clip` by its `requestId` and follow
     its answer (`references/clips.md`, Waiting and errors). Only when it
     answers `stalled` or `clip_not_found`, or the server has no `get_clip`,
     send `generate_clip` again with the same `requestId`, inputs and
     `confirm: true`.
   - `generate_portraits` or `enhance_portrait`: send that same tool again
     with the same `requestId`, inputs and `confirm: true` (it has no
     status call); the same id never charges twice.
4. Read `get_account`. When `limits.videoGeneration` is false, say video
   generation was turned off for this account and stop
   (`references/clips.md`, What a clip is). When `pricing.generation` has no
   clip price, say clips aren't offered here and stop. Otherwise keep its
   size and price table for planning: the sizes served here and, for each,
   the price of a clip up to 5, 10 and 15 seconds (`references/clips.md`,
   Sizes and prices).
5. Call `list_avatars` and keep the creator's ready avatars (name and id):
   with one, planning offers an avatar clip even if they didn't mention it.
   When the creator named an avatar, search for it with `q`. Otherwise page
   on (pass `nextBefore` as `before`) until you have a ready avatar or the
   pages run out: the newest may all be drafts. No ready avatar on any
   page, or a key without `avatars:read`, means only clips from a picture.

### 1. Plan the clip

Ask what they want to see. When they ask for their avatar ("my avatar
saying …"), or when `list_avatars` shows they have one, offer an avatar clip
as well as a picture clip (`references/clips.md`, Avatar clips): no picture
or recording needed, the avatar's own look and voice, the same in every
clip. Then settle, in as few rounds as the answers allow.

For an avatar clip:

- **The avatar,** from `list_avatars` (offer its ready ones by name).
- **The script** it says, about 14 seconds of speech at most. Draft it with
  them.
- **The prompt,** optional: gestures or a camera move. Without it the
  avatar speaks to camera.
- **The size,** as for a picture clip below. The length follows the speech.

For a picture clip:

- **The picture:** theirs (`references/clips.md`, Uploading the creator's
  files), or one made first from a description or from their photo
  (`references/clips.md`, A picture to start from). It is the first frame.
- **The prompt,** written as a shot list: the camera, then concrete actions
  in order, enough for the whole length. Draft it for them and let them
  change it.
- **The sound:** silent, or a voice recording of what is said on screen.
  Speech moves the lips; music or rain play back but don't. If they have
  only a script, ask them to record it (a phone is fine).
- **The size,** from the shape they want: square, wide, or tall for a phone.
  Offer the sizes of that shape with their prices for the length they have
  in mind, cheapest first, so they choose knowing the cost.
- **The length:** any whole number of seconds from 1 to 15 (with a
  recording, its length decides). The price changes only at 5 and 10
  seconds: when a length is just over one, say a little shorter costs less.

Give the planning tips that fit their idea (`references/clips.md`, Planning a
good clip), as tips, not limits. Never refuse an idea because it might not
work: say what tends to happen and let them choose.

**A longer video is several clips,** made one at a time and joined in an
editor (`references/clips.md`, Planning a good clip). Plan and make the
first clip now; offer the next one after it.

### 2. Price and yes

Look up the clip's price for its size and length (`references/clips.md`,
Price and balance), read the balance, and ask once: "Make a 10-second
720×1280 clip with your recording (<price>)? You have <balance> left." with
Yes and No. A picture made first is in the same question.

An avatar clip's length is known only once the speech is made, so quote its
size's range, from the up-to-5-seconds price to the 15-second price, and say
the balance must cover the 15-second price first: "Make a 720×1280 clip of
Mira saying your script (<price> to <price>, by its length)? You have
<balance> left."

### 3. Render

Upload the creator's files just before the call. Write `pending.json`, then
call `generate_clip` with the `requestId`, every input you planned,
`durationSeconds` (a picture clip only) and `confirm: true`. Tell the creator it is being made and
may take a few minutes. When it answers `request_in_progress`, follow it with
`get_clip` until it is `ready` (`references/clips.md`, Waiting and errors).

### 4. Save and show

Save the clip at once with `scripts/media.sh save` (run by its full path
from the project folder; kind `seeds`, folder `atmee-avatars/clips`, named
`clip_1`, `clip_2`, …), delete `pending.json`, and run `scripts/media.sh
show` on the saved file. Put the printed block in your reply unchanged: it
has a `file://` link the creator clicks to open the clip. Without a shell,
give `clip.url` and say it works until `urlExpiresAt` (`references/clips.md`,
Saving and handing over). Read `get_balance` again
and say what's left.

### 5. Hand over

End with the clip's link, what it cost (`microcreditsCharged`; for an avatar
clip, its actual length and charge) and the balance, and ask in plain text
how it looks. Offer, in one line each:

- another render or a fix if it looks off (`references/clips.md`, Coaching a
  clip): each is a new paid clip, asked first;
- the next clip, when they are making a longer video.

Never make another clip unasked.

## References

| File | Read when |
|---|---|
| `references/clips.md` | before planning a clip, and for avatar clips, prices, waiting, errors, pictures, uploads and coaching |
| `references/setup.md` | at the start, or when a tool is missing or asks for sign-in |
