// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package translation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/molesignal/i18n-cli/internal/progress"
)

// Translator converts a batch of source strings into the requested target language.
type Translator interface {
	Translate(context.Context, string, string, map[string]string) (map[string]string, error)
}

// Options controls file discovery, incremental planning, and translation execution.
type Options struct {
	Dir, Source, Instructions    string
	Targets                      []string
	Include                      []string
	BatchSize                    int
	DryRun, Force, AdoptExisting bool
	Out                          io.Writer
}

// state stores source fingerprints used to detect strings requiring translation.
type state struct {
	Version int               `json:"version"`
	Hashes  map[string]string `json:"hashes"`
}

// job holds one target document and its pending or completed string translations.
type job struct {
	path, target string
	document     any
	strings      map[string]string
	pending      []string
}

var localePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

// Run plans incremental translations, validates all responses, and writes the results.
// Dry runs report pending strings without calling the translator or writing files.
// Output files are replaced individually, with the state file written last.
func Run(ctx context.Context, o Options, translator Translator) (runErr error) {
	reporter := progress.New(o.Out)
	defer func() { reporter.Finish(o.DryRun, runErr) }()

	if !localePattern.MatchString(o.Source) || len(o.Targets) == 0 {
		return fmt.Errorf("source and target locales are required")
	}

	if o.BatchSize < 1 || o.BatchSize > 200 {
		return fmt.Errorf("batch size must be between 1 and 200")
	}

	if o.Force && o.AdoptExisting {
		return fmt.Errorf("--force and --adopt-existing are mutually exclusive")
	}

	seen := map[string]bool{strings.ToLower(o.Source): true}

	for _, target := range o.Targets {
		lower := strings.ToLower(target)

		if !localePattern.MatchString(target) || seen[lower] {
			return fmt.Errorf("invalid, duplicate or source-equivalent target %q", target)
		}

		seen[lower] = true
	}

	if o.Out == nil {
		o.Out = io.Discard
	}

	root, err := filepath.Abs(o.Dir)
	if err != nil {
		return err
	}

	if err := noSymlinks(root); err != nil {
		return err
	}

	if !o.DryRun {
		lock := filepath.Join(root, ".i18n-cli.lock")
		f, err := os.OpenFile(lock, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return fmt.Errorf("acquire lock (remove stale .i18n-cli.lock only when no translator is running): %w", err)
		}

		f.Close()
		defer os.Remove(lock)
	}

	statePath := filepath.Join(root, ".i18n-cli-state.json")

	if err := noSymlinks(statePath); err != nil {
		return err
	}

	st := state{Version: 1, Hashes: map[string]string{}}

	if data, err := os.ReadFile(statePath); err == nil {
		if err := json.Unmarshal(data, &st); err != nil || st.Version != 1 || st.Hashes == nil {
			return fmt.Errorf("invalid or unsupported state file")
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	patterns := o.Include

	if len(patterns) == 0 {
		patterns = []string{"[locale]/**/*.json"}
	}

	files, err := discover(root, o.Source, patterns)
	if err != nil {
		return err
	}

	// Validate every document before the first paid request or output write.
	sourcePaths := map[string]bool{}

	for _, file := range files {
		sourcePaths[filepath.Join(root, filepath.FromSlash(file.path))] = true
	}

	outputPaths := map[string]bool{}
	var jobs []job
	next := state{Version: 1, Hashes: map[string]string{}}

	for k, v := range st.Hashes {
		next.Hashes[k] = v
	}

	for _, target := range o.Targets {
		prefix := o.Source + "/" + target + "/"

		for key := range next.Hashes {
			if strings.HasPrefix(key, prefix) {
				delete(next.Hashes, key)
			}
		}

		for _, file := range files {
			rel := file.path
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				return err
			}

			doc, err := Decode(data)
			if err != nil {
				return fmt.Errorf("%s: %w", rel, err)
			}

			original := Strings(doc)
			path := filepath.Join(root, filepath.FromSlash(file.before+target+file.after))

			if sourcePaths[path] || outputPaths[path] {
				return fmt.Errorf("source/output path collision: %s", path)
			}

			outputPaths[path] = true

			if err := noSymlinks(path); err != nil {
				return err
			}

			existing := map[string]string{}

			if data, err := os.ReadFile(path); err == nil {
				translated, err := Decode(data)
				if err != nil {
					return fmt.Errorf("%s: %w", path, err)
				}

				existing = Strings(translated)
			} else if !errors.Is(err, fs.ErrNotExist) {
				return err
			}

			j := job{path: path, target: target, document: doc, strings: map[string]string{}}

			for key, text := range original {
				if complexICU.MatchString(text) {
					return fmt.Errorf("%s %s: ICU plural/select is not supported", rel, key)
				}

				hashKey := prefix + filepath.ToSlash(rel) + "#" + key
				raw, _ := json.Marshal([]string{text, o.Instructions})
				sum := sha256.Sum256(raw)
				hash := hex.EncodeToString(sum[:])
				next.Hashes[hashKey] = hash
				old, exists := existing[key]
				valid := exists && Validate(text, old) == nil

				if text == "" {
					j.strings[key] = ""
					continue
				}

				if !o.Force && valid && (st.Hashes[hashKey] == hash || (o.AdoptExisting && st.Hashes[hashKey] == "")) {
					j.strings[key] = old
				} else {
					j.pending = append(j.pending, key)
				}
			}

			sort.Strings(j.pending)
			jobs = append(jobs, j)
		}
	}

	pending := 0
	for _, j := range jobs {
		pending += len(j.pending)
	}

	reporter.Plan(o.Source, len(o.Targets), len(jobs), pending)
	if o.DryRun {
		for i, j := range jobs {
			relative, _ := filepath.Rel(root, j.path)
			reporter.File(j.target, filepath.ToSlash(relative), i+1, len(j.pending), (len(j.pending)+o.BatchSize-1)/o.BatchSize)
		}

		return nil
	}

	// Stage all responses in memory. API/validation failures leave translations and state untouched.
	for i := range jobs {
		j := &jobs[i]
		original := Strings(j.document)
		relative, _ := filepath.Rel(root, j.path)
		displayPath := filepath.ToSlash(relative)
		batches := (len(j.pending) + o.BatchSize - 1) / o.BatchSize
		reporter.File(j.target, displayPath, i+1, len(j.pending), batches)

		for start := 0; start < len(j.pending); start += o.BatchSize {
			if err := ctx.Err(); err != nil {
				return err
			}

			end := min(start+o.BatchSize, len(j.pending))
			batch := map[string]string{}

			for _, key := range j.pending[start:end] {
				batch[key] = original[key]
			}

			if translator == nil {
				return fmt.Errorf("translator is required")
			}

			batchNumber := start/o.BatchSize + 1
			stopHeartbeat := reporter.StartBatch(j.target, displayPath, batchNumber, batches, len(batch))
			result, err := translator.Translate(ctx, o.Source, j.target, batch)
			duration := stopHeartbeat()
			if err != nil {
				return fmt.Errorf("%s: %w", j.path, err)
			}

			if len(result) != len(batch) {
				return fmt.Errorf("translation key count mismatch")
			}

			for key, source := range batch {
				value, ok := result[key]

				if !ok {
					return fmt.Errorf("missing translation key %q", key)
				}

				if err := Validate(source, value); err != nil {
					return fmt.Errorf("%s %s: %w", j.path, key, err)
				}

				j.strings[key] = value
			}

			reporter.BatchDone(j.target, displayPath, batchNumber, batches, len(batch), duration)
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	reporter.Writing()

	for _, j := range jobs {
		if err := writeJSON(j.path, rebuild(j.document, "", j.strings)); err != nil {
			return err
		}
	}

	return writeJSON(statePath, next)
}

// noSymlinks rejects symlink ancestors and final paths, including future output paths.
func noSymlinks(path string) error {
	for p := path; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)

		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not supported: %s", p)
		}

		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}

		if filepath.Dir(p) == p {
			return nil
		}
	}
}

// writeJSON replaces a changed JSON file using a temporary file in the same directory.
// Identical content is left untouched to preserve its modification time.
func writeJSON(path string, value any) error {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(value); err != nil {
		return err
	}

	data := buffer.Bytes()

	if previous, err := os.ReadFile(path); err == nil && bytes.Equal(previous, data) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	f, err := os.CreateTemp(filepath.Dir(path), ".i18n-*.tmp")
	if err != nil {
		return err
	}

	defer os.Remove(f.Name())

	if err := f.Chmod(0644); err != nil {
		f.Close()
		return err
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}

	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(f.Name(), path)
}
