// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package progress

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestHeartbeatStops verifies periodic output and joins the writer before returning.
func TestHeartbeatStops(t *testing.T) {
	var out bytes.Buffer
	reporter := New(&out)
	reporter.interval = time.Millisecond
	reporter.Plan("en", 1, 1, 2)
	stop := reporter.StartBatch("ja", "ja/app.json", 1, 1, 2)
	time.Sleep(20 * time.Millisecond)
	duration := stop()
	before := out.String()
	time.Sleep(5 * time.Millisecond)

	if out.String() != before || !strings.Contains(before, "Waiting | language=ja | file=ja/app.json | batch 1/1") {
		t.Fatalf("heartbeat did not run or stop: %s", out.String())
	}

	reporter.BatchDone("ja", "ja/app.json", 1, 1, 2, duration)
	reporter.Finish(false, nil)

	if strings.Contains(out.String(), "\x1b[") || !strings.Contains(out.String(), "Complete | 2 strings translated") {
		t.Fatalf("unexpected redirected output: %s", out.String())
	}
}

// TestNoColorDisablesTerminalFormatting verifies the standard opt-out for file output.
func TestNoColorDisablesTerminalFormatting(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var out bytes.Buffer
	reporter := New(&out)
	reporter.Finish(true, nil)

	if reporter.color || strings.Contains(out.String(), "\x1b[") {
		t.Fatal("NO_COLOR output contains terminal formatting")
	}
}
