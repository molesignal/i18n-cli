// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestConfigDryRunAndOverrides verifies config loading and explicit locale overrides.
func TestConfigDryRunAndOverrides(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	os.MkdirAll(filepath.Join(dir, "src/locales"), 0755)
	os.WriteFile(filepath.Join(dir, "src/locales/en.json"), []byte(`{"a":"Hello"}`), 0644)
	path := filepath.Join(dir, "i18n.config.json")
	os.WriteFile(path, []byte(`{"version":1,"provider":{"id":"openai","model":"test","prompt":"from {source} to {target}"},"locale":{"source":"en","targets":["en"]},"buckets":{"json":{"include":["src/locales/[locale].json"]}}}`), 0644)

	if err := Run(context.Background(), []string{"translate", "--config", path, "--dry-run", "--target", "ja,ko"}, io.Discard); err != nil {
		t.Fatal(err)
	}

	if err := Run(context.Background(), []string{"translate", "--config", path, "--dry-run"}, io.Discard); err == nil {
		t.Fatal("invalid configured locale accepted")
	}
}
