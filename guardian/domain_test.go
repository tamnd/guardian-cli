package guardian

import (
	"testing"
)

// These tests are offline: they exercise the URI driver's pure string functions.
// The client's HTTP behaviour is covered in guardian_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "guardian" {
		t.Errorf("Scheme = %q, want guardian", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "guardian" {
		t.Errorf("Identity.Binary = %q, want guardian", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in  string
		typ string
		id  string
	}{
		{"technology/2024/abc", "article", "technology/2024/abc"},
		{"technology", "section", "technology"},
		{"climate change", "query", "climate change"},
		{"https://www.theguardian.com/technology/2024/abc", "article", "technology/2024/abc"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil {
			t.Errorf("Classify(%q) error: %v", tc.in, err)
			continue
		}
		if typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q), want (%q, %q)",
				tc.in, typ, id, tc.typ, tc.id)
		}
	}
}

func TestLocate(t *testing.T) {
	cases := []struct {
		uriType string
		id      string
		want    string
	}{
		{"article", "technology/2024/abc", "https://www.theguardian.com/technology/2024/abc"},
		{"section", "technology", "https://www.theguardian.com/technology"},
		{"query", "climate change", "https://www.theguardian.com/search?q=climate+change"},
	}
	for _, tc := range cases {
		got, err := Domain{}.Locate(tc.uriType, tc.id)
		if err != nil {
			t.Errorf("Locate(%q, %q) error: %v", tc.uriType, tc.id, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Locate(%q, %q) = %q, want %q", tc.uriType, tc.id, got, tc.want)
		}
	}
}
