# key-rebinder

A tiny Windows background utility, written in Go, that turns **Caps Lock** and
**Enter** into dual-role keys:

| Key       | Tap             | Held (as modifier) |
|-----------|-----------------|---------------------|
| Caps Lock | Escape          | Left Ctrl           |
| Enter     | Enter            | Right Ctrl          |

It works system-wide via a low-level Windows keyboard hook
(`WH_KEYBOARD_LL`) — no drivers, no admin rights, no config file.

## How it works

`main.go` installs a global low-level keyboard hook and feeds every physical
key event through `step()` in `logic.go`, which decides whether to suppress
the real event and what synthetic key(s) to inject instead
(`SendInput`). Injected events are tagged with a sentinel value in
`dwExtraInfo` so the hook can tell its own synthetic input apart from real
keystrokes and avoid re-processing them.

- **Tap** a dual-role key (press and release with nothing else pressed) →
  its `tapVk` is injected (Escape for Caps Lock, Enter for Enter).
- **Hold** a dual-role key and press another key → the dual-role key's
  `modVk` (Left/Right Ctrl) is injected as a held modifier for as long as
  any chord key is down.
- **Caps Lock** additionally uses a **grace period** (150ms, see
  `graceWindow` in `main.go`): after releasing a tap, the Escape injection
  is briefly deferred so that a chord key arriving just after release is
  still treated as `Ctrl+key` instead of `Escape` followed by the bare key.
  A quick double-tap or an unrelated key press during that window resolves
  the deferred tap correctly either way.

## Building

Requires Go (see `go.mod` for the minimum version) and cross-compiles from
any platform since it only needs `GOOS=windows`.

```sh
make            # builds key-rebinder.exe (amd64) and key-rebinder-arm64.exe
make clean      # removes built binaries
```

Release binaries are built with `-H windowsgui` (no console window) and
never include keystroke logging, regardless of environment variables.

### Debug builds

```sh
make debug.exe        # amd64, console subsystem, -tags debug
make debug-arm64.exe  # arm64, console subsystem, -tags debug
```

Debug builds keep a visible console (so `stderr` is visible) and compile in
keystroke logging via the `debug` build tag (`logwriter_debug.go` /
`logwriter_release.go`). With a debug build, set `KEY_REBINDER_LOG=<path>`
to log to a file instead of stderr.

## Running

Just run the built `.exe` — no installation or admin privileges required.
Exit by killing the process (e.g. via Task Manager), since it runs headless
with no tray icon or window.

## Testing

```sh
go test ./...
```

`logic_test.go` covers the tap/hold state machine in `logic.go` (chording,
grace-period transitions, double-tap, etc.) without touching the real
Windows hook.
