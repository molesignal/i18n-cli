// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package translation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type fake struct {
	calls   int
	entries int
	fail    bool
}

// Translate records batch usage and returns predictable translations or a simulated failure.
func (f *fake) Translate(_ context.Context, _, target string, input map[string]string) (map[string]string, error) {
	f.calls++
	f.entries += len(input)

	if f.fail {
		return nil, errors.New("provider failed")
	}

	out := map[string]string{}

	for k, v := range input {
		out[k] = target + ":" + v
	}

	return out, nil
}

// root creates a temporary directory with resolved symlinks for filesystem tests.
func root(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	return dir
}

// put writes a test fixture, creating its parent directories as needed.
func put(t *testing.T, path, data string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestIncrementalNestedAndDeletion verifies incremental updates and source structure preservation.
func TestIncrementalNestedAndDeletion(t *testing.T) {
	dir := root(t)
	source := filepath.Join(dir, "en", "app.json")
	put(t, source, `{"a":"Hello {{name}}","nested":{"b":"World"},"arr":["Item",42,true,null],"empty":""}`)
	o := Options{Dir: dir, Source: "en", Targets: []string{"zh-CN"}, BatchSize: 2}
	f := &fake{}

	if err := Run(context.Background(), o, f); err != nil {
		t.Fatal(err)
	}

	if f.entries != 3 || f.calls != 2 {
		t.Fatalf("calls=%d entries=%d", f.calls, f.entries)
	}

	if err := Run(context.Background(), o, f); err != nil {
		t.Fatal(err)
	}

	if f.entries != 3 {
		t.Fatal("unchanged source translated again")
	}

	put(t, source, `{"a":"Hi {{name}}","arr":["Item",43,true,null],"empty":""}`)

	if err := Run(context.Background(), o, f); err != nil {
		t.Fatal(err)
	}

	if f.entries != 4 {
		t.Fatal("expected only changed string")
	}

	data, _ := os.ReadFile(filepath.Join(dir, "zh-CN", "app.json"))
	got, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}

	want, _ := Decode([]byte(`{"a":"zh-CN:Hi {{name}}","arr":["zh-CN:Item",43,true,null],"empty":""}`))

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected output %s", data)
	}
}

// TestFailureAndDryRunDoNotWrite verifies that failures and dry runs leave no output files.
func TestFailureAndDryRunDoNotWrite(t *testing.T) {
	for _, dry := range []bool{false, true} {
		t.Run(map[bool]string{false: "failure", true: "dry"}[dry], func(t *testing.T) {
			dir := root(t)
			put(t, filepath.Join(dir, "en", "a.json"), `{"a":"A"}`)
			o := Options{Dir: dir, Source: "en", Targets: []string{"ja"}, BatchSize: 1, DryRun: dry}
			f := &fake{fail: true}
			err := Run(context.Background(), o, f)

			if (err == nil) != dry {
				t.Fatalf("unexpected error %v", err)
			}

			for _, path := range []string{"ja/a.json", ".i18n-cli-state.json", ".i18n-cli.lock"} {
				if _, err := os.Stat(filepath.Join(dir, path)); !os.IsNotExist(err) {
					t.Fatalf("unexpected file %s", path)
				}
			}

			if dry && f.calls != 0 {
				t.Fatal("dry run called provider")
			}
		})
	}
}

// TestAdoptAndFilePattern verifies existing translation adoption and prompt invalidation.
func TestAdoptAndFilePattern(t *testing.T) {
	dir := root(t)
	put(t, filepath.Join(dir, "src/locales/en.json"), `{"a":"Hello","b":"New"}`)
	put(t, filepath.Join(dir, "src/locales/ja.json"), `{"a":"こんにちは"}`)
	o := Options{Dir: dir, Source: "en", Targets: []string{"ja"}, BatchSize: 40, AdoptExisting: true, Include: []string{"src/locales/[locale].json"}}
	f := &fake{}

	if err := Run(context.Background(), o, f); err != nil {
		t.Fatal(err)
	}

	if f.entries != 1 {
		t.Fatalf("translated %d", f.entries)
	}

	o.Instructions = "New style"

	if err := Run(context.Background(), o, f); err != nil {
		t.Fatal(err)
	}

	if f.entries != 3 {
		t.Fatal("prompt change did not invalidate hashes")
	}
}

// TestInvalidInputs verifies source preflight and placeholder validation failures.
func TestInvalidInputs(t *testing.T) {
	for _, data := range []string{`{"a":"A","a":"B"}`, `{"a":"{n, plural, one {item} other {items}}"}`} {
		dir := root(t)
		put(t, filepath.Join(dir, "en/a.json"), data)
		f := &fake{}

		if err := Run(context.Background(), Options{Dir: dir, Source: "en", Targets: []string{"ja"}, BatchSize: 2}, f); err == nil {
			t.Fatal("expected validation error")
		}

		if f.calls != 0 {
			t.Fatal("invalid input triggered request")
		}
	}

	for _, pair := range [][2]string{{"Hello {{name}}", "Hello"}, {"%s %s", "%s"}, {"<b>Hi</b>", "Hi"}, {"{name}", "{other}"}} {
		if Validate(pair[0], pair[1]) == nil {
			t.Fatal("invalid placeholder accepted")
		}
	}
}

// TestSymlinkOutputRejected verifies that output paths cannot traverse symlinks.
func TestSymlinkOutputRejected(t *testing.T) {
	dir := root(t)
	put(t, filepath.Join(dir, "en/a.json"), `{"a":"A"}`)

	if err := os.Symlink(root(t), filepath.Join(dir, "ja")); err != nil {
		t.Fatal(err)
	}

	if err := Run(context.Background(), Options{Dir: dir, Source: "en", Targets: []string{"ja"}, BatchSize: 2}, &fake{}); err == nil {
		t.Fatal("symlink accepted")
	}
}
