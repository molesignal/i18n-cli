# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 MoleSignal Authors

.PHONY: build test fmt
build:
	go build -o bin/molesignal-i18n ./cmd/molesignal-i18n
test:
	go vet ./...
	go test -race ./...
fmt:
	gofmt -w cmd internal
