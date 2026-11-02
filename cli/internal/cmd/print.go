package cmd

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Atmanity/toolkit/cli/internal/api"
)

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

func engine(e any) string {
	s := fmt.Sprint(e)
	if s == "" || s == "<nil>" {
		return "v1"
	}
	return s
}

var stageOrder = []string{"appearance", "voice", "persona", "knowledge", "previews"}

func stageIcon(status string) string {
	switch status {
	case "completed":
		return "✓"
	case "failed":
		return "✗"
	case "skipped":
		return "-"
	}
	return "⟳"
}

func (a *app) printAvatars(items []api.AvatarSummary) {
	if len(items) == 0 {
		a.printf("No avatars.\n")
		return
	}
	w := tabwriter.NewWriter(a.out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tENGINE\tKIND\tSTATUS\tVISIBILITY\tCREATED")
	for _, x := range items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", x.Id, x.Name, engine(deref(x.Engine)), x.Kind,
			strings.ReplaceAll(string(x.Status), "_", " "), x.Visibility, x.CreatedAt.Format("2006-01-02"))
	}
	_ = w.Flush()
}

func (a *app) printAvatar(x *api.Avatar) {
	a.printf("%s  %s\n", x.Name, x.Id)
	a.printf("Engine %s · %s · %s · %s · ready to chat: %v\n", engine(deref(x.Engine)), x.Kind, x.Visibility,
		strings.ReplaceAll(string(x.Status), "_", " "), x.ReadyToChat)
	if x.MissingInputs != nil {
		a.noteMissing(x.Id.String(), missingNames(*x.MissingInputs))
	}
	if engine(deref(x.Engine)) == "v2" {
		serving := "-"
		if x.ServingCandidateId != nil {
			serving = x.ServingCandidateId.String()
		}
		sel := "auto"
		if x.SeedSelection != nil {
			sel = string(*x.SeedSelection)
		}
		rebuild := ""
		if deref(x.Rebuilding) {
			rebuild = ", rebuilding"
		}
		a.printf("Serving candidate: %s (selection %s%s)\n", serving, sel, rebuild)
		if x.SelectedCandidateId != nil && (x.ServingCandidateId == nil || *x.SelectedCandidateId != *x.ServingCandidateId) {
			a.printf("Selected candidate: %s (serving once its config is built)\n", x.SelectedCandidateId.String())
		}
		if x.RuntimeSeed != nil {
			a.printf("Runtime seed: %d\n", *x.RuntimeSeed)
		}
		if r := x.Resolution; r != nil {
			a.printf("Resolution: %dx%d\n", r.Width, r.Height)
		}
		if cd := x.Conductor; cd != nil {
			if cd.Configured {
				a.printf("Conductor: %s (atmee avatars conductor get %s)\n", plural(deref(cd.Nodes), "node"), x.Id)
			} else {
				a.printf("Conductor: none\n")
			}
		}
	}
	a.printSettings(x)
	for _, name := range stageOrder {
		st, ok := stageOf(x, name)
		if !ok {
			continue
		}
		line := fmt.Sprintf("  %s %s: %s", stageIcon(st.status), name, st.status)
		if st.feedback != "" {
			line += " (" + st.feedback + ")"
		}
		a.printf("%s\n", line)
	}
	if f := x.LatestFailure; f != nil {
		a.printf("Last failure at %s: %s\n", f.Stage, orDefault(deref(f.UserFeedback), "no details"))
	}
	if p := x.PreviewUrls; p != nil && p.Talking != nil {
		a.printf("Talking preview: %s\n", *p.Talking)
	}
	if l := x.Links; l != nil {
		a.printf("Chat page: %s\nStudio: %s\n", l.Chat, l.Studio)
	}
	if deref(x.SelectionRequired) {
		a.printf("Next: atmee avatars candidates pick %s\n", x.Id)
	}
}

type stageView struct{ status, feedback string }

// stageOf reads one build stage by name (absent stages have no status).
func stageOf(x *api.Avatar, name string) (stageView, bool) {
	var st api.BuildStage
	switch name {
	case "appearance":
		st = x.Stages.Appearance
	case "voice":
		st = x.Stages.Voice
	case "persona":
		st = x.Stages.Persona
	case "knowledge":
		st = x.Stages.Knowledge
	case "previews":
		st = x.Stages.Previews
	}
	if st.Status == "" {
		return stageView{}, false
	}
	return stageView{status: string(st.Status), feedback: deref(st.UserFeedback)}, true
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func (a *app) progress(x *api.Avatar) {
	var parts []string
	for _, name := range stageOrder {
		if st, ok := stageOf(x, name); ok {
			parts = append(parts, name+"="+st.status)
		}
	}
	a.note("%s: %s\n", x.Status, strings.Join(parts, " "))
}

func (a *app) printCandidates(p *api.CandidatesPage) {
	title := fmt.Sprintf("Candidates (%s", p.SeedSelection)
	if p.Quota != nil {
		title += fmt.Sprintf("; room for %d more captures", p.Quota.Remaining)
	}
	a.printf("%s)\n", title)
	w := tabwriter.NewWriter(a.out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "CANDIDATE\tVERSION\tSTATUS\tSERVING\tPREVIEW")
	restorable := false
	for _, c := range p.Items {
		serving := ""
		if c.Serving {
			serving = "yes"
		}
		preview := deref(c.PreviewUrl)
		if preview == "" {
			preview = deref(c.UserFeedback)
		}
		version := deref(c.VersionId)
		if version == "" {
			version = "-"
		}
		if c.Current != nil && !*c.Current {
			version += " (older)"
		}
		if c.Restorable != nil && *c.Restorable {
			restorable = true
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", c.CandidateId, version, c.Status, serving, preview)
	}
	_ = w.Flush()
	if restorable {
		a.printf("Selecting a completed candidate of an older version goes back to that version.\n")
	}
	if p.SelectionRequired {
		a.printf("A candidate is waiting to be picked.\n")
	}
	if p.Warning != nil && *p.Warning != "" {
		a.printf("%s\n", *p.Warning)
	}
}

// missingNames are a draft's missing inputs as the API names them.
func missingNames[T ~string](in []T) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}

// noteMissing says what a draft still needs and the command that gives it.
func (a *app) noteMissing(id string, missing []string) {
	if len(missing) == 0 {
		return
	}
	cmds := map[string]string{"image": "atmee avatars replace-appearance " + id + " PORTRAIT", "voice": "atmee avatars replace-voice " + id + " VOICE_SAMPLE"}
	a.printf("Draft: no %s yet. It builds once given:\n", strings.Join(missing, " or "))
	for _, m := range missing {
		if c, ok := cmds[m]; ok {
			a.printf("  %s\n", a.rename(c))
		}
	}
}

// printSettings shows what was set on the avatar (the full text is in
// --json; the facts and files in get-knowledge).
func (a *app) printSettings(x *api.Avatar) {
	if x.Language != nil {
		a.printf("Language: %s\n", *x.Language)
	}
	if x.PreviewMessage != nil {
		a.printf("Preview message: %s\n", *x.PreviewMessage)
	}
	if p := x.Persona; p != nil {
		for _, f := range []struct{ name, v string }{
			{"Alias", deref(p.PreferredAlias)}, {"Who are you", deref(p.WhoPrompt)}, {"What are we doing", deref(p.WhatPrompt)},
			{"Greeting", deref(p.Greeting)}, {"Self-introduction", deref(p.SelfIntroduction)},
		} {
			if f.v != "" {
				a.printf("%s: %s\n", f.name, oneLine(f.v, 100))
			}
		}
		if p.Guardrails != nil && len(*p.Guardrails) > 0 {
			a.printf("Guardrails: %s\n", plural(len(*p.Guardrails), "rule"))
		}
	}
	a.printProfile(x)
	if k := x.Knowledge; k != nil && k.Facts != nil {
		a.printf("Facts: %s\n", oneLine(*k.Facts, 100))
	}
	if k := x.Knowledge; k != nil && (k.FactsChars > 0 || len(k.Files) > 0 || k.QuestionsAnswered > 0 || len(k.IntegrationIds) > 0) {
		a.printf("Knowledge: %d characters of facts, %s, %s answered, %s (atmee avatars get-knowledge %s)\n",
			k.FactsChars, plural(len(k.Files), "file"), plural(k.QuestionsAnswered, "question"),
			plural(len(k.IntegrationIds), "integration"), x.Id)
	}
}

// oneLine is s on one line, cut to max characters.
func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return s
}

func (a *app) printKnowledge(k *api.Knowledge) {
	if k.Facts != nil {
		a.printf("Facts:\n%s\n\n", *k.Facts)
	} else {
		a.printf("Facts: none\n")
	}
	if len(k.Questionnaire) > 0 {
		a.printf("Questionnaire:\n")
		qs := make([]string, 0, len(k.Questionnaire))
		for q := range k.Questionnaire {
			qs = append(qs, q)
		}
		sort.Strings(qs)
		for _, q := range qs {
			a.printf("  %s\n    %s\n", q, k.Questionnaire[q])
		}
	}
	if len(k.IntegrationIds) > 0 {
		a.printf("Integrations (news and social feeds): %s\n", strings.Join(k.IntegrationIds, ", "))
	}
	if len(k.Files) == 0 {
		a.printf("Files: none\n")
		return
	}
	a.printf("Files:\n")
	for _, f := range k.Files {
		a.printf("  %s", f.Name)
		if f.Description != nil {
			a.printf(" — %s", *f.Description)
		}
		a.printf("\n")
		if f.Url != nil {
			a.printf("    %s\n", *f.Url)
		} else {
			a.printf("    (no download link: not stored with this avatar, or it could not be signed just now; try again)\n")
		}
	}
	if k.ExpiresAt != nil {
		a.note("Links valid until %s\n", k.ExpiresAt.Local().Format(time.RFC1123))
	}
}

// statementNames are the statement types as --statement names them.
var statementNames = map[string]string{
	"self_verification": "self", "representative_verification": "representative", "fictional_verification": "fictional",
}

// printProfile shows the Explore profile (Studio's share card) and
// monetization, compactly.
func (a *app) printProfile(x *api.Avatar) {
	p := x.Profile
	if p == nil {
		return
	}
	if v := deref(p.Title); v != "" {
		a.printf("Title: %s\n", oneLine(v, 100))
	}
	if v := deref(p.Subtitle); v != "" {
		a.printf("Subtitle: %s\n", oneLine(v, 100))
	}
	if len(p.Tags) > 0 {
		a.printf("Tags: %s\n", strings.Join(p.Tags, ", "))
	}
	if len(p.SocialLinks) > 0 {
		links := make([]string, len(p.SocialLinks))
		for i, l := range p.SocialLinks {
			links[i] = l.Type + " " + l.Url
		}
		a.printf("Social: %s\n", strings.Join(links, " · "))
	}
	statement := "none"
	if s := deref(p.StatementType); s != "" {
		statement = orDefault(statementNames[s], s)
		if p.Verified {
			statement += ", verified"
		}
	}
	thumb := "none (Explore shows the portrait)"
	if p.ThumbnailSource != nil {
		thumb = string(*p.ThumbnailSource)
	}
	a.printf("Statement: %s · monetized: %s · thumbnail: %s\n", statement, yesNo(p.MonetizationEnabled), thumb)
}
