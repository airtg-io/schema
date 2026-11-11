package schema

import (
	"regexp"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	// Go module tags need MAJOR.MINOR.PATCH: CI tags v<VERSION>.
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(Version) {
		t.Fatalf("Version %q is not MAJOR.MINOR.PATCH", Version)
	}
}

func TestSubjects(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{MetricsSubject("core-sw1"), "airtg.v1.metrics.core-sw1"},
		{InventorySubject("core-sw1"), "airtg.v1.inventory.core-sw1"},
		{MetricsSubject("10.0.0.1-161"), "airtg.v1.metrics.10~2e0~2e0~2e1-161"},
		{SubjectAll, "airtg.v1.>"},
		{SubjectMetricsAll, "airtg.v1.metrics.*"},
		{SubjectInventoryAll, "airtg.v1.inventory.*"},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
}

func TestToken(t *testing.T) {
	for _, s := range []string{
		"", "core-sw1", "a_b-C9", "10.0.0.1-161", "~", "~2e", "a.b*c>d e\tf",
		"é", "\x00\xff", strings.Repeat(".", 300),
	} {
		tok := Token(s)
		// A valid NATS token: not empty unless s is, none of . * > or white space.
		if strings.ContainsAny(tok, ".*> \t\r\n") {
			t.Errorf("Token(%q) = %q: not a valid subject token", s, tok)
		}
		if !regexp.MustCompile(`^[A-Za-z0-9_~-]*$`).MatchString(tok) {
			t.Errorf("Token(%q) = %q: unexpected characters", s, tok)
		}
		back, err := ParseToken(tok)
		if err != nil || back != s {
			t.Errorf("ParseToken(Token(%q)) = %q, %v", s, back, err)
		}
	}
	// Injective: ids differing only by an escaped character never collide.
	if Token("a.b") == Token("a_b") || Token("a.b") == Token("a-b") || Token("a~2eb") == Token("a.b") {
		t.Error("Token is not injective")
	}
}

func TestParseTokenErrors(t *testing.T) {
	for _, tok := range []string{"a.b", "~", "~2", "~2E", "~zz", "a b", "a*", "é"} {
		if s, err := ParseToken(tok); err == nil {
			t.Errorf("ParseToken(%q) = %q, want an error", tok, s)
		}
	}
}

func TestValidID(t *testing.T) {
	for _, id := range []string{"a", "core-sw1", "10.0.0.1-161", "A_b.c-9", strings.Repeat("a", MaxIDLength)} {
		if !ValidID(id) {
			t.Errorf("ValidID(%q) = false", id)
		}
	}
	for _, id := range []string{"", ".a", "-a", "_a", "a b", "a/b", "a~b", "é", strings.Repeat("a", MaxIDLength+1)} {
		if ValidID(id) {
			t.Errorf("ValidID(%q) = true", id)
		}
	}
}
