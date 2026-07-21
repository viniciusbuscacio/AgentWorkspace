package domain

import "testing"

func TestParseBrowserProfileDefaultsToaw(t *testing.T) {
	profile, err := ParseBrowserProfile("")
	if err != nil {
		t.Fatalf("ParseBrowserProfile(empty) error = %v", err)
	}
	if profile != BrowserProfileaw {
		t.Fatalf("ParseBrowserProfile(empty) = %q, want %q", profile, BrowserProfileaw)
	}
}

// The personal profile is gone for good: Chrome/Edge 136+ ignore the
// remote-debugging port on the default user data dir, so it cannot work.
func TestParseBrowserProfileRejectsPersonal(t *testing.T) {
	if _, err := ParseBrowserProfile("personal"); err == nil {
		t.Fatal("personal profile must be rejected")
	}
}

func TestParseBrowserProfileIsCaseInsensitiveAndTrims(t *testing.T) {
	for raw, want := range map[string]string{
		" AW ":       BrowserProfileaw,
		"INPRIVATE ": BrowserProfileInPrivate,
	} {
		got, err := ParseBrowserProfile(raw)
		if err != nil {
			t.Fatalf("ParseBrowserProfile(%q) error = %v", raw, err)
		}
		if got != want {
			t.Fatalf("ParseBrowserProfile(%q) = %q, want %q", raw, got, want)
		}
	}
}
