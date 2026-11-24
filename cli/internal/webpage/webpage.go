// Package webpage tells a web page from an API's answer: a firewall's block
// page or a proxy's error page, which is not for a terminal.
package webpage

import (
	"bytes"
	"strconv"
	"strings"
)

// Is says whether an answer is a web page.
func Is(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return true
	}
	head := bytes.ToLower(bytes.TrimSpace(body[:min(len(body), 512)]))
	for _, tag := range []string{"<!doctype html", "<html", "<head", "<body"} {
		if bytes.HasPrefix(head, []byte(tag)) {
			return true
		}
	}
	return false
}

// Error is a web page answering where the API should.
type Error struct {
	// Where is the host and path asked for.
	Where  string
	Status int
}

func (e *Error) Error() string { return e.Where + ": " + statusText(e.Status) + Message(e.Status) }

func statusText(status int) string {
	if status >= 200 && status < 300 {
		return ""
	}
	return strconv.Itoa(status) + " "
}

// Message is the one line said instead of a web page.
func Message(status int) string {
	m := "the server answered with a web page, not the API"
	if status == 403 {
		m += "; a firewall may block this path"
	}
	return m
}
