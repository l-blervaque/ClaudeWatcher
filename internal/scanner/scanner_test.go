package scanner

import "testing"

func TestSessionIDFromCmdline(t *testing.T) {
	const uuid = "feb19cc6-df85-43fc-a894-deca67b44acb"
	cases := []struct {
		name    string
		cmdline string
		want    string
	}{
		{"plain resume", "claude --resume " + uuid, uuid},
		{"abs path resume", "/Users/ludo/.local/bin/claude --resume " + uuid, uuid},
		{"equals form", "claude --resume=" + uuid, uuid},
		{
			// cmux wraps a long --settings JSON blob before --resume; the uuid is
			// still recovered from the trailing argument.
			"resume after settings blob",
			`/Users/ludo/.local/bin/claude --settings {"hooks":{"Stop":[]}} --resume ` + uuid,
			uuid,
		},
		{"fresh session, no resume", "claude", ""},
		{"resume without value", "claude --resume", ""},
		{"resume with non-uuid value", "claude --resume not-a-uuid", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sessionIDFromCmdline(c.cmdline); got != c.want {
				t.Errorf("sessionIDFromCmdline(%q) = %q, want %q", c.cmdline, got, c.want)
			}
		})
	}
}

func TestIsSessionProc(t *testing.T) {
	const uuid = "feb19cc6-df85-43fc-a894-deca67b44acb"
	cases := []struct {
		name    string
		cmdline string
		want    bool
	}{
		{"resumed session", "claude --resume " + uuid, true},
		{"fresh session", "claude", true},
		{"abs path session", "/Users/ludo/.local/bin/claude --resume " + uuid, true},
		{"daemon", "claude daemon run", false},
		{"abs path daemon", "/Users/ludo/.local/bin/claude daemon", false},
		{"headless -p", "claude -p \"summarize\"", false},
		{"headless --print", "claude --print \"summarize\"", false},
		{"prompt containing daemon word is not the daemon", "claude --resume " + uuid + " restart the daemon", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isSessionProc(c.cmdline); got != c.want {
				t.Errorf("isSessionProc(%q) = %v, want %v", c.cmdline, got, c.want)
			}
		})
	}
}

func TestSockRE(t *testing.T) {
	const arg = "/tmp/cc-daemon-501/e0d9d869/pty/1fe22414.sock"
	m := sockRE.FindStringSubmatch(arg)
	if m == nil {
		t.Fatalf("sockRE did not match %q", arg)
	}
	if m[1] != "1fe22414" {
		t.Errorf("sockRE prefix = %q, want %q", m[1], "1fe22414")
	}
}
