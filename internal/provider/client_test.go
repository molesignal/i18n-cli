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

// TestCompatibleAPIAndRetry verifies request authentication, prompt expansion, and retries.
func TestCompatibleAPIAndRetry(t *testing.T) {
	calls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++

		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Error("bad endpoint/auth")
		}

		var body map[string]any

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}

		messages := body["messages"].([]any)

		if !strings.Contains(messages[0].(map[string]any)["content"].(string), "from en to ja") {
			t.Error("prompt placeholders not expanded")
		}

		if calls == 1 {
			w.WriteHeader(429)
			return
		}

		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": `{"/a":"こんにちは {{name}}"}`}}}})
	}))

	defer server.Close()
	c := Client{BaseURL: server.URL + "/v1", APIKey: "secret", Model: "test", JSONMode: true, Retries: 1, Instructions: "from {source} to {target}"}
	got, err := c.Translate(context.Background(), "en", "ja", map[string]string{"/a": "Hello {{name}}"})

	if err != nil || got["/a"] != "こんにちは {{name}}" || calls != 2 {
		t.Fatalf("%v %v calls=%d", got, err, calls)
	}
}

// TestMalformedResponses verifies rejection of truncated or structurally invalid output.
func TestMalformedResponses(t *testing.T) {
	for _, body := range []string{
		`{"choices":[{"finish_reason":"length","message":{"content":"{}"}}]}`,
		`{"choices":[{"finish_reason":"stop","message":{"content":"{\"/a\":\"Hi\"}"}}]}`,
		`{"choices":[{"finish_reason":"stop","message":{"content":"{\"wrong\":\"Hi {{name}}\"}"}}]}`,
		`{"choices":[{"finish_reason":"stop","message":{"content":"{\"/a\":\"Hi {{name}}\",\"/a\":\"Hi {{name}}\"}"}}]}`,
	} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
			defer server.Close()
			c := Client{BaseURL: server.URL, Model: "test"}

			if _, err := c.Translate(context.Background(), "en", "ja", map[string]string{"/a": "Hi {{name}}"}); err == nil {
				t.Fatal("invalid response accepted")
			}
		})
	}
}

// TestRedirectDoesNotLeakKey verifies that authenticated requests never follow redirects.
func TestRedirectDoesNotLeakKey(t *testing.T) {
	reached := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 307) }))
	defer source.Close()
	c := Client{BaseURL: source.URL, APIKey: "secret"}

	if _, err := c.Translate(context.Background(), "en", "ja", map[string]string{}); err == nil || reached {
		t.Fatal("redirect followed")
	}
}
