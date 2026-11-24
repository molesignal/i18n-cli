// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/molesignal/i18n-cli/internal/provider"
	"github.com/molesignal/i18n-cli/internal/translation"
)

// Run resolves configuration and command-line overrides, then starts translation.
func Run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" || args[0] == "-h" {
		fmt.Fprintln(out, "Usage: i18n-cli translate --dir ./locales --source en --target zh-CN,ja-JP,ko-KR\nRun translate --help for options.")
		return nil
	}

	if args[0] != "translate" {
		return fmt.Errorf("unknown command %q", args[0])
	}

	c, configDir, err := loadConfig(args[1:])
	if err != nil {
		return err
	}

	values, err := loadDotEnv(".env")
	if err != nil {
		return err
	}

	defaultDir := "locales"

	if configDir != "" {
		defaultDir = configDir
	}

	flags := flag.NewFlagSet("translate", flag.ContinueOnError)
	flags.SetOutput(out)
	var o translation.Options
	var targets, baseURL, model, instructionsFile, configPath string
	var jsonMode, allowNoKey bool
	var retries int
	var timeout time.Duration
	flags.StringVar(&configPath, "config", "i18n.config.json", "configuration file (paths relative to its directory)")
	flags.StringVar(&o.Dir, "dir", defaultDir, "locale root directory (<dir>/<locale>/**/*.json)")
	flags.StringVar(&o.Source, "source", c.Locale.Source, "source locale directory name (case-sensitive)")
	flags.StringVar(&targets, "target", strings.Join(c.Locale.Targets, ","), "comma-separated target locale directory names")
	flags.IntVar(&o.BatchSize, "batch-size", c.BatchSize, "maximum strings per API request (1-200)")
	flags.BoolVar(&o.DryRun, "dry-run", false, "report pending translations without API calls or writes")
	flags.BoolVar(&o.Force, "force", false, "retranslate all non-empty strings")
	flags.BoolVar(&o.AdoptExisting, "adopt-existing", false, "trust existing translations without a recorded source hash")
	flags.StringVar(&baseURL, "base-url", providerEnv(values, "OPENAI_BASE_URL", fallback(c.Provider.BaseURL, "https://api.openai.com/v1")), "API base URL including /v1 if required")
	flags.StringVar(&model, "model", providerEnv(values, "OPENAI_MODEL", c.Provider.Model), "model ID (or OPENAI_MODEL)")
	flags.StringVar(&instructionsFile, "instructions", "", "UTF-8 glossary/style guidance file")
	flags.StringVar(&o.Plurals, "plurals", c.Buckets.JSON.Plurals, "plural key layout: i18next adds each target language's CLDR plural forms")
	defaultJSONMode := true

	if c.Provider.JSONMode != nil {
		defaultJSONMode = *c.Provider.JSONMode
	}

	flags.BoolVar(&jsonMode, "json-mode", defaultJSONMode, "request response_format=json_object; disable for incompatible providers")
	flags.BoolVar(&allowNoKey, "allow-no-key", false, "allow a local provider without OPENAI_API_KEY")
	flags.IntVar(&retries, "retries", 3, "HTTP 429/5xx retry count (0-6)")
	flags.DurationVar(&timeout, "timeout", 90*time.Second, "per-request timeout")

	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}

		return err
	}

	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}

	if targets != "" {
		for _, t := range strings.Split(targets, ",") {
			o.Targets = append(o.Targets, strings.TrimSpace(t))
		}
	}

	if retries < 0 || retries > 6 || timeout <= 0 {
		return fmt.Errorf("retries must be 0-6 and timeout must be positive")
	}

	o.Include = c.Buckets.JSON.Include
	o.Instructions = c.Provider.Prompt

	if instructionsFile != "" {
		data, err := os.ReadFile(instructionsFile)
		if err != nil {
			return err
		}

		o.Instructions += "\n" + string(data)
	}

	key := providerEnv(values, "OPENAI_API_KEY", "")

	if !o.DryRun && (model == "" || (key == "" && !allowNoKey)) {
		return fmt.Errorf("set OPENAI_MODEL and OPENAI_API_KEY (or --allow-no-key for a local provider)")
	}

	o.Out = out
	return translation.Run(ctx, o, &provider.Client{BaseURL: baseURL, APIKey: key, Model: model, Instructions: o.Instructions, Plurals: o.Plurals == translation.PluralI18next, JSONMode: jsonMode, Retries: retries, HTTP: &http.Client{Timeout: timeout}})
}

// fallback returns value when non-empty, otherwise defaultValue.
func fallback(value, defaultValue string) string {
	if value != "" {
		return value
	}

	return defaultValue
}
