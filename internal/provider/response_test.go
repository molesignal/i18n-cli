// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCorrectiveRetry verifies wrapped output is retried without relaxing key validation.
func TestCorrectiveRetry(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var request struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}

		var input map[string]string
		if err := json.Unmarshal([]byte(request.Messages[1].Content), &input); err != nil || len(input) != 2 || input["/a"] != "Hello {{name}}" {
			t.Error("expected an unwrapped flat input object")
		}

		content := `{"entries":{"/a":"PRIVATE_VALUE","/b":"B"}}`
		if calls > 1 {
			if !strings.Contains(request.Messages[0].Content, "previous attempt failed") || strings.Contains(request.Messages[0].Content, "PRIVATE_VALUE") {
				t.Error("expected safe corrective instructions")
			}
			content = `{"/a":"Bonjour {{name}}","/b":"B"}`
		}

		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": content}}}})
	}))
	defer server.Close()

	c := Client{BaseURL: server.URL, Model: "test"}
	got, err := c.Translate(context.Background(), "en", "fr", map[string]string{"/a": "Hello {{name}}", "/b": "B"})
	if err != nil || calls != 2 || got["/a"] != "Bonjour {{name}}" {
		t.Fatalf("unexpected retry result: %v calls=%d", err, calls)
	}
}

// TestOutputRetryLimit verifies persistent mismatches produce bounded, useful diagnostics.
func TestOutputRetryLimit(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": `{"wrong":"PRIVATE_VALUE"}`}}}})
	}))
	defer server.Close()

	c := Client{BaseURL: server.URL, Model: "test"}
	_, err := c.Translate(context.Background(), "en", "fr", map[string]string{"/a": "A"})
	if err == nil || calls != 3 {
		t.Fatalf("expected three attempts, got %d: %v", calls, err)
	}

	for _, fragment := range []string{"after 3 attempts", `missing=["/a"]`, `unexpected=["wrong"]`} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("missing diagnostic %q: %v", fragment, err)
		}
	}

	if strings.Contains(err.Error(), "PRIVATE_VALUE") {
		t.Fatal("response values leaked into diagnostics")
	}
}

// TestLiteralPointerKeys verifies keys remain literal even when they contain escaped segments.
func TestLiteralPointerKeys(t *testing.T) {
	entries := map[string]string{"/a~1b/~0name": "Hi {{name}}"}
	if _, err := decodeTranslation(`{"/a~1b/~0name":"Salut {{name}}"}`, entries); err != nil {
		t.Fatal(err)
	}

	if _, err := decodeTranslation(`{"a/b":{"~name":"Salut {{name}}"}}`, entries); err == nil {
		t.Fatal("nested output must not be accepted as an exact key match")
	}
}

// TestPluralGuidanceOnlyWhenEnabled verifies that the plural categories are explained to the
// model only for an i18next plural layout.
func TestPluralGuidanceOnlyWhenEnabled(t *testing.T) {
	if prompt := translationPrompt("en", "ru", "", "", true); !strings.Contains(prompt, "CLDR cardinal plural category of ru") {
		t.Fatalf("plural guidance missing: %s", prompt)
	}

	if prompt := translationPrompt("en", "ru", "", "", false); strings.Contains(prompt, "CLDR") {
		t.Fatalf("plural guidance without plurals: %s", prompt)
	}
}
