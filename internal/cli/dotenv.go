// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// loadDotEnv reads supported provider settings without changing the process environment.
// A missing file is optional; malformed entries report line numbers without values.
func loadDotEnv(path string) (map[string]string, error) {
	values := map[string]string{}
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return values, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read .env: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "export ") || strings.HasPrefix(line, "export\t") {
			line = strings.TrimSpace(line[len("export"):])
		}

		key, raw, found := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if key != "OPENAI_API_KEY" && key != "OPENAI_BASE_URL" && key != "OPENAI_MODEL" {
			continue
		}

		if !found {
			return nil, fmt.Errorf("invalid .env assignment at line %d", lineNumber)
		}

		value, err := dotenvValue(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("invalid .env value at line %d", lineNumber)
		}

		values[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("could not read .env near line %d", lineNumber+1)
	}

	return values, nil
}

// dotenvValue parses one single-line value without expanding variables or executing commands.
func dotenvValue(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}

	if raw[0] != '\'' && raw[0] != '"' {
		for i, char := range raw {
			if char == '#' && (i == 0 || raw[i-1] == ' ' || raw[i-1] == '\t') {
				return strings.TrimSpace(raw[:i]), nil
			}
		}

		return raw, nil
	}

	quote := raw[0]
	for i := 1; i < len(raw); i++ {
		if quote == '"' && raw[i] == '\\' {
			i++
			continue
		}

		if raw[i] != quote {
			continue
		}

		rest := strings.TrimSpace(raw[i+1:])
		if rest != "" && !strings.HasPrefix(rest, "#") {
			return "", fmt.Errorf("unexpected trailing text")
		}

		if quote == '\'' {
			return raw[1:i], nil
		}

		value, err := strconv.Unquote(raw[:i+1])
		if err != nil {
			return "", fmt.Errorf("invalid quoted value")
		}

		return value, nil
	}

	return "", fmt.Errorf("unterminated quoted value")
}

// providerEnv resolves non-empty process settings before dotenv values and defaults.
func providerEnv(values map[string]string, key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	if value := values[key]; value != "" {
		return value
	}

	return defaultValue
}
