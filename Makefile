.PHONY: all clean

LDFLAGS := -ldflags="-H windowsgui"

all: key-rebinder.exe key-rebinder-arm64.exe

key-rebinder.exe:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $@ .

key-rebinder-arm64.exe:
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o $@ .

# Debug builds: console subsystem so stderr is visible in a cmd window.
debug-arm64.exe:
	GOOS=windows GOARCH=arm64 go build -o $@ .

debug.exe:
	GOOS=windows GOARCH=amd64 go build -o $@ .

clean:
	rm -f key-rebinder.exe key-rebinder-arm64.exe debug-arm64.exe debug.exe
