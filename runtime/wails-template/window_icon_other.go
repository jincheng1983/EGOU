//go:build !windows
// +build !windows

package main

import "unsafe"

func setWindowIcon(hwnd uintptr) {}

func getWindowHWND(nativeWindow unsafe.Pointer) uintptr {
	return 0
}
