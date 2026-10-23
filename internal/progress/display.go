// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package progress

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// displayState holds the active file and batch details for the live terminal panel.
type displayState struct {
	source, language, path, phase           string
	languages, index, batch, batches, frame int
	batchStart                              time.Time
	batchDuration                           time.Duration
}

// paint refreshes a fixed-height panel without appending per-batch log lines.
func (r *Reporter) paint() {
	width := r.width
	if r.terminal != nil {
		if current, ok := terminalWidth(r.terminal); ok {
			width = current
		}
	}

	rows := r.panel(max(1, width-1))
	var output strings.Builder
	if r.drawn {
		fmt.Fprintf(&output, "\x1b[%dA", len(rows))
	}

	for _, row := range rows {
		output.WriteString("\r\x1b[2K")
		output.WriteString(row)
		output.WriteByte('\n')
	}

	fmt.Fprint(r.out, output.String())
	r.drawn = true
	r.display.frame++
}

// panel builds a compact view shared by live rendering and terminal layout tests.
func (r *Reporter) panel(width int) []string {
	d := r.display
	phase, code := d.phase, "36"
	marker := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}[d.frame%10]
	if phase == "Complete" || phase == "Up to date" {
		marker, code = "✓", "32"
	} else if phase == "Failed" {
		marker, code = "✗", "31"
	} else if phase == "Dry run" {
		marker = "○"
	}

	percent := 0
	if r.pending > 0 {
		percent = r.translated * 100 / r.pending
	} else if phase == "Complete" || phase == "Up to date" {
		percent = 100
	}

	barSize := min(24, max(4, width-34))
	filled := percent * barSize / 100
	bar := strings.Repeat("━", filled) + strings.Repeat("─", barSize-filled)
	rows := []string{
		"",
		"  i18n-cli",
		fmt.Sprintf("  %s %s  ·  %s → %d languages", marker, phase, safeText(d.source), d.languages),
		"",
		fmt.Sprintf("  %s  %3d%%  %d / %d strings", bar, percent, r.translated, r.pending),
		fmt.Sprintf("  %s  ·  file %d/%d  ·  batch %d/%d", safeText(d.language), d.index, r.files, d.batch, d.batches),
		"  " + safeText(d.path),
		fmt.Sprintf("  %s elapsed", clockTime(time.Since(r.started))),
		"",
	}

	if !d.batchStart.IsZero() {
		duration := d.batchDuration
		if phase == "Translating" {
			duration = time.Since(d.batchStart)
		}
		rows[7] += fmt.Sprintf("  ·  %.1fs batch", duration.Seconds())
	}

	switch phase {
	case "Complete", "Up to date":
		rows[5] = fmt.Sprintf("  %d translated  ·  %d files processed", r.translated, r.files)
		rows[6] = "  Translation files and state saved."
		rows[7] = "  Total time  " + clockTime(time.Since(r.started))
	case "Dry run":
		rows[4] = fmt.Sprintf("  %d strings pending  ·  %d target files", r.pending, r.files)
		rows[5] = "  No API calls. No files written."
		rows[6] = ""
		rows[7] = "  Total time  " + clockTime(time.Since(r.started))
	case "Failed":
		rows[5] = fmt.Sprintf("  %d / %d strings validated", r.translated, r.pending)
		rows[7] = "  Output not confirmed  ·  " + clockTime(time.Since(r.started))
	case "Saving":
		rows[6] = "  Saving translation files and state…"
	}

	for i, row := range rows {
		rows[i] = shorten(row, width)
		if r.color && (i == 1 || i == 2 || i == 4) {
			style := code
			if i == 1 {
				style = "1"
			}
			rows[i] = "\x1b[" + style + "m" + rows[i] + "\x1b[0m"
		}
	}

	return rows
}

// safeText escapes control sequences and wide characters in user-controlled labels.
func safeText(value string) string {
	quoted := strconv.QuoteToASCII(value)
	return quoted[1 : len(quoted)-1]
}

// shorten keeps each panel row within the available terminal columns.
func shorten(value string, width int) string {
	if utf8.RuneCountInString(value) <= width {
		return value
	}

	if width <= 1 {
		return "…"
	}

	chars := []rune(value)
	return string(chars[:width-1]) + "…"
}

// clockTime formats elapsed duration consistently for fast and long-running batches.
func clockTime(duration time.Duration) string {
	seconds := int(duration.Seconds())
	if seconds >= 3600 {
		return fmt.Sprintf("%02d:%02d:%02d", seconds/3600, seconds/60%60, seconds%60)
	}

	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}
