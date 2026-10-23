// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package progress

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestLanguageSummary verifies per-language aggregation, stable order, and skipped files.
func TestLanguageSummary(t *testing.T) {
	for _, interactive := range []bool{false, true} {
		var out bytes.Buffer
		r := New(&out)
		r.interactive = interactive
		r.Plan("en", 3, 4, 7)
		r.File("zh-CN", "zh-CN/a.json", 1, 4, 2)
		r.BatchDone("zh-CN", "zh-CN/a.json", 1, 2, 2, 1200*time.Millisecond)
		r.BatchDone("zh-CN", "zh-CN/a.json", 2, 2, 2, 800*time.Millisecond)
		r.File("zh-CN", "zh-CN/b.json", 2, 0, 0)
		r.File("ja", "ja/a.json", 3, 3, 1)
		r.BatchDone("ja", "ja/a.json", 1, 1, 3, 3500*time.Millisecond)
		r.File("fr", "fr/a.json", 4, 0, 0)
		r.Finish(false, nil)

		rows := r.languageSummaryRows()
		expected := []string{"zh-CN 1 4 2.00s", "ja 1 3 3.50s", "fr 0 0 0.00s"}
		for i, want := range expected {
			got := strings.Join(strings.Fields(rows[i+2]), " ")
			if got != want || !strings.Contains(out.String(), rows[i+2]) {
				t.Fatalf("unexpected summary row %q; want %q", got, want)
			}
		}
	}
}

// TestLanguageSummaryOnlyAfterSuccess prevents dry runs or failures from claiming translated files.
func TestLanguageSummaryOnlyAfterSuccess(t *testing.T) {
	for _, interactive := range []bool{false, true} {
		for _, dry := range []bool{false, true} {
			var out bytes.Buffer
			r := New(&out)
			r.interactive = interactive
			r.Plan("en", 1, 1, 2)
			r.File("ja", "ja/a.json", 1, 2, 2)
			r.BatchDone("ja", "ja/a.json", 1, 2, 1, time.Second)
			err := errors.New("incomplete output")
			if dry {
				err = nil
			}
			r.Finish(dry, err)

			if strings.Contains(out.String(), "Language") || len(r.summaries["ja"].files) != 0 {
				t.Fatal("incomplete translation was reported as a completed file")
			}
		}
	}
}
