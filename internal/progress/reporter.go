// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

// Package progress reports translation activity as readable terminal and CI logs.
package progress

import (
	"fmt"
	"io"
	"os"
	"time"
)

// Reporter renders a live terminal panel or sequential logs for redirected output.
// Callers must stop a batch heartbeat before writing another status line.
type Reporter struct {
	out                        io.Writer
	started                    time.Time
	color                      bool
	terminal                   *os.File
	interactive, drawn         bool
	width                      int
	display                    displayState
	summaries                  map[string]*languageSummary
	languageOrder              []string
	interval                   time.Duration
	pending, translated, files int
}

// New starts a reporter, enabling color only for terminal output without NO_COLOR.
func New(out io.Writer) *Reporter {
	if out == nil {
		out = io.Discard
	}

	reporter := &Reporter{out: out, started: time.Now(), interval: 5 * time.Second, width: 80}
	if file, ok := out.(*os.File); ok && os.Getenv("TERM") != "dumb" && os.Getenv("CI") == "" {
		if width, terminal := terminalWidth(file); terminal {
			reporter.terminal = file
			reporter.width = width
			reporter.interactive = true
			reporter.color = os.Getenv("NO_COLOR") == ""
			reporter.interval = 100 * time.Millisecond
		}
	}

	return reporter
}

// line writes one English status line, optionally coloring its status label.
func (r *Reporter) line(status, code, format string, args ...any) {
	if r.color {
		status = "\x1b[" + code + "m" + status + "\x1b[0m"
	}

	fmt.Fprintf(r.out, "%s | %s\n", status, fmt.Sprintf(format, args...))
}

// Plan reports pending strings across all target-language file jobs.
func (r *Reporter) Plan(source string, languages, files, pending int) {
	r.pending, r.files = pending, files
	r.display.source, r.display.languages, r.display.phase = source, languages, "Preparing"
	if r.interactive {
		r.paint()
		return
	}

	r.line("Plan", "36", "source=%s | %d target languages | %d files | %d strings pending", source, languages, files, pending)
}

// File reports the current target file and whether it requires API translation.
func (r *Reporter) File(language, path string, index, pending, batches int) {
	r.languageStats(language)

	r.display.language, r.display.path, r.display.index = language, path, index
	r.display.batch, r.display.batches = 0, batches
	r.display.batchStart = time.Time{}
	r.display.phase = "Preparing"
	if pending == 0 {
		r.display.phase = "Unchanged"
	}
	if r.interactive {
		r.paint()
		return
	}

	status := "File"
	if pending == 0 {
		status = "Skip"
	}

	r.line(status, "36", "language=%s | file=%s | file %d/%d | %d strings | %d batches | elapsed %s", language, path, index, r.files, pending, batches, elapsed(r.started))
}

// StartBatch reports a request immediately and emits periodic elapsed-time updates.
// The returned function must be called once to stop and join the heartbeat.
func (r *Reporter) StartBatch(language, path string, batch, batches, count int) func() time.Duration {
	started := time.Now()
	r.display.language, r.display.path = language, path
	r.display.batch, r.display.batches = batch, batches
	r.display.batchStart, r.display.phase = started, "Translating"
	label := fmt.Sprintf("language=%s | file=%s | batch %d/%d | %d strings", language, path, batch, batches, count)
	if r.interactive {
		r.paint()
	} else {
		r.line("Translating", "33", "%s | batch elapsed %s | total elapsed %s", label, elapsed(started), elapsed(r.started))
	}
	stop, done := make(chan struct{}), make(chan struct{})

	go func() {
		defer close(done)
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if r.interactive {
					r.paint()
				} else {
					r.line("Waiting", "33", "%s | batch elapsed %s | total elapsed %s", label, elapsed(started), elapsed(r.started))
				}
			}
		}
	}()

	return func() time.Duration {
		close(stop)
		<-done
		return time.Since(started)
	}
}

// BatchDone records only validated translations and reports completed batch progress.
func (r *Reporter) BatchDone(language, path string, batch, batches, count int, duration time.Duration) {
	r.translated += count
	summary := r.languageStats(language)
	summary.strings += count
	summary.duration += duration
	if batch == batches {
		summary.files[path] = true
	}

	r.display.batchDuration, r.display.phase = duration, "Batch complete"
	if r.interactive {
		r.paint()
		return
	}

	r.line("Batch done", "32", "language=%s | file=%s | batch %d/%d | %d strings | batch elapsed %s | progress %d/%d | total elapsed %s", language, path, batch, batches, count, duration.Round(time.Millisecond), r.translated, r.pending, elapsed(r.started))
}

// Writing announces persistence after all model responses have been validated.
func (r *Reporter) Writing() {
	if r.interactive {
		r.display.phase = "Saving"
		r.paint()
		return
	}

	r.line("Writing", "36", "saving translation files and incremental state | elapsed %s", elapsed(r.started))
}

// Finish distinguishes persisted success, dry-run planning, and incomplete execution.
func (r *Reporter) Finish(dryRun bool, err error) {
	if r.interactive {
		r.display.phase = "Complete"
		if r.translated == 0 {
			r.display.phase = "Up to date"
		}
		if dryRun {
			r.display.phase = "Dry run"
		}
		if err != nil {
			r.display.phase = "Failed"
		}

		r.paint()
		if !dryRun && err == nil {
			r.printLanguageSummary()
		}
		return
	}

	if err != nil {
		r.line("Failed", "31", "%d/%d strings validated | output not confirmed | total elapsed %s", r.translated, r.pending, elapsed(r.started))
		return
	}

	if dryRun {
		r.line("Dry run complete", "36", "%d strings pending | 0 strings translated | no files written | total elapsed %s", r.pending, elapsed(r.started))
		return
	}

	r.line("Complete", "32", "%d strings translated | %d files processed | total elapsed %s", r.translated, r.files, elapsed(r.started))
	r.printLanguageSummary()
}

// elapsed formats monotonic elapsed time with millisecond precision.
func elapsed(started time.Time) string {
	return time.Since(started).Round(time.Millisecond).String()
}
