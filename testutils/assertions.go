package testutils

import (
	"strings"
	"testing"
)

// AssertDiffContains verifies that the given diff contains all expected strings.
// It calls t.Helper() to ensure accurate stack traces and fails the test if any
// expected string is missing.
func AssertDiffContains(t *testing.T, diff string, want ...string) {
	t.Helper()
	for _, s := range want {
		if !strings.Contains(diff, s) {
			t.Fatalf("staged diff missing %q\n\nActual diff:\n%s", s, diff)
		}
	}
}

// AssertDiffNotContains verifies that the given diff does not contain any unwanted strings.
// It calls t.Helper() to ensure accurate stack traces and fails the test if any
// unwanted string is found.
func AssertDiffNotContains(t *testing.T, diff string, unwanted ...string) {
	t.Helper()
	for _, s := range unwanted {
		if strings.Contains(diff, s) {
			t.Fatalf("staged diff should not contain %q\n\nActual diff:\n%s", s, diff)
		}
	}
}
