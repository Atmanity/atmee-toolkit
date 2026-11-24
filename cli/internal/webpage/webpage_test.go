package webpage

import "testing"

func TestIs(t *testing.T) {
	for _, c := range []struct {
		contentType, body string
		want              bool
	}{
		{"text/html; charset=UTF-8", "Attention Required!", true},
		{"", "\n  <!DOCTYPE html><html>", true},
		{"text/plain", "<html lang=en>", true},
		{"application/json", `{"error":"x"}`, false},
		{"text/plain", "404 page not found", false},
		{"application/xml", `<?xml version="1.0"?><Error>`, false},
		{"", "", false},
	} {
		if got := Is(c.contentType, []byte(c.body)); got != c.want {
			t.Errorf("%q %q: %v", c.contentType, c.body, got)
		}
	}
}
