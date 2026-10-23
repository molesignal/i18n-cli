// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package translation

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunProgress verifies batch counts, target paths, and incremental summaries.
func TestRunProgress(t *testing.T) {
	dir := root(t)
	put(t, filepath.Join(dir, "en/app.json"), `{"a":"A","b":"B","c":"C"}`)
	var out bytes.Buffer
	options := Options{Dir: dir, Source: "en", Targets: []string{"ja", "ko"}, BatchSize: 2, Out: &out}

	if err := Run(context.Background(), options, &fake{}); err != nil {
		t.Fatal(err)
	}

	for _, fragment := range []string{"6 strings pending", "language=ja | file=ja/app.json", "language=ko | file=ko/app.json", "batch 2/2 | 1 strings", "progress 6/6", "Complete | 6 strings translated | 2 files processed | total elapsed"} {
		if !strings.Contains(out.String(), fragment) {
			t.Fatalf("missing %q in %s", fragment, out.String())
		}
	}

	out.Reset()
	if err := Run(context.Background(), options, &fake{}); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(out.String(), "Translating |") || !strings.Contains(out.String(), "Skip |") || !strings.Contains(out.String(), "Complete | 0 strings translated") {
		t.Fatalf("unexpected unchanged-run output: %s", out.String())
	}
}

// TestProgressFailureAndDryRun verifies that incomplete runs never report success.
func TestProgressFailureAndDryRun(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		dir := root(t)
		put(t, filepath.Join(dir, "en/app.json"), `{"a":"A"}`)
		var out bytes.Buffer
		options := Options{Dir: dir, Source: "en", Targets: []string{"ja"}, BatchSize: 1, DryRun: dryRun, Out: &out}
		err := Run(context.Background(), options, &fake{fail: true})
		expected := "Failed | 0/1 strings validated"

		if dryRun {
			expected = "Dry run complete | 1 strings pending | 0 strings translated | no files written"
		}

		if (err == nil) != dryRun || !strings.Contains(out.String(), expected) || strings.Contains(out.String(), "Complete |") {
			t.Fatalf("unexpected output: %s (error: %v)", out.String(), err)
		}
	}
}
