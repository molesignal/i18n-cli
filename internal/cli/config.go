// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// config defines the versioned on-disk translation configuration.
type config struct {
	Version  int `json:"version"`
	Provider struct {
		ID       string `json:"id"`
		BaseURL  string `json:"baseUrl"`
		Model    string `json:"model"`
		Prompt   string `json:"prompt"`
		JSONMode *bool  `json:"jsonMode"`
	} `json:"provider"`
	Locale struct {
		Source  string   `json:"source"`
		Targets []string `json:"targets"`
	} `json:"locale"`
	Buckets struct {
		JSON struct {
			Include []string `json:"include"`
			Plurals string   `json:"plurals"`
		} `json:"json"`
	} `json:"buckets"`
	BatchSize int `json:"batchSize"`
}

// loadConfig reads and validates the selected configuration.
// It returns defaults and an empty directory when the implicit file is absent.
func loadConfig(args []string) (config, string, error) {
	path, explicit := "i18n.config.json", false

	for i, arg := range args {
		if arg == "--config" || arg == "-config" {
			if i+1 >= len(args) {
				return config{}, "", fmt.Errorf("--config requires a path")
			}

			path, explicit = args[i+1], true
		} else if strings.HasPrefix(arg, "--config=") || strings.HasPrefix(arg, "-config=") {
			path, explicit = strings.SplitN(arg, "=", 2)[1], true
		}
	}

	c := config{Version: 1, BatchSize: 40}
	c.Locale.Source = "en"
	data, err := os.ReadFile(path)

	if errors.Is(err, os.ErrNotExist) && !explicit {
		return c, "", nil
	}

	if err != nil {
		return c, "", err
	}

	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()

	if err := d.Decode(&c); err != nil {
		return c, "", fmt.Errorf("config: %w", err)
	}

	if err := d.Decode(new(any)); err != io.EOF {
		return c, "", fmt.Errorf("config must contain one JSON object")
	}

	if c.Version != 1 {
		return c, "", fmt.Errorf("unsupported config version %d", c.Version)
	}

	if c.Provider.ID != "" && c.Provider.ID != "openai" && c.Provider.ID != "openai-compatible" {
		return c, "", fmt.Errorf("provider.id must be openai or openai-compatible")
	}

	if len(c.Buckets.JSON.Include) == 0 {
		return c, "", fmt.Errorf("config buckets.json.include is required")
	}

	abs, err := filepath.Abs(path)
	return c, filepath.Dir(abs), err
}
