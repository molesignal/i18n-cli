// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package translation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Object is a JSON object that keeps its keys in document order, so a translated file lists
// its keys as the source does.
type Object struct {
	Keys   []string
	Values map[string]any
}

// Decode rejects duplicate object keys so no source text is silently discarded.
// Objects decode to *Object, arrays to []any, numbers to json.Number.
func Decode(data []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	v, err := decodeValue(d)
	if err != nil {
		return nil, err
	}

	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("expected one JSON document")
	}

	switch v.(type) {
	case *Object, []any:

	default:
		return nil, fmt.Errorf("JSON root must be an object or array")
	}

	return v, nil
}

// decodeValue recursively decodes one JSON value while rejecting duplicate keys.
func decodeValue(d *json.Decoder) (any, error) {
	t, err := d.Token()
	if err != nil {
		return nil, err
	}

	switch t {
	case json.Delim('{'):
		m := &Object{Values: map[string]any{}}

		for d.More() {
			k, err := d.Token()
			if err != nil {
				return nil, err
			}

			key, ok := k.(string)

			if !ok {
				return nil, fmt.Errorf("invalid object key")
			}

			if _, exists := m.Values[key]; exists {
				return nil, fmt.Errorf("duplicate JSON key %q", key)
			}

			v, err := decodeValue(d)
			if err != nil {
				return nil, err
			}

			m.Keys = append(m.Keys, key)
			m.Values[key] = v
		}

		_, err := d.Token()
		return m, err

	case json.Delim('['):
		a := []any{}

		for d.More() {
			v, err := decodeValue(d)
			if err != nil {
				return nil, err
			}

			a = append(a, v)
		}

		_, err := d.Token()
		return a, err

	default:
		return t, nil
	}
}

// pointer escapes a single reference token for use in a JSON Pointer.
func pointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

// Strings indexes every string leaf by its JSON Pointer, including array elements.
func Strings(v any) map[string]string {
	out := map[string]string{}
	var walk func(any, string)

	walk = func(v any, p string) {
		switch x := v.(type) {
		case string:
			out[p] = x

		case *Object:
			for _, k := range x.Keys {
				walk(x.Values[k], p+"/"+pointer(k))
			}

		case []any:
			for i, child := range x {
				walk(child, p+"/"+strconv.Itoa(i))
			}
		}
	}

	walk(v, "")
	return out
}

// rebuild copies the source structure and replaces string leaves with translations.
func rebuild(v any, p string, translated map[string]string) any {
	switch x := v.(type) {
	case string:
		return translated[p]

	case *Object:
		out := &Object{Keys: append([]string(nil), x.Keys...), Values: make(map[string]any, len(x.Keys))}

		for _, k := range x.Keys {
			out.Values[k] = rebuild(x.Values[k], p+"/"+pointer(k), translated)
		}

		return out

	case []any:
		out := make([]any, len(x))

		for i, child := range x {
			out[i] = rebuild(child, p+"/"+strconv.Itoa(i), translated)
		}

		return out

	default:
		return v
	}
}

// encode writes value as JSON indented by two spaces with a final newline. Objects keep their
// key order; HTML characters are written as they are.
func encode(value any) ([]byte, error) {
	var compact bytes.Buffer
	if err := writeValue(&compact, value); err != nil {
		return nil, err
	}

	var out bytes.Buffer
	if err := json.Indent(&out, compact.Bytes(), "", "  "); err != nil {
		return nil, err
	}

	out.WriteByte('\n')
	return out.Bytes(), nil
}

// writeValue writes value as compact JSON.
func writeValue(b *bytes.Buffer, value any) error {
	switch x := value.(type) {
	case *Object:
		b.WriteByte('{')

		for i, k := range x.Keys {
			if i > 0 {
				b.WriteByte(',')
			}

			if err := writeValue(b, k); err != nil {
				return err
			}

			b.WriteByte(':')

			if err := writeValue(b, x.Values[k]); err != nil {
				return err
			}
		}

		b.WriteByte('}')

	case []any:
		b.WriteByte('[')

		for i, item := range x {
			if i > 0 {
				b.WriteByte(',')
			}

			if err := writeValue(b, item); err != nil {
				return err
			}
		}

		b.WriteByte(']')

	default:
		var one bytes.Buffer
		encoder := json.NewEncoder(&one)
		encoder.SetEscapeHTML(false)

		if err := encoder.Encode(x); err != nil {
			return err
		}

		b.Write(bytes.TrimRight(one.Bytes(), "\n"))
	}

	return nil
}

// ICU plural/select messages require a parser and are deliberately rejected in v1.
var complexICU = regexp.MustCompile(`\{\s*[^{} ,]+\s*,\s*(plural|select|selectordinal)\s*,`)

// A printf directive takes no space flag: "100% of" and "10 % des" are prose, not "% o" or "% d".
var tokens = regexp.MustCompile(`\{\{[^{}]+\}\}|\$\{[^{}]+\}|\{[A-Za-z_][A-Za-z0-9_.-]*\}|%(?:\[[0-9]+\]|[0-9]+\$)?[-+#0]*[0-9]*(?:\.[0-9]+)?[a-zA-Z%]|</?[A-Za-z][^<>]*>`)

// Validate rejects blank translations and changes to placeholder or HTML tag counts.
func Validate(source, target string) error {
	if source != "" && strings.TrimSpace(target) == "" {
		return fmt.Errorf("empty translation")
	}

	a, b := tokens.FindAllString(source, -1), tokens.FindAllString(target, -1)
	sort.Strings(a)
	sort.Strings(b)

	if strings.Join(a, "\x00") != strings.Join(b, "\x00") {
		return fmt.Errorf("placeholders or HTML tags changed")
	}

	return nil
}
