package events

import "testing"

func TestRegistryIsClosedAndIncludesLegacyKinds(t *testing.T) {
	for _, name := range []string{"start", "cardClick", "settingsToggleAnimation", "deploySmoke"} {
		if !Allowed(name) {
			t.Fatalf("known event %q is absent", name)
		}
	}
	if Allowed("privateText") || Allowed("") {
		t.Fatal("unknown event was accepted")
	}
}
