# Changelog

## 0.10.12 (2026-11-04)


### Features

* **atmee:** describe Atmee V2 as paid by credits that work for it
* **atmee:** follow a video clip by its request id
* **atmee:** name Atmee V2 HD engine v2hd, keep v2pro working
* **atmee:** show clip prices by length
* **cli:** show Atmee V2 HD's own session rates


### Bug Fixes

* **atmee:** describe Atmee V2 by access, not by a kind of credit
* **atmee:** price portraits per call, up to two from Gemini and Grok


### Documentation

* **skills:** credit packs are the same on every plan, Free included

## 0.10.11 (2026-11-03)


### Features

* make portraits for the avatar's engine in the cli and skill
* price portraits per portrait and offer count in the skills
* resolution-only sizes and per-clip prices for video clips


### Bug Fixes

* show 1080p clips as coming soon


### Documentation

* **atmee-product:** pricing facts for the 2026 lineup
* use the atmee v2 size table in examples

## 0.10.10 (2026-11-03)


### Features

* allow v2 preview messages up to 300 characters
* aspect-ratio presets for v2 avatars and any-size clips


### Documentation

* **atmee-product:** /v1 in a path is the API version, not the engine
* **skills:** make sure agents know Atmee v2 makes video clips

## 0.10.9 (2026-11-02)


### Features

* follow the 2026 pricing fields in the cli and skills


### Documentation

* **plugin:** describe packs, earnings and clip prices as prod has them today

## 0.10.8 (2026-11-02)


### Features

* set a chat session's check-in from the cli
* start a chat session without the avatar's welcome
* update plugin pricing, sponsoring, clips and session skills

## 0.10.7 (2026-10-30)


### Features

* **cli:** redeem invite codes from the CLI, and let agents ask for one when v2 is refused

## 0.10.6 (2026-10-30)


### Features

* **cli:** handle invite-only v2 in the CLI, skills and docs

## 0.10.5 (2026-10-30)


### Features

* **cli:** open-source the CLI in toolkit, and point the public copy at production

## 0.10.4 (2026-10-30)


### Features

* publish each release to Atmanity/toolkit with the CLI binaries
* publish each release to Atmanity/toolkit with the CLI binaries


### Bug Fixes

* install the CLI through gh while toolkit is members-only

## 0.10.3 (2026-10-28)


### Features

* **create-avatar:** offer restyling the creator's photo with a prompt
* **create-avatar:** offer restyling the creator's photo with a prompt

## 0.10.2 (2026-10-28)


### Documentation

* **build-avatar-app:** file drops are proven live on staging
* **skills:** state the staging environment the app, CLI and tests must use

## 0.10.1 (2026-10-26)


### Features

* a build-avatar-app skill and a starter for apps around an avatar
* app tools in the CLI and the build-avatar-app skill
* create avatars without a portrait or voice, and edit their facts
* create v2 avatars with one take that goes live by itself
* **create-avatar:** add generated content and pose negative examples
* **create-avatar:** ask how much of the avatar to show; framing is never a defect
* **create-avatar:** build framing and quality negative examples from one invented face
* **create-avatar:** cap spending per avatar, keep the person in enhanced portraits
* **create-avatar:** check every portrait as it arrives; plugin 0.3.0
* **create-avatar:** describe only what the avatars server does
* **create-avatar:** five steps, review in chat or on the page
* **create-avatar:** make the final test a talk the creator holds
* **create-avatar:** measure a portrait before judging it by eye
* **create-avatar:** media.sh saves and opens review files without Node
* **create-avatar:** motion number, facts gate, confirm, one vocabulary
* **create-avatar:** one block per option, always with a file link
* **create-avatar:** open results for chat review and name the files in choices
* **create-avatar:** portrait checklist and example metadata for the agent
* **create-avatar:** show media to suit where the creator works
* **create-avatar:** show saved images inline in the reply
* **create-avatar:** start a pairing with 3 seeds, not 5
* **create-avatar:** upload the creator's own files to the avatars server
* **create-avatar:** wait with the server's tool, never duplicate a build, stop repeated failures
* Creator Studio parity in the CLI
* design portraits and voices through atmee-avatars alone (0.10.0)
* design portraits and voices through atmee-avatars alone (0.10.0)
* detect-host tells the guide and skills which agent runs them
* detect-host tells the guide and skills which agent runs them
* hand out the website links the Avatar API returns
* **plugin:** a plan-avatar-app skill that shapes an app idea into a brief
* **plugin:** an atmee-product skill so agents know what Atmee is and offers
* **plugin:** build Atmee avatars by talking to an agent, in Claude Code, Codex and Cursor
* **plugin:** install in Codex and Cursor from the same folder, one VERSION
* **plugin:** let a page drop files into the avatar call
* **plugin:** point everything at plugins/atmee and the atmee name; title the workbench atmee.ai / &lt;name&gt;
* **plugin:** point the avatars server at the release candidate; 0.4.0
* **plugin:** rename the plugin to atmee; title the workbench atmee.ai / &lt;name&gt;
* **plugin:** repair the workbench connection without the creator
* set a v2 avatar's runtime seed from the CLI and the create-avatar skill
* set a v2 avatar's runtime seed from the CLI and the create-avatar skill
* **setup:** offer to stop Claude Code's prompt before each atmee tool call
* **skills:** "Use when / NOT for" descriptions, and checks for links and prices
* **skills:** a workbench wrapper in each skill, and workbench doctor
* **skills:** apply Anthropic's skill authoring best practices
* **skills:** call the captured clip a seed everywhere
* **skills:** create-avatar runs the workbench and hears clicks in any agent
* **skills:** make the atmee skills installable with npx skills add
* **skills:** pick the account once for a multi-account sign-in
* **skills:** setup adds the atmee server in each agent, after a yes
* **skills:** work in Codex and Cursor without the workbench or bin/ on PATH
* wait for the creator's upload and ask before applying it
* **workbench:** + Add uploads the creator's own photo or voice on the page
* **workbench:** a five-step workbench that repairs its own connection
* **workbench:** a new pairing waits for Confirm; drop notes and the magic box
* **workbench:** a Talk click is the yes; controls on every call screen
* **workbench:** choose a seed from any version
* **workbench:** claim each click once and close the ones overtaken
* **workbench:** deliver clicks to any listening agent, nudge otherwise
* **workbench:** drop the Notes & learnings switch
* **workbench:** drop the talk command; pending declines old Talk clicks
* **workbench:** explain why to try more than one pairing
* **workbench:** fold takes into face-and-voice groups and five steps
* **workbench:** names, a credit pill and a complete history
* **workbench:** one detached helper per project that knows who listens
* **workbench:** package the workbench as @atmee/workbench
* **workbench:** pause and resume a voice from its play button
* **workbench:** persona versions kept on the creator's machine
* **workbench:** preview first, with the creator's step in the header
* **workbench:** retire the workbench call
* **workbench:** run detached, once per project, and stop when idle
* **workbench:** show and lock the pair Claude is making takes for
* **workbench:** show credits with atmee.ai's credit coin
* **workbench:** talk on a call page that mirrors atmee.ai's call screen
* **workbench:** talk to the avatar from the page
* **workbench:** the creator can remove a pairing
* **workbench:** the five-step page
* **workbench:** the page answers every press and keeps the creator's place
* **workbench:** wake Codex through codex queue
* **workbench:** warn about and block unusable faces and voices


### Bug Fixes

* **create-avatar:** actually play the first voice on macOS
* **create-avatar:** connect chat to the workbench, ask every question as a choice, report progress
* **create-avatar:** don't block on a question the workbench can answer
* **create-avatar:** fall back to a relative image path, not %20
* **create-avatar:** keep Git Bash from rewriting cmd.exe's /c
* **create-avatar:** no "I'll describe it" options; say what to type in the question
* **create-avatar:** no framing question; just don't limit the framing
* **create-avatar:** offer page choices both on the workbench and in chat
* **create-avatar:** one confirmation per paid generation, in chat
* **create-avatar:** one price question per round, no git offers, no passed checks in chat
* **create-avatar:** one price question per round, no git offers, no passed checks in chat
* **create-avatar:** open the first voice instead of playing it
* **create-avatar:** open the first voice instead of playing it, and sync the spec
* **create-avatar:** pass confirm: true after the creator's yes
* **create-avatar:** pass the avatar's language on every talk
* **create-avatar:** pre-approve the talk tools under every server name
* **create-avatar:** print the test link and questions before asking
* **create-avatar:** print the test link and questions before asking
* **create-avatar:** print the test link and questions; ask only what drifted
* **create-avatar:** read the balance only from the avatars server
* **create-avatar:** resume a named-but-undrafted avatar without the workbench
* **create-avatar:** show files and the test link as text, and really play the first voice
* **create-avatar:** show files and the test link as text, then ask in chat
* **create-avatar:** show portraits inline in Claude Desktop
* **create-avatar:** show portraits inline in Claude Desktop
* **create-avatar:** Windows playback and UTF-8 links in media show
* **plugin:** fix the server addresses so Claude Desktop can sign in
* **plugin:** fix the server addresses so Claude Desktop can sign in
* **review:** apply code review findings #1 and #2, and the agent-native gaps
* **review:** count lost paid calls, and keep Contents in step with headings
* **review:** keep a paid call's requestId safe across interruptions
* route motion complaints to the motion setting, in the skill's own words
* **setup:** drop the .gitignore question
* **skill:** fail the example-library check loudly, move rows N1-N4
* **skills:** act on the team's #api feedback on the creator flow
* **skills:** keep skill front matter valid YAML, bound the build wait
* **workbench:** a hang-up mid-join can't reconnect, a restore waits for its batch
* **workbench:** a spawn lock outlasts the longest start and its first write
* **workbench:** a talk in the history no longer gets the Talk panel's border
* **workbench:** Bugbot findings on PR 21
* **workbench:** hand over queued take requests, stop 0.7.x helpers, keep focus
* **workbench:** line the history timeline up with its header
* **workbench:** one set of coin gradients, and no Remove while a seed is being chosen
* **workbench:** one spinner per batch, step 2 done once takes are asked for, takes play in place
* **workbench:** page clicks need no second yes; spinner while the call starts
* **workbench:** Ready is ticked once reached, and Test points to the links
* **workbench:** redraw the page when the helper comes back
* **workbench:** review findings — lapsed Talk at hand-over, npm gate, call-page nudge
* **workbench:** show tries every file; the hook never adopts a parent folder
* **workbench:** tell a previewed seed apart from the chosen one


### Documentation

* chat-only, preview-first plugin at version 0.9.0
* **create-avatar:** open voice samples and name their files in choices too
* **create-avatar:** write the voice sample text from the draft, offer only full-length previews
* document --runtime-seed on update and in --config, offer it as a choice
* **evals:** record the first eval run
* install text matches 0.8.0
* install the skills with npx or the plugin, release the workbench to npm
* **install:** load the plugin with /reload-plugins instead of a restart
* **plugin:** install runbook for agents, scored rehearsals, creator scripts
* **rehearsals:** score N1 and N4 from a headless run
* settle the old workbench plan and name the talk's wallet
* **skills:** contents lists on long reference files, checked
* **skills:** one Current limits section in atmee-product and build-avatar-app
* **workbench:** five steps, one helper, self-repair; version 0.8.0
