package schema

import (
	"regexp"
	"testing"
)

func TestUnits(t *testing.T) {
	seen := map[string]bool{}
	form := regexp.MustCompile(`^[a-z]+([ -][a-z]+)*$`)
	for _, u := range Units {
		if seen[u] {
			t.Errorf("%q listed twice", u)
		}
		seen[u] = true
		if !form.MatchString(u) {
			t.Errorf("%q: units are lowercase words, without /s", u)
		}
	}
	for _, u := range []string{"octet", "octet/s", "bit/s", "allocation unit", "percent", "second", "enum", "kilobyte"} {
		if !ValidUnit(u) {
			t.Errorf("ValidUnit(%q) = false", u)
		}
	}
	for _, u := range []string{"", "%", "bytes", "Octet", "octets/s", "centisecond", "octet/s/s", "/s", "KiB"} {
		if ValidUnit(u) {
			t.Errorf("ValidUnit(%q) = true", u)
		}
	}
}
