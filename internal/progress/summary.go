// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package progress

import (
	"fmt"
	"strings"
	"time"
)

// languageSummary counts translated files, validated strings, and API batch time.
type languageSummary struct {
	files    map[string]bool
	strings  int
	duration time.Duration
}

// languageStats registers a target language in first-seen order, including skipped targets.
func (r *Reporter) languageStats(language string) *languageSummary {
	if r.summaries == nil {
		r.summaries = make(map[string]*languageSummary)
	}

	if summary, exists := r.summaries[language]; exists {
		return summary
	}

	summary := &languageSummary{files: make(map[string]bool)}
	r.summaries[language] = summary
	r.languageOrder = append(r.languageOrder, language)
	return summary
}

// printLanguageSummary appends a stable table only after successful persistence.
// Duration includes provider retries and excludes planning and filesystem writes.
func (r *Reporter) printLanguageSummary() {
	if len(r.languageOrder) == 0 {
		return
	}

	width := r.width
	if r.terminal != nil {
		if current, ok := terminalWidth(r.terminal); ok {
			width = current
		}
	}

	for _, row := range r.languageSummaryRows() {
		if r.interactive {
			row = shorten(row, max(1, width-1))
		}
		fmt.Fprintln(r.out, row)
	}
}

// languageSummaryRows formats deterministic columns without terminal control sequences.
func (r *Reporter) languageSummaryRows() []string {
	localeWidth := len("Language")
	for _, language := range r.languageOrder {
		localeWidth = max(localeWidth, len(safeText(language)))
	}
	localeWidth = min(localeWidth, 24)

	rows := []string{
		fmt.Sprintf("  %-*s  %7s  %9s  %10s", localeWidth, "Language", "Files", "Strings", "Time"),
		"  " + strings.Repeat("─", localeWidth+32),
	}

	for _, language := range r.languageOrder {
		summary := r.summaries[language]
		rows = append(rows, fmt.Sprintf("  %-*s  %7d  %9d  %9.2fs", localeWidth, shorten(safeText(language), localeWidth), len(summary.files), summary.strings, summary.duration.Seconds()))
	}

	return append(rows, "")
}
