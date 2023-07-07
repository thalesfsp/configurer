// Package github provides a github provider.
package github

import (
	"context"
	"testing"

	"github.com/thalesfsp/go-common-types/safeslice"
)

func TestNew(t *testing.T) {
	// Should only run if integration tests are enabled or if it's possible.
	//
	// NOTE: Uncomment the line below to run the integration test.
	// t.Skip("skipping integration test")

	p, err := New(false, "WreckingBallStudioLabs", "proj-ringboost-primary-api")
	if err != nil {
		t.Fatal(err)
	}

	list, err := List(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}

	ss := safeslice.New(list.Secrets...)

	secretNames := safeslice.Pluck(ss, func(s SecretsResponseSecret) string {
		if s.Name != "" {
			return s.Name
		}

		return ""
	})

	t.Log(secretNames)

	// if err := Delete(context.Background(), p, secretNames...); err != nil {
	// 	t.Fatal(err)
	// }

	// ss.Filter(func(sR *SecretsResponseSecret) bool {
	// 	return sR.Name != ""
	// })

	// if err := p.Write(context.Background(), map[string]interface{}{
	// 	"TEST1": "123",
	// 	"TEST2": "456",
	// }); err != nil {
	// 	t.Fatal(err)
	// }
}
