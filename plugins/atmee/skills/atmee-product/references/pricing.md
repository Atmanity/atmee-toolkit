# How Atmee pricing works

## Contents

- Credits
- Plans and packs
- What costs credits
- Who pays
- Controlling spend
- Sponsoring
- Earnings

The structure below changes rarely. The numbers in it change often and are
never written here: read them from a live source (SKILL.md, Pricing) and say
the date you read them.

## Credits

- Everything is paid in credits. The API reports microcredits: divide by 100
  for credits.
- An account holds three buckets: **plan** credits (granted each billing
  cycle), **top-up** credits (bought as add-on credit packs) and **coupon**
  credits (promotions). `get_balance` shows each bucket, what running
  sessions hold, and the next expiry (`nextExpiry`, soonest first).
- Plan credits are spent first and expire at the end of each billing period.
  Pack credits never expire, even after a downgrade to the Free plan.
- `spendableMicrocredits` is what a new session can start on. It is the only
  balance to report; ignore any balance another tool returns.
- A session **reserves** credits when it starts and gives back the unused
  part when it ends (`billingMode: reserve`). Enterprise **metered** keys are
  billed for what they use, with no fixed session length; Atmee sets them up.

## Plans and packs

- A Free plan and paid monthly plans. A paid plan grants more credits each
  cycle and lifts the limits: longer sessions, more usage, more
  customisation. Names, prices and limits: Studio → Billing.
- For the account in hand, `get_account`: `plan` (its code and name, the
  credits it grants each cycle, when the cycle ends, a plan change scheduled
  for the cycle's end) and `limits` (longest session, sessions at once, how
  many API-created avatars, whether the API is on at all).
- Add-on credit packs top up without changing the plan. What a credit costs
  depends on how it was bought: plan credits come cheaper than packs (the
  public costs page has the figures).
- The Free plan works with the API, within the Free plan's own limits; pack
  and coupon credits are not held to its daily limit, so a top-up lifts it.
- **Signature avatars** on atmee.ai (the monetized ones) take a paid plan's
  credits, pack credits or coupon credits, never the Free plan's.

## What costs credits

| What | Charged | Live price |
|---|---|---|
| A conversation through the Session API | per minute, counted from the moment both the visitor and the avatar are in the room | `get_account` `pricing.sessionPerMinuteMicrocredits` |
| Rendering an avatar for the developer's own agent (the render path) | per minute while the render lasts | `pricing.renderV1PerMinuteMicrocredits`, `pricing.renderV2PerMinuteMicrocredits` |
| A seed (a v2 avatar's captured talking clip) | per seed (can be zero) | `pricing.previewClipMicrocredits` |
| Generating portraits, enhancing a portrait, designing a voice through the avatars server | per call, only when results come back | `get_account` `pricing.generation` |
| A speech session (speech-to-speech, no video) | per minute from the first usage report | `pricing.speechPerMinuteMicrocredits` |
| A clip (`generate_clip`, `atmee generate clip`, an mp4 of 1 to 15 seconds, with or without sound) | per second, only when it comes back; needs v2 access | `pricing.generation.clipPerSecondMicrocredits` (absent: clips aren't offered) |
| Generating a portrait in Studio | per generation | shown in Studio |
| A minute with a Signature (monetized) avatar on atmee.ai | per minute, at a higher rate; API sessions are never charged it | `pricing.monetizedPerMinuteMicrocredits` |

Nothing is charged for a session nobody joined (`no_user_joined`), a start
refused for capacity (`503 no_capacity`) or a v2 avatar not ready yet (`409
avatar_not_ready`). Selecting an older seed again captures nothing and
charges nothing.

To estimate what an app will cost: the account's per-minute price for its
path, times the minutes the developer expects, against the plan's limits
(session length and sessions at once bite before money does).

## Who pays

- Sessions started with an API key are billed to the key's account. The
  visitor needs no Atmee account and is never charged.
- On atmee.ai a signed-in visitor talks on their own credits, unless the
  avatar's owner sponsors them.

## Controlling spend

- **Per API key:** a spend cap per window (daily, monthly or total), a
  longest session, sessions at once, and optionally one avatar only. Keys
  are created in Studio → API or with `atmee keys create`; `get_api_key`
  shows the cap, what is spent and when the window resets. A start past the
  cap answers `402 api_key_spend_cap_reached`.
- **Per session:** `maxDurationSeconds`. The server grants the smallest of
  the request, the key's limit, the plan's limit and what the balance can pay
  for, and reports it as `actualDurationSeconds`.
- **Out of credits:** `402 insufficient_credits`. The owner tops up in Studio
  → Billing; never retry the start in a loop.

## Sponsoring

An avatar can have its own **sponsor pool**: credits moved from the account
into the avatar so that visitors talk on them instead of their own. Its
settings: the most one session may spend, what happens when the pool runs
out (pause, or spend the account's balance), automatic top-up, and whether
visitors with no Atmee account may talk (with a daily allowance per
visitor). Credits that no session holds can be withdrawn back.

A signed-in agent or user can sponsor directly, as can Studio → Billing or an
API key with `billing:write`. To let anyone talk free without an account:

1. Make the avatar public or unlisted (`update_avatar`, or `atmee avatars
   update ID --visibility unlisted`).
2. Fund it (`grant_sponsor`, or `atmee avatars sponsor grant ID MICROCREDITS`).
3. Turn on anonymous access (`update_sponsor`, or `atmee avatars sponsor
   update ID --anonymous --anonymous-daily-seconds N`).

The money tools (`grant_sponsor`, `update_sponsor`, `withdraw_sponsor`) need
`confirm: true`: ask the user first. Refusals name the next step:

- `avatar_not_budget_managed`: nothing was funded yet; grant credits or turn
  on auto top-up first.
- `avatar_not_shared`: the avatar is private; make it public or unlisted.
- `auto_topup_incomplete`: `autoTopupEnabled` needs a threshold and an amount;
  send both with it.
- `monetized_avatar_cannot_be_sponsored`: turn monetization off first, or
  leave it unsponsored.

## Earnings

Verified creators on atmee.ai turn avatars into Signature avatars and earn a
share of the credits visitors pay to talk to them, whether plan, pack or
coupon credits. The creator must be 18 or older, verified
(or hold documented authorisation for the person the avatar represents) and
selected by Atmee: `https://www.atmee.ai/creator-monetization-terms`.

- Earnings are switched on per avatar in Studio (the Finalize step), for a
  public or unlisted avatar that meets the eligibility rules Studio shows; a
  plan allows a limited number of earning avatars.
- Payouts go to the creator's Stripe Connect account, on request, above a
  minimum.
- Sponsored sessions and the owner's own sessions earn nothing, and a
  sponsored avatar can't also earn.
- Sessions started through an API key never earn: the account is talking to
  its own avatar.
