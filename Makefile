# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 MoleSignal Authors

.PHONY: build test fmt
build:
	go build -o bin/i18n-cli ./cmd/i18n-cli
test:
	go vet ./...
	go test -race ./...
fmt:
	gofmt -w cmd internal
