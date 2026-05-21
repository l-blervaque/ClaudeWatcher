package audio_test

import (
	"testing"

	"github.com/ludo/claudewatcher/internal/audio"
)

func TestPlayKnownSoundsDoNotPanic(t *testing.T) {
	audio.Play("glass")
	audio.Play("ping")
	audio.Play("funk")
}

func TestPlayUnknownFallsBackSilently(t *testing.T) {
	audio.Play("unknown")
	audio.Play("")
}
