package constants

import "testing"

func TestWindowsBatchLaunchersRequireTheCommandInterpreter(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{`C:\Users\me\AppData\Roaming\npm\codex.cmd`, true},
		{`C:\Tools\CODEX.BAT`, true},
		{`C:\Users\me\.codex\bin\codex.exe`, false},
	}
	for _, test := range tests {
		if got := NeedsShellSpawn(test.path, "windows"); got != test.want {
			t.Errorf("NeedsShellSpawn(%q, windows) = %v, want %v", test.path, got, test.want)
		}
	}
}

func TestPOSIXExecutablesAndScriptsNeverNeedAShell(t *testing.T) {
	tests := []struct {
		path string
		goos string
	}{
		{"/Applications/ChatGPT.app/Contents/Resources/codex", "darwin"},
		{"/usr/local/bin/codex", "linux"},
	}
	for _, test := range tests {
		if NeedsShellSpawn(test.path, test.goos) {
			t.Errorf("NeedsShellSpawn(%q, %q) = true, want false", test.path, test.goos)
		}
	}
}
