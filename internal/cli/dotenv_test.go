// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDotEnvParsing verifies supported dotenv syntax without shell expansion.
func TestDotEnvParsing(t *testing.T) {
	tests := []struct{ raw, want string }{
		{"", ""},
		{"value # comment", "value"},
		{"value#suffix", "value#suffix"},
		{`"quoted value" # comment`, "quoted value"},
		{`'${TOKEN} $(command)'`, "${TOKEN} $(command)"},
		{`"escaped\tvalue"`, "escaped\tvalue"},
		{`"escaped\"quote"`, "escaped\"quote"},
	}

	for _, tt := range tests {
		got, err := dotenvValue(tt.raw)
		if err != nil || got != tt.want {
			t.Fatalf("unexpected parsed value for test input %q", tt.raw)
		}
	}

	path := filepath.Join(t.TempDir(), ".env")
	data := "\ufeff# comment\r\nexport OPENAI_API_KEY='file-key'\r\nOPENAI_BASE_URL = https://api.deepseek.com # comment\nOPENAI_MODEL=old\nOPENAI_MODEL=deepseek-flash\nOTHER=${IGNORED}\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}

	values, err := loadDotEnv(path)
	if err != nil || len(values) != 3 || values["OPENAI_MODEL"] != "deepseek-flash" || values["OPENAI_API_KEY"] != "file-key" || values["OPENAI_BASE_URL"] != "https://api.deepseek.com" {
		t.Fatal("dotenv values were not loaded correctly")
	}
}

// TestDotEnvMissingAndRedaction verifies optional files and errors that do not leak values.
func TestDotEnvMissingAndRedaction(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if values, err := loadDotEnv(path); err != nil || len(values) != 0 {
		t.Fatal("missing dotenv must be optional")
	}

	for _, value := range []string{`"secret-unclosed`, `'secret-unclosed`, `"secret" trailing`, `"secret\q"`} {
		if err := os.WriteFile(path, []byte("# header\nOPENAI_API_KEY="+value), 0600); err != nil {
			t.Fatal(err)
		}

		_, err := loadDotEnv(path)
		if err == nil || !strings.Contains(err.Error(), "line 2") || strings.Contains(err.Error(), "secret") {
			t.Fatalf("expected a redacted line-number error, got %v", err)
		}
	}
}

// TestProviderEnvironmentPrecedence verifies process, dotenv, and fallback priority.
func TestProviderEnvironmentPrecedence(t *testing.T) {
	values := map[string]string{"OPENAI_MODEL": "file-model"}
	t.Setenv("OPENAI_MODEL", "process-model")
	if providerEnv(values, "OPENAI_MODEL", "config-model") != "process-model" {
		t.Fatal("process environment did not take precedence")
	}

	t.Setenv("OPENAI_MODEL", "")
	if providerEnv(values, "OPENAI_MODEL", "config-model") != "file-model" {
		t.Fatal("dotenv value did not take precedence over configuration")
	}

	values["OPENAI_MODEL"] = ""
	if providerEnv(values, "OPENAI_MODEL", "config-model") != "config-model" {
		t.Fatal("empty values should fall back to configuration")
	}
}

// TestCLIDotEnvRequest verifies dotenv credentials reach the API and CLI flags override them.
func TestCLIDotEnvRequest(t *testing.T) {
	for _, override := range []bool{false, true} {
		t.Run(map[bool]string{false: "dotenv", true: "flags"}[override], func(t *testing.T) {
			for _, key := range []string{"OPENAI_API_KEY", "OPENAI_BASE_URL", "OPENAI_MODEL"} {
				t.Setenv(key, "")
			}

			expectedModel := "deepseek-flash"
			if override {
				expectedModel = "flag-model"
			}

			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var body struct {
					Model string `json:"model"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}

				if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer file-key" || body.Model != expectedModel {
					t.Error("unexpected provider endpoint, credentials, or model")
				}

				io.WriteString(w, `{"choices":[{"finish_reason":"stop","message":{"content":"{\"/hello\":\"Bonjour\"}"}}]}`)
			}))
			defer server.Close()

			dir, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Chdir(dir)

			if err := os.MkdirAll("locales/en", 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile("locales/en/app.json", []byte(`{"hello":"Hello"}`), 0644); err != nil {
				t.Fatal(err)
			}

			fileURL := server.URL
			if override {
				fileURL = "https://invalid.example"
			}

			data := "OPENAI_API_KEY=file-key\nOPENAI_BASE_URL=" + fileURL + "\nOPENAI_MODEL=deepseek-flash\n"
			if err := os.WriteFile(".env", []byte(data), 0600); err != nil {
				t.Fatal(err)
			}

			args := []string{"translate", "--source", "en", "--target", "fr"}
			if override {
				args = append(args, "--base-url", server.URL, "--model", expectedModel)
			}

			if err := Run(context.Background(), args, io.Discard); err != nil {
				t.Fatal(err)
			}

			if calls != 1 || os.Getenv("OPENAI_API_KEY") != "" {
				t.Fatal("expected one API call and an unchanged process environment")
			}
		})
	}
}
