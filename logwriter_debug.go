//go:build windows && debug

package main

// debugLoggingEnabled gates all keystroke logging (stderr and KEYBINDER_LOG).
// Only set for debug builds (see Makefile's -tags debug) so release binaries
// can never be made to log keystrokes.
const debugLoggingEnabled = true
