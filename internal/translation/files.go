// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package translation

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// sourceFile identifies a source document and the path surrounding its locale token.
type sourceFile struct{ path, before, after string }

// discover expands locale-aware include patterns into sorted, unique source files.
// Each result retains the path segments needed to derive its target locale path.
func discover(root, source string, patterns []string) ([]sourceFile, error) {
	var files []sourceFile
	seen := map[string]bool{}

	for _, pattern := range patterns {
		pattern = filepath.ToSlash(pattern)

		if filepath.IsAbs(pattern) || strings.Contains(pattern, "\\") || strings.Contains(pattern, "../") || strings.Count(pattern, "[locale]") != 1 {
			return nil, fmt.Errorf("include must be a relative path with exactly one [locale]: %q", pattern)
		}

		var expression strings.Builder
		expression.WriteString("^")

		for i := 0; i < len(pattern); {
			switch {
			case strings.HasPrefix(pattern[i:], "[locale]"):
				expression.WriteString("(" + regexp.QuoteMeta(source) + ")")
				i += len("[locale]")

			case strings.HasPrefix(pattern[i:], "**/"):
				expression.WriteString("(?:.*/)?")
				i += 3

			case strings.HasPrefix(pattern[i:], "**"):
				expression.WriteString(".*")
				i += 2

			case pattern[i] == '*':
				expression.WriteString("[^/]*")
				i++

			case pattern[i] == '?':
				expression.WriteString("[^/]")
				i++

			default:
				expression.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
				i++
			}
		}

		expression.WriteString("$")
		matcher, err := regexp.Compile(expression.String())
		if err != nil {
			return nil, err
		}

		sourcePattern := strings.Replace(pattern, "[locale]", source, 1)
		first := strings.IndexAny(sourcePattern, "*?")
		prefix := sourcePattern

		if first >= 0 {
			prefix = sourcePattern[:first]
		}

		base := filepath.Join(root, filepath.FromSlash(filepath.Dir(prefix)))

		if strings.HasSuffix(prefix, "/") {
			base = filepath.Join(root, filepath.FromSlash(prefix))
		}

		if err := noSymlinks(base); err != nil {
			return nil, err
		}

		count := 0

		err = filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if entry.Type()&fs.ModeSymlink != 0 {
				return fmt.Errorf("symlinks are not supported: %s", path)
			}

			if entry.IsDir() {
				return nil
			}

			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}

			rel = filepath.ToSlash(rel)
			indices := matcher.FindStringSubmatchIndex(rel)

			if indices == nil {
				return nil
			}

			if !strings.HasSuffix(rel, ".json") {
				return fmt.Errorf("include matched non-JSON file %s", rel)
			}

			count++

			if !seen[rel] {
				files = append(files, sourceFile{path: rel, before: rel[:indices[2]], after: rel[indices[3]:]})
				seen[rel] = true
			}

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("include %q: %w", pattern, err)
		}

		if count == 0 {
			return nil, fmt.Errorf("include %q matched no source JSON files", pattern)
		}
	}

	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	return files, nil
}
