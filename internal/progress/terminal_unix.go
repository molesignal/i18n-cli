//go:build darwin || linux

// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 MoleSignal Authors

package progress

import (
	"os"
	"syscall"
	"unsafe"
)

// terminalWidth queries the output descriptor, rejecting redirected files and devices.
func terminalWidth(file *os.File) (int, bool) {
	var size struct{ rows, columns, x, y uint16 }
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&size)))
	if errno != 0 {
		return 0, false
	}

	width := int(size.columns)
	if width == 0 {
		width = 80
	}

	return width, true
}
