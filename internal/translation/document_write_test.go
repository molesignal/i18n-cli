// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package translation

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestWriteJSONPreservesHTML verifies readable HTML without changing JSON semantics.
func TestWriteJSONPreservesHTML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "translated.json")
	value := map[string]string{"link": "<strong>查看 Trace</strong> & \"details\""}
	if err := writeJSON(path, value); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	expected := "{\n  \"link\": \"<strong>查看 Trace</strong> & \\\"details\\\"\"\n}\n"
	if string(data) != expected {
		t.Fatalf("unexpected serialized JSON: %s", data)
	}

	decoded, err := Decode(data)
	if err != nil || !reflect.DeepEqual(Strings(decoded), map[string]string{"/link": value["link"]}) {
		t.Fatalf("JSON round trip failed: %v", err)
	}

	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := writeJSON(path, value); err != nil {
		t.Fatal(err)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("unchanged output was rewritten")
	}
}
