//go:build windows

package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	whKeyboardLL         = 13
	wmKeyDown            = 0x0100
	wmKeyUp              = 0x0101
	wmSysKeyDown         = 0x0104
	wmSysKeyUp           = 0x0105
	inputKeyboard        = 1
	keyeventfKeyUp       = 0x0002
	keyeventfExtendedKey = 0x0001
	llkhfExtended        = 0x01
	// sentinel tags our injected events so hookProc doesn't re-process them.
	sentinel = 0xCA5510C4
	// graceWindow: after a tap, wait this long before emitting Escape, so that
	// a chord key arriving just after Caps release is treated as Ctrl+key.
	graceWindow = 150 * time.Millisecond
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	procSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procSendInput           = user32.NewProc("SendInput")
	procMessageBoxW         = user32.NewProc("MessageBoxW")
)

// kbdllHookStruct mirrors Win32 KBDLLHOOKSTRUCT.
type kbdllHookStruct struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

// keyInput mirrors the Win32 INPUT structure (keyboard variant) on 64-bit Windows.
//
// Layout (40 bytes total):
//
//	Offset  0 │ Type      (4) — INPUT_KEYBOARD = 1
//	Offset  4 │ typePad   (4) — aligns union to 8-byte boundary
//	Offset  8 │ Wvk       (2) — virtual-key code
//	Offset 10 │ Wscan     (2) — hardware scan code
//	Offset 12 │ DwFlags   (4) — KEYEVENTF_* flags
//	Offset 16 │ Time      (4) — timestamp (0 = system time)
//	Offset 20 │ extraPad  (4) — aligns ExtraInfo to 8-byte boundary
//	Offset 24 │ ExtraInfo (8) — sentinel to prevent re-entry
//	Offset 32 │ unionPad  (8) — pads union to MOUSEINPUT size
type keyInput struct {
	Type      uint32
	typePad   uint32
	Wvk       uint16
	Wscan     uint16
	DwFlags   uint32
	Time      uint32
	extraPad  uint32
	ExtraInfo uintptr
	unionPad  [8]byte
}

var (
	hookHandle uintptr
	logWriter  io.Writer
)

// Grace-period state (protected by graceMu).
// After a tap detection, the tap key is deferred for graceWindow so that a
// chord key arriving in that window is handled as modVk+key instead.
var (
	graceMu       sync.Mutex
	gracePending  bool         // tap seen; timer running, no chord key yet
	graceCtrl     bool         // chord key arrived; modVk injected, waiting for key-up
	graceTimer    *time.Timer
	graceKeys     int          // keys currently held in graceCtrl mode
	graceSourceVk uint32       // which dual-role key started the grace period
	graceModVk    uint16       // modifier to inject/release in grace ctrl mode
)

func debugf(format string, args ...any) {
	if logWriter != nil {
		fmt.Fprintf(logWriter, format+"\n", args...)
	}
}

func startGrace(sourceVk uint32, modVk uint16, tapVk uint16) {
	graceMu.Lock()
	gracePending = true
	graceCtrl = false
	graceKeys = 0
	graceSourceVk = sourceVk
	graceModVk = modVk
	graceTimer = time.AfterFunc(graceWindow, func() {
		graceMu.Lock()
		if !gracePending {
			graceMu.Unlock()
			return
		}
		gracePending = false
		graceTimer = nil
		graceMu.Unlock()
		// Fire the deferred tap (down+up).
		buf := [2]keyInput{
			syntheticKey(tapVk, false),
			syntheticKey(tapVk, true),
		}
		procSendInput.Call(2, uintptr(unsafe.Pointer(&buf[0])), unsafe.Sizeof(buf[0]))
		runtime.KeepAlive(buf)
		debugf("  grace timeout: injected tap vk=0x%02X", tapVk)
	})
	graceMu.Unlock()
	debugf("  grace period started (%v) src=0x%02X mod=0x%02X tap=0x%02X", graceWindow, sourceVk, modVk, tapVk)
}

func hookProc(nCode, wParam, lParam uintptr) uintptr {
	if int(nCode) >= 0 {
		ks := (*kbdllHookStruct)(unsafe.Pointer(lParam))
		isSentinel := ks.DwExtraInfo == sentinel
		debugf("hook vk=0x%02X wParam=0x%04X flags=0x%02X sentinel=%v", ks.VkCode, wParam, ks.Flags, isSentinel)

		if !isSentinel {
			isDown := wParam == wmKeyDown || wParam == wmSysKeyDown

			// Non-dual-role keys during grace period.
			if !isDualRoleKey(ks.VkCode) {
				graceMu.Lock()
				switch {
				case gracePending:
					// Try to cancel the timer and enter ctrl mode.
					stopped := graceTimer != nil && graceTimer.Stop()
					graceTimer = nil
					if stopped {
						modVk := graceModVk
						gracePending = false
						graceCtrl = true
						if isDown {
							graceKeys = 1
						}
						graceMu.Unlock()
						if isDown {
							debugf("  grace→ctrl: mod=0x%02X+vk=0x%02X", modVk, ks.VkCode)
							var buf [2]keyInput
							buf[0] = syntheticKey(modVk, false)
							buf[1] = reInjectKey(ks, !isDown)
							procSendInput.Call(2, uintptr(unsafe.Pointer(&buf[0])), unsafe.Sizeof(buf[0]))
							runtime.KeepAlive(buf)
						}
						return 1
					}
					// Timer already fired (tap sent): pass through as normal key.
					gracePending = false
					graceMu.Unlock()

				case graceCtrl:
					modVk := graceModVk
					if isDown {
						graceKeys++
						graceMu.Unlock()
						debugf("  grace ctrl key down vk=0x%02X graceKeys=%d", ks.VkCode, graceKeys)
						buf := [1]keyInput{reInjectKey(ks, false)}
						procSendInput.Call(1, uintptr(unsafe.Pointer(&buf[0])), unsafe.Sizeof(buf[0]))
						runtime.KeepAlive(buf)
					} else {
						graceKeys--
						done := graceKeys <= 0
						if done {
							graceCtrl = false
							graceKeys = 0
						}
						graceMu.Unlock()
						if done {
							debugf("  grace ctrl last key up vk=0x%02X, releasing mod=0x%02X", ks.VkCode, modVk)
							var buf [2]keyInput
							buf[0] = reInjectKey(ks, true)
							buf[1] = syntheticKey(modVk, true)
							procSendInput.Call(2, uintptr(unsafe.Pointer(&buf[0])), unsafe.Sizeof(buf[0]))
							runtime.KeepAlive(buf)
						} else {
							debugf("  grace ctrl key up vk=0x%02X graceKeys=%d", ks.VkCode, graceKeys)
							buf := [1]keyInput{reInjectKey(ks, true)}
							procSendInput.Call(1, uintptr(unsafe.Pointer(&buf[0])), unsafe.Sizeof(buf[0]))
							runtime.KeepAlive(buf)
						}
					}
					return 1

				default:
					graceMu.Unlock()
				}
			}

			// Re-press of the grace-source key during pending grace: fire the
			// deferred tap first, then process this press normally (double-tap path).
			if isDualRoleKey(ks.VkCode) && isDown {
				graceMu.Lock()
				if gracePending && ks.VkCode == graceSourceVk {
					fired := graceTimer == nil || !graceTimer.Stop()
					gracePending = false
					graceTimer = nil
					graceMu.Unlock()
					if !fired {
						// Timer didn't fire yet — inject the tap ourselves.
						dk := dualKeys[0] // graceSourceVk is always the first useGrace key
						for _, d := range dualKeys {
							if d.vk == ks.VkCode {
								dk = d
								break
							}
						}
						buf := [2]keyInput{
							syntheticKey(dk.tapVk, false),
							syntheticKey(dk.tapVk, true),
						}
						procSendInput.Call(2, uintptr(unsafe.Pointer(&buf[0])), unsafe.Sizeof(buf[0]))
						runtime.KeepAlive(buf)
						debugf("  grace cancelled by re-press: injected tap vk=0x%02X", dk.tapVk)
					}
					// If fired==true the timer callback already sent the tap.
				} else {
					graceMu.Unlock()
				}
			}

			suppress, pre, grace := step(ks.VkCode, isDown)
			if suppress {
				// Tap with grace: defer injection through the grace period.
				if grace {
					dk := dualKeys[0]
					for _, d := range dualKeys {
						if d.vk == ks.VkCode {
							dk = d
							break
						}
					}
					startGrace(ks.VkCode, dk.modVk, dk.tapVk)
					return 1
				}

				// Batch all events into one SendInput call — MSDN guarantees
				// events within a single call are never interleaved with other
				// input, ensuring the modifier immediately precedes the key.
				var buf [8]keyInput
				n := 0
				for _, a := range pre {
					buf[n] = syntheticKey(a.vk, a.keyUp)
					debugf("  inject synthetic vk=0x%02X keyUp=%v", a.vk, a.keyUp)
					n++
				}
				if !isDualRoleKey(ks.VkCode) {
					buf[n] = reInjectKey(ks, !isDown)
					debugf("  reinject vk=0x%02X keyUp=%v", ks.VkCode, !isDown)
					n++
				}
				if n > 0 {
					ret, _, lerr := procSendInput.Call(
						uintptr(n),
						uintptr(unsafe.Pointer(&buf[0])),
						unsafe.Sizeof(buf[0]),
					)
					runtime.KeepAlive(buf)
					debugf("  SendInput(%d) = %d err=%v", n, ret, lerr)
				}
				return 1
			}
		}
	}
	r, _, _ := procCallNextHookEx.Call(hookHandle, nCode, wParam, lParam)
	return r
}

func syntheticKey(vk uint16, keyUp bool) keyInput {
	in := keyInput{Type: inputKeyboard, Wvk: vk, ExtraInfo: sentinel}
	if keyUp {
		in.DwFlags = keyeventfKeyUp
	}
	return in
}

func reInjectKey(ks *kbdllHookStruct, keyUp bool) keyInput {
	var flags uint32
	if keyUp {
		flags |= keyeventfKeyUp
	}
	if ks.Flags&llkhfExtended != 0 {
		flags |= keyeventfExtendedKey
	}
	return keyInput{
		Type:      inputKeyboard,
		Wvk:       uint16(ks.VkCode),
		Wscan:     uint16(ks.ScanCode),
		DwFlags:   flags,
		ExtraInfo: sentinel,
	}
}

func main() {
	// LockOSThread ensures the message pump and the hook run on the same OS
	// thread — required by WH_KEYBOARD_LL.
	runtime.LockOSThread()

	// Keystroke logging (stderr and KEYBINDER_LOG) only exists in debug
	// builds — see logwriter_debug.go / logwriter_release.go. Release
	// binaries have no way to log keystrokes, even if the env var is set.
	if debugLoggingEnabled {
		logWriter = os.Stderr
		if path := os.Getenv("KEYBINDER_LOG"); path != "" {
			f, err := os.Create(path)
			if err == nil {
				logWriter = f
				defer f.Close()
			}
		}
	}
	debugf("keybinder debug log started")
	debugf("INPUT size=%d ExtraInfo offset=%d",
		unsafe.Sizeof(keyInput{}),
		unsafe.Offsetof(keyInput{}.ExtraInfo))

	cb := syscall.NewCallback(hookProc)
	h, _, err := procSetWindowsHookExW.Call(whKeyboardLL, cb, 0, 0)
	if h == 0 {
		msg, _ := syscall.UTF16PtrFromString(err.Error())
		title, _ := syscall.UTF16PtrFromString("keybinder")
		procMessageBoxW.Call(0, uintptr(unsafe.Pointer(msg)), uintptr(unsafe.Pointer(title)), 0x10)
		os.Exit(1)
	}
	hookHandle = h
	defer procUnhookWindowsHookEx.Call(hookHandle)

	// MSG is only used as an opaque buffer; 96 bytes covers it on 64-bit Windows.
	var m [96]byte
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m[0])), 0, 0, 0)
		if r == 0 || r == ^uintptr(0) { // WM_QUIT or error
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m[0])))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m[0])))
	}
}
