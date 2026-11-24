// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package translation

import "testing"

// TestPercentSignsInProseAreNotPlaceholders verifies that a percent sign before a word is text,
// while printf directives are still checked.
func TestPercentSignsInProseAreNotPlaceholders(t *testing.T) {
	if err := Validate("{{percent}}% of today's limit", "{{percent}} % des heutigen Limits"); err != nil {
		t.Fatalf("prose percent rejected: %v", err)
	}

	if err := Validate("Saved %d files to %s", "Saved files"); err == nil {
		t.Fatal("dropped printf directives were accepted")
	}

	if err := Validate("%1$s of %2$s", "%2$s / %1$s"); err != nil {
		t.Fatalf("reordered positional directives rejected: %v", err)
	}
}
