//go:build !darwin && !linux

// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package progress

import "os"

// terminalWidth selects plain logs on platforms without supported terminal probing.
func terminalWidth(_ *os.File) (int, bool) {
	return 0, false
}
