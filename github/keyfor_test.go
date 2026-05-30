package github

import "testing"

// TestPublicKeyForTarget guards the regression where Actions secrets were
// encrypted with the Codespaces public key. The selected key must match the
// requested target, defaulting to Actions.
func TestPublicKeyForTarget(t *testing.T) {
	actions := &PublicKeyResponse{Key: "actions-key", KeyID: "actions-id"}
	codespaces := &PublicKeyResponse{Key: "codespaces-key", KeyID: "codespaces-id"}

	g := &GitHub{
		publicKeyResponseActions:   actions,
		publicKeyResponseCodespace: codespaces,
	}

	tests := map[string]*PublicKeyResponse{
		Actions.String():    actions,
		Codespaces.String(): codespaces,
		"":                  actions, // default target is actions
		"unknown":           actions, // anything non-codespaces -> actions
	}

	for target, want := range tests {
		if got := g.publicKeyForTarget(target); got != want {
			t.Errorf("publicKeyForTarget(%q) = %+v, want %+v", target, got, want)
		}
	}
}
