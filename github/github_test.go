// Package github provides a github provider.
package github

import (
	"context"
	"testing"
)

func TestNew(t *testing.T) {
	// Should only run if integration tests are enabled or if it's possible.
	//
	// NOTE: Uncomment the line below to run the integration test.
	t.Skip("skipping integration test")

	p, err := New(false, "owner", "repository")
	if err != nil {
		t.Fatal(err)
	}

	if err := p.Write(context.Background(), map[string]interface{}{
		"TEST1": "123",
		"TEST2": "456",
	}); err != nil {
		t.Fatal(err)
	}
}
