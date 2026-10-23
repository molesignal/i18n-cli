// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package progress

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// TestTerminalPanel verifies in-place refresh, batch progress, and saved completion.
func TestTerminalPanel(t *testing.T) {
	var out bytes.Buffer
	r := New(&out)
	r.interactive, r.color, r.width = true, false, 80
	r.Plan("en", 2, 4, 24)
	r.File("ja", "locales/ja/common.json", 3, 6, 2)
	stop := r.StartBatch("ja", "locales/ja/common.json", 1, 2, 3)
	duration := stop()
	r.BatchDone("ja", "locales/ja/common.json", 1, 2, 3, duration)

	active := strings.Join(r.panel(79), "\n")
	for _, want := range []string{"i18n-cli", "12%", "3 / 24 strings", "ja  ·  file 3/4  ·  batch 1/2", "locales/ja/common.json"} {
		if !strings.Contains(active, want) {
			t.Fatalf("missing %q in panel:\n%s", want, active)
		}
	}

	if !strings.Contains(out.String(), "\x1b[9A") || strings.Contains(out.String(), "Translating |") {
		t.Fatal("terminal output should redraw instead of appending logs")
	}

	r.Writing()
	r.Finish(false, nil)
	summary := strings.Join(r.panel(79), "\n")
	if !strings.Contains(summary, "✓ Complete") || !strings.Contains(summary, "3 translated") || !strings.Contains(summary, "Total time") {
		t.Fatalf("unexpected summary:\n%s", summary)
	}
}

// TestTerminalFinalStates verifies dry-run and failure summaries cannot claim saved success.
func TestTerminalFinalStates(t *testing.T) {
	for _, tc := range []struct {
		dry  bool
		err  error
		want string
	}{
		{true, nil, "No API calls. No files written."},
		{false, errors.New("cancelled"), "Output not confirmed"},
		{false, nil, "Up to date"},
	} {
		var out bytes.Buffer
		r := New(&out)
		r.interactive = true
		r.Plan("en", 1, 1, 0)
		r.Finish(tc.dry, tc.err)
		panel := strings.Join(r.panel(79), "\n")

		if !strings.Contains(panel, tc.want) {
			t.Fatalf("missing final state %q: %s", tc.want, panel)
		}
		if (tc.dry || tc.err != nil) && strings.Contains(panel, "state saved") {
			t.Fatal("incomplete execution claimed persistence")
		}
	}
}

// TestPanelWidthAndEscaping verifies long paths cannot wrap rows or inject terminal controls.
func TestPanelWidthAndEscaping(t *testing.T) {
	r := New(&bytes.Buffer{})
	r.display = displayState{source: "en", language: "ja", path: "長い名前/" + strings.Repeat("long/path/", 30) + "\x1b[2J\nfile.json", phase: "Translating"}
	r.pending = 24
	for _, width := range []int{1, 20, 40, 79} {
		rows := r.panel(width)
		if len(rows) != 9 {
			t.Fatal("panel height changed")
		}
		for _, row := range rows {
			if utf8.RuneCountInString(row) > width || strings.ContainsAny(row, "\x1b\n\r") {
				t.Fatalf("unsafe row for width %d: %q", width, row)
			}
		}
	}
}

// TestAnimatedHeartbeatStops verifies no renderer writes after the batch is joined.
func TestAnimatedHeartbeatStops(t *testing.T) {
	var out bytes.Buffer
	r := New(&out)
	r.interactive = true
	r.interval = time.Millisecond
	r.Plan("en", 1, 1, 1)
	stop := r.StartBatch("ja", "ja/app.json", 1, 1, 1)
	time.Sleep(20 * time.Millisecond)
	stop()
	before := out.String()
	time.Sleep(5 * time.Millisecond)

	if before != out.String() || strings.Count(before, "\x1b[9A") < 2 {
		t.Fatal("animation did not update or stop")
	}
}
