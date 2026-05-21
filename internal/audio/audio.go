package audio

import "os/exec"

var soundFiles = map[string]string{
	"glass": "/System/Library/Sounds/Glass.aiff",
	"ping":  "/System/Library/Sounds/Ping.aiff",
	"funk":  "/System/Library/Sounds/Funk.aiff",
}

func Play(name string) {
	path, ok := soundFiles[name]
	if !ok {
		path = soundFiles["glass"]
	}
	go exec.Command("afplay", path).Run() //nolint:errcheck
}
