package main

import "testing"

// TestNormalizeNamespace pins the Windows path-separator regression: filepath
// operations emit '\' on Windows, and downstream identifier builders split on
// '/', so a backslash used to leak straight into generated symbols (e.g.
// "useChannels\channelActions"). This runs and passes on every OS because the
// helper replaces backslashes explicitly, not only the host separator.
func TestNormalizeNamespace(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"already slashed", "channels/channelActions", "channels/channelActions"},
		{"windows backslash", `channels\channelActions`, "channels/channelActions"},
		{"deep windows path", `a\b\c`, "a/b/c"},
		{"single segment", "flat", "flat"},
		{"mixed separators", `a\b/c`, "a/b/c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeNamespace(tc.in); got != tc.want {
				t.Errorf("normalizeNamespace(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestApiNamespaceConversions locks in that the API name builders treat the
// namespace as a '/'-delimited logical path (not an OS path).
func TestApiNamespaceConversions(t *testing.T) {
	if got := apiNamespaceToFileName("events/voting"); got != "events-voting" {
		t.Errorf("apiNamespaceToFileName(events/voting) = %q, want events-voting", got)
	}
	if got := apiNamespaceToExportName("events/voting"); got != "EventsVoting" {
		t.Errorf("apiNamespaceToExportName(events/voting) = %q, want EventsVoting", got)
	}
}
