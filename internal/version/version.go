package version

// Version is the current application version.
const Version = "2.3"

// Commit is the short git hash the binary was built from, injected at build
// time via:
//
//	go build -ldflags "-X github.com/ludo/claudewatcher/internal/version.Commit=$(git rev-parse --short HEAD)"
//
// Empty for a bare `go build`.
var Commit string

// Full returns the version, with the build commit when known: "2.3 (abc1234)".
func Full() string {
	if Commit == "" {
		return Version
	}
	return Version + " (" + Commit + ")"
}
