//go:build windows
// +build windows

package main

import (
	"log"
	"syscall"
	"unsafe"
)

var (
	user32DLL            = syscall.NewLazyDLL("user32.dll")
	kernel32DLL          = syscall.NewLazyDLL("kernel32.dll")
	procSendMessageW     = user32DLL.NewProc("SendMessageW")
	procLoadImageW       = user32DLL.NewProc("LoadImageW")
	procGetModuleHandleW = kernel32DLL.NewProc("GetModuleHandleW")
	procGetSystemMetrics = user32DLL.NewProc("GetSystemMetrics")
)

const (
	WM_SETICON    = 0x0080
	ICON_SMALL    = 0
	ICON_BIG      = 1
	IMAGE_ICON    = 1
	LR_DEFAULTSIZE = 0x00000040
	LR_SHARED     = 0x00008000
	SM_CXICON     = 11
	SM_CYICON     = 12
	SM_CXSMICON   = 49
	SM_CYSMICON   = 50
)

func setWindowIcon(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	hModule, _, _ := procGetModuleHandleW.Call(0)
	if hModule == 0 {
		log.Printf("[runtime] GetModuleHandleW failed")
		return
	}
	cxSm, _, _ := procGetSystemMetrics.Call(uintptr(SM_CXSMICON))
	cySm, _, _ := procGetSystemMetrics.Call(uintptr(SM_CYSMICON))
	cxBig, _, _ := procGetSystemMetrics.Call(uintptr(SM_CXICON))
	cyBig, _, _ := procGetSystemMetrics.Call(uintptr(SM_CYICON))

	hIconSm, _, _ := procLoadImageW.Call(
		hModule,
		uintptr(1),
		uintptr(IMAGE_ICON),
		cxSm, cySm,
		LR_SHARED,
	)
	hIconBig, _, _ := procLoadImageW.Call(
		hModule,
		uintptr(1),
		uintptr(IMAGE_ICON),
		cxBig, cyBig,
		LR_SHARED,
	)

	if hIconSm == 0 && hIconBig == 0 {
		log.Printf("[runtime] LoadImageW failed (no icon resource in exe)")
		return
	}
	if hIconSm != 0 {
		procSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, hIconSm)
	}
	if hIconBig != 0 {
		procSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, hIconBig)
	}
	if hIconSm == 0 {
		procSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, hIconBig)
	}
	if hIconBig == 0 {
		procSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, hIconSm)
	}
	log.Printf("[runtime] 窗口图标已通过 Win32 API 设置 (small=%v big=%v)", hIconSm != 0, hIconBig != 0)
}

func getWindowHWND(nativeWindow unsafe.Pointer) uintptr {
	return uintptr(nativeWindow)
}
