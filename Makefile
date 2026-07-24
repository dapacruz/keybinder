.PHONY: all clean

LDFLAGS := -ldflags="-H windowsgui"

all: keybinder.exe keybinder-arm64.exe

keybinder.exe:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $@ .

keybinder-arm64.exe:
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o $@ .

# Debug builds: console subsystem so stderr is visible in a cmd window, and
# -tags debug enables keystroke logging (stderr + KEYBINDER_LOG). Release
# builds above never get this tag, so they can't log keystrokes at all.
debug-arm64.exe:
	GOOS=windows GOARCH=arm64 go build -tags debug -o $@ .

debug.exe:
	GOOS=windows GOARCH=amd64 go build -tags debug -o $@ .

clean:
	rm -f keybinder.exe keybinder-arm64.exe debug-arm64.exe debug.exe
