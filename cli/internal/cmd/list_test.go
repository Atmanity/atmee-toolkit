package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const (
	listA = "11111111-2222-4333-8444-555555555551"
	listB = "11111111-2222-4333-8444-555555555552"
	listC = "11111111-2222-4333-8444-555555555553"
)

func listRow(id, name string, usage, copyID string) string {
	row := `{"id":"` + id + `","name":"` + name + `","kind":"conversational","visibility":"private",` +
		`"status":"ready","engine":"v1","readyToChat":true,"createdAt":"2026-11-10T10:00:00Z","updatedAt":"2026-11-10T10:00:00Z"`
	if usage != "" {
		row += `,"usage":` + usage
	}
	if copyID != "" {
		row += `,"upgradedCopyId":"` + copyID + `"`
	}
	return row + "}"
}

// listAPI answers GET /v1/avatars with the pages in order and records each
// request's query string.
func listAPI(t *testing.T, queries *[]string, pages ...string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*queries = append(*queries, r.URL.RawQuery)
		page := pages[min(len(*queries), len(pages))-1]
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, page)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
}

func TestListSendsSortAndFilters(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	page := `{"items":[` + listRow(listA, "Ada", `{"users":3,"avgSessionMinutes":3.4,"totalMinutes":20}`, listB) + `],"total":1}`
	cases := []struct {
		args []string
		want string
	}{
		// No flags: the sorted list in creation order, so usage comes back.
		{nil, "limit=50&order=desc&sort=created"},
		{[]string{"--sort", "totalChat", "--order", "asc", "--limit", "5"}, "limit=5&order=asc&sort=totalChat"},
		{[]string{"--q", "mira", "--engine", "v1, v2hd", "--frame", "portrait"},
			"engine=v1%2Cv2hd&frame=portrait&limit=50&q=mira"},
		{[]string{"--visibility", "public,unlisted", "--badge", "standard", "--sponsored", "false", "--offset", "20"},
			"badge=standard&limit=50&offset=20&sponsored=false&visibility=public%2Cunlisted"},
	}
	for _, c := range cases {
		var queries []string
		listAPI(t, &queries, page)
		out, errOut, code := run(t, "", append([]string{"avatars", "list"}, c.args...)...)
		if code != 0 || len(queries) != 1 || !sameQuery(queries[0], c.want) {
			t.Fatalf("%v: code %d %s query %v, want %s", c.args, code, errOut, queries, c.want)
		}
		for _, want := range []string{"USERS", "AVG MIN", "TOTAL MIN", "V2 COPY", "3.4", "20.0", listB} {
			if !strings.Contains(out, want) {
				t.Fatalf("%v: output lacks %q:\n%s", c.args, want, out)
			}
		}
	}
}

func TestListAllFollowsOffsets(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	var queries []string
	listAPI(t, &queries,
		`{"items":[`+listRow(listA, "Ada", `{"users":1,"avgSessionMinutes":1,"totalMinutes":1}`, "")+`],"total":2,"nextOffset":1}`,
		`{"items":[`+listRow(listC, "Cy", `{"users":0,"avgSessionMinutes":0,"totalMinutes":0}`, "")+`],"total":2}`)
	out, errOut, code := run(t, "", "avatars", "list", "--sort", "name", "--limit", "1", "--all", "--json")
	if code != 0 || len(queries) != 2 || !strings.Contains(queries[1], "offset=1") {
		t.Fatalf("code %d %s queries %v", code, errOut, queries)
	}
	got := jsonOf(t, out)
	if items, _ := got["items"].([]any); len(items) != 2 || got["total"] != float64(2) {
		t.Fatalf("json: %v", got)
	}

	queries = nil
	listAPI(t, &queries, `{"items":[`+listRow(listA, "Ada", `{"users":1,"avgSessionMinutes":1,"totalMinutes":1}`, "")+`],"total":2,"nextOffset":1}`)
	_, errOut, _ = run(t, "", "avatars", "list", "--limit", "1")
	if !strings.Contains(errOut, "Showing 1 of 2") || !strings.Contains(errOut, "--offset 1") {
		t.Fatalf("next-page hint: %s", errOut)
	}
}

func TestListTimeCursorKeepsTheOldPath(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	var queries []string
	listAPI(t, &queries, `{"items":[`+listRow(listA, "Ada", "", "")+`]}`)
	out, errOut, code := run(t, "", "avatars", "list", "--before", "2026-11-10T10:00:00Z")
	if code != 0 || len(queries) != 1 || strings.Contains(queries[0], "sort=") || !strings.Contains(queries[0], "before=") {
		t.Fatalf("code %d %s queries %v", code, errOut, queries)
	}
	if strings.Contains(out, "USERS") {
		t.Fatalf("no usage columns without usage:\n%s", out)
	}
}

func TestListUsageErrors(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"--sort", "popular"}, "--sort takes created, updated, name, users, avgChat or totalChat"},
		{[]string{"--order", "up"}, "--order takes asc or desc"},
		{[]string{"--engine", "v3"}, "--engine takes one or more of v2hd, v2, v1"},
		{[]string{"--visibility", "public,secret"}, "--visibility takes one or more of public, unlisted, private"},
		{[]string{"--sponsored", "yes"}, "--sponsored takes true or false"},
		{[]string{"--offset", "-1"}, "--offset takes 0 or more"},
		{[]string{"--limit", "0"}, "--limit takes 1 to 100"},
		{[]string{"--limit", "101"}, "--limit takes 1 to 100"},
		{[]string{"--before", "2026-11-10T10:00:00Z", "--limit", "-5"}, "--limit takes 1 to 100"},
		{[]string{"--before", "2026-11-10T10:00:00Z", "--sort", "name"}, "--before pages by creation time"},
	} {
		var queries []string
		listAPI(t, &queries, `{"items":[]}`)
		_, errOut, code := run(t, "", append([]string{"avatars", "list"}, c.args...)...)
		if code == 0 || !strings.Contains(errOut, c.want) || len(queries) != 0 {
			t.Fatalf("%v: code %d err %q queries %v", c.args, code, errOut, queries)
		}
	}
}

// sameQuery compares two query strings as parameter sets, in any order.
func sameQuery(got, want string) bool {
	g, err1 := url.ParseQuery(got)
	w, err2 := url.ParseQuery(want)
	return err1 == nil && err2 == nil && g.Encode() == w.Encode()
}
