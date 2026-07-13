package store

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Nvidia NIM":  "nvidia-nim",
		"Qwen Cloud":  "qwen-cloud",
		"MistralAi":   "mistralai",
		"  Foo  Bar ": "foo-bar",
		"":            "provider",
		"a:b":         "a-b",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q)=%q want %q", in, got, want)
		}
	}
}

func TestValidSlug(t *testing.T) {
	ok := []string{"mistral", "nvidia-nim", "qwen.cloud", "a1", "groq"}
	bad := []string{"", "Nvidia", "has space", "-lead", "trail-", "a:b", "UPPER"}
	for _, s := range ok {
		if !ValidSlug(s) {
			t.Errorf("ValidSlug(%q) want true", s)
		}
	}
	for _, s := range bad {
		if ValidSlug(s) {
			t.Errorf("ValidSlug(%q) want false", s)
		}
	}
}
