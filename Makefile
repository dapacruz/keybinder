.PHONY: all clean

LDFLAGS := -ldflags="-H windowsgui"

all: key-rebinder.exe key-rebinder-arm64.exe

key-rebinder.exe:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $@ .

key-rebinder-arm64.exe:
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o $@ .

# Debug builds: console subsystem so stderr is visible in a cmd window, and
# -tags debug enables keystroke logging (stderr + KEY_REBINDER_LOG). Release
# builds above never get this tag, so they can't log keystrokes at all.
debug-arm64.exe:
	GOOS=windows GOARCH=arm64 go build -tags debug -o $@ .

debug.exe:
	GOOS=windows GOARCH=amd64 go build -tags debug -o $@ .

clean:
	rm -f key-rebinder.exe key-rebinder-arm64.exe debug-arm64.exe debug.exe
