package version

import "testing"

func TestFull(t *testing.T) {
	orig := Commit
	t.Cleanup(func() { Commit = orig })

	Commit = ""
	if got := Full(); got != Version {
		t.Errorf("Full() without commit = %q, want %q", got, Version)
	}
	Commit = "abc1234"
	want := Version + " (abc1234)"
	if got := Full(); got != want {
		t.Errorf("Full() with commit = %q, want %q", got, want)
	}
}
