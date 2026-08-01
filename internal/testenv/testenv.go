// Copyright 2022 The configurer Authors. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// Package testenv provides helpers to put environment variables into a known
// state during tests.
//
// Providers decide whether to overwrite a variable based on its PRESENCE, not
// on whether it's empty, so tests must be able to say "absent" and "present but
// empty" without ambiguity. Spelling "absent" as `t.Setenv(key, "")` says
// "present and empty" instead, which is how an empty-value clobber bug once
// survived the test suite.
package testenv

import (
	"os"
	"testing"
)

// Unset removes the given keys from the environment for the duration of the
// test, restoring whatever was there before once the test finishes.
//
// t.Setenv runs first so the testing package registers its restore cleanup,
// then the variable is genuinely removed.
func Unset(tb testing.TB, keys ...string) {
	tb.Helper()

	for _, key := range keys {
		tb.Setenv(key, "placeholder")

		if err := os.Unsetenv(key); err != nil {
			tb.Fatalf("unset %s: %v", key, err)
		}

		if _, present := os.LookupEnv(key); present {
			tb.Fatalf("precondition: %s should be absent", key)
		}
	}
}

// Set makes the given key present with the given value (possibly empty) for the
// duration of the test.
func Set(tb testing.TB, key, value string) {
	tb.Helper()

	tb.Setenv(key, value)

	if _, present := os.LookupEnv(key); !present {
		tb.Fatalf("precondition: %s should be present", key)
	}
}

// RequireSet asserts that key is still PRESENT in the environment and holds
// want. os.Getenv alone cannot express this: it returns "" both for a variable
// preserved as the empty string and for one that was removed outright.
func RequireSet(tb testing.TB, key, want string) {
	tb.Helper()

	got, present := os.LookupEnv(key)
	if !present {
		tb.Fatalf("postcondition: %s should still be present, want %q", key, want)
	}

	if got != want {
		tb.Fatalf("postcondition: %s = %q, want %q", key, got, want)
	}
}

// SetPresence puts key into the requested state: present with value, or absent.
func SetPresence(tb testing.TB, key, value string, present bool) {
	tb.Helper()

	if present {
		Set(tb, key, value)

		return
	}

	Unset(tb, key)
}
