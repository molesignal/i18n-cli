// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client calls an OpenAI-compatible API with configurable transport and retry behavior.
type Client struct {
	BaseURL, APIKey, Model, Instructions string
	HTTP                                 *http.Client
	JSONMode                             bool
	// Plurals tells the model that keys ending in a CLDR plural category are that plural form.
	Plurals bool
	Retries int
}

// translateAttempt sends one batch with optional response-format correction guidance.
// HTTP 429 and 5xx responses are retried with bounded exponential backoff.
func (c *Client) translateAttempt(ctx context.Context, source, target string, entries map[string]string, correction string) (map[string]string, error) {
	u, err := url.Parse(c.BaseURL)

	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return nil, fmt.Errorf("invalid API base URL (expected http(s) URL without credentials, query or fragment)")
	}

	input, _ := json.Marshal(entries)

	body := map[string]any{
		"model": c.Model,
		"messages": []map[string]string{
			{"role": "system", "content": translationPrompt(source, target, c.Instructions, correction, c.Plurals)},
			{"role": "user", "content": string(input)},
		},
	}

	if c.JSONMode {
		body["response_format"] = map[string]string{"type": "json_object"}
	}

	payload, _ := json.Marshal(body)
	client := c.HTTP

	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}

	// Never forward API credentials through redirects.
	copyClient := *client
	copyClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }

	for attempt := 0; attempt <= c.Retries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/chat/completions", bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}

		req.Header.Set("Content-Type", "application/json")

		if c.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.APIKey)
		}

		resp, err := copyClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("API request failed: %w", err)
		}

		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20+1))
		resp.Body.Close()

		if readErr != nil {
			return nil, fmt.Errorf("read API response: %w", readErr)
		}

		if len(data) > 8<<20 {
			return nil, fmt.Errorf("API response exceeds 8 MiB")
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			if attempt < c.Retries {
				delay := time.Second * time.Duration(1<<attempt)
				timer := time.NewTimer(delay)

				select {
				case <-ctx.Done():
					timer.Stop()
					return nil, ctx.Err()

				case <-timer.C:
				}

				continue
			}
		}

		if resp.StatusCode != http.StatusOK {
			// Error bodies can echo credentials or private strings; do not log them.
			return nil, fmt.Errorf("API returned HTTP %d", resp.StatusCode)
		}

		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
				Finish string `json:"finish_reason"`
			} `json:"choices"`
		}

		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("invalid API response JSON")
		}

		if len(result.Choices) != 1 || result.Choices[0].Finish != "stop" {
			return nil, fmt.Errorf("API did not return one complete response (expected finish_reason=stop)")
		}

		return decodeTranslation(result.Choices[0].Message.Content, entries)
	}

	return nil, fmt.Errorf("API retries exhausted")
}
