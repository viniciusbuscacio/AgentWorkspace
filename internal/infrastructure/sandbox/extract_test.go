package sandbox

import (
	"strings"
	"testing"
)

// Ported from AW2's extractPaths cases in path-validator.test.ts.
func TestExtractPaths(t *testing.T) {
	cases := []struct {
		name    string
		command string
		want    []string
	}{
		{"simple absolute path", "cat /etc/hosts", []string{"/etc/hosts"}},
		{"tilde path", "ls ~/Documents", []string{"~/Documents"}},
		{"relative ./", "cat ./src/index.ts", []string{"./src/index.ts"}},
		{"relative ../", "cat ../parent/file.txt", []string{"../parent/file.txt"}},
		{"redirect target", "echo hello > /tmp/output.txt", []string{"/tmp/output.txt"}},
		{"append redirect target", "echo hello >> /tmp/log.txt", []string{"/tmp/log.txt"}},
		{"multiple paths from cp", "cp /src/file.txt /dst/file.txt", []string{"/src/file.txt", "/dst/file.txt"}},
		{"piped command", "cat /etc/hosts | grep localhost > /tmp/result.txt", []string{"/etc/hosts", "/tmp/result.txt"}},
		{"&& command", "cd /tmp && ls /var/log", []string{"/tmp", "/var/log"}},
		{"double-quoted path", `cat "/tmp/my file.txt"`, []string{"/tmp/my file.txt"}},
		{"single-quoted path", "cat '/tmp/spaced file.txt'", []string{"/tmp/spaced file.txt"}},
		{"curl -o target", "curl https://example.com -o /tmp/download.html", []string{"/tmp/download.html"}},
		{"wget -O target", "wget https://example.com -O /tmp/page.html", []string{"/tmp/page.html"}},
		{"windows drive backslash path", `type C:\Users\alice\.ssh\config`, []string{`C:\Users\alice\.ssh\config`}},
		{"windows drive slash path", `type C:/Users/alice/.ssh/config`, []string{`C:/Users/alice/.ssh/config`}},
		{"windows UNC path", `dir \\server\share\folder`, []string{`\\server\share\folder`}},
		{"windows relative backslash path", `type src\renderer\App.tsx`, []string{`src\renderer\App.tsx`}},
		{"windows redirect target", `echo hello > C:\Temp\out.txt`, []string{`C:\Temp\out.txt`}},
		{"windows curl -o target", `curl https://example.com -o C:\Temp\download.html`, []string{`C:\Temp\download.html`}},
	}
	for _, tc := range cases {
		got := ExtractPaths(tc.command)
		for _, want := range tc.want {
			if !contains(got, want) {
				t.Errorf("%s: ExtractPaths(%q) = %v, missing %q", tc.name, tc.command, got, want)
			}
		}
	}
}

func TestExtractPathsNegativesAndDedup(t *testing.T) {
	if got := ExtractPaths("echo hello world"); len(got) != 0 {
		t.Errorf("no-path command extracted %v", got)
	}
	if got := ExtractPaths("curl https://example.com"); contains(got, "https://example.com") {
		t.Errorf("URL extracted as path: %v", got)
	}
	got := ExtractPaths("cat /tmp/file.txt && head /tmp/file.txt")
	count := 0
	for _, path := range got {
		if path == "/tmp/file.txt" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("duplicate path not deduplicated: %v", got)
	}
}

func TestIsSafeNoPathCommand(t *testing.T) {
	cases := []struct {
		command string
		want    bool
	}{
		{"echo hello", true},
		{"date", true},
		{"pwd", true},
		{"env", false},
		{"echo $HOME", false},        // shell expansion
		{"echo hi; rm -rf x", false}, // operators
		{"echo hi | cat", false},
		{"", false},
		{"/bin/echo hi", true}, // basename is allowlisted
	}
	for _, tc := range cases {
		if got := IsSafeNoPathCommand(tc.command); got != tc.want {
			t.Errorf("IsSafeNoPathCommand(%q) = %v, want %v", tc.command, got, tc.want)
		}
	}
}

// Ported from AW2's sandbox-shell.test.ts (decision flow only — execution
// stays in the tools layer).
func TestCheckCommandBlockAll(t *testing.T) {
	check := CheckCommand("echo hello", "", blockAllPolicy())
	if check.Allowed {
		t.Fatal("block_all allowed a command")
	}
	if !strings.Contains(check.Reason, "BLOCK_ALL") {
		t.Errorf("reason = %q, want BLOCK_ALL marker", check.Reason)
	}
}

func TestCheckCommandPermitAll(t *testing.T) {
	if check := CheckCommand("echo hello-sandbox", "", permitAllPolicy()); !check.Allowed {
		t.Errorf("permit_all blocked a plain command: %+v", check)
	}
	check := CheckCommand("cat ~/.ssh/id_rsa", "", permitAllPolicy())
	if check.Allowed {
		t.Fatal("permit_all allowed a built-in sensitive path")
	}
	if !containsSubstring(check.BlockedPaths, ".ssh") {
		t.Errorf("blocked paths = %v, want .ssh entry", check.BlockedPaths)
	}
}

func TestCheckCommandPermitList(t *testing.T) {
	policy := permitListPolicy()
	if check := CheckCommand("ls "+testWorkspace, "", policy); !check.Allowed {
		t.Errorf("workspace folder blocked: %+v", check)
	}
	if check := CheckCommand("ls /etc/hosts", "", policy); check.Allowed || !contains(check.BlockedPaths, "/etc/hosts") {
		t.Errorf("ls /etc/hosts = %+v, want blocked with path", check)
	}
	if check := CheckCommand("ls /tmp", "", policy); !check.Allowed {
		t.Errorf("ls /tmp blocked: %+v", check)
	}
	if check := CheckCommand("ls /tmp/test-sandbox-12345", "", permitListPolicy("/tmp")); !check.Allowed {
		t.Errorf("user-configured folder blocked: %+v", check)
	}
	if check := CheckCommand("cat ~/.ssh/id_rsa", "", policy); check.Allowed || !containsSubstring(check.BlockedPaths, ".ssh") {
		t.Errorf("cat ~/.ssh/id_rsa = %+v, want blocked", check)
	}
	if check := CheckCommand("echo hello", "", policy); !check.Allowed {
		t.Errorf("safe no-path command blocked: %+v", check)
	}
	if check := CheckCommand("env", "", policy); check.Allowed || !strings.Contains(check.Reason, "PERMIT_LIST") {
		t.Errorf("env = %+v, want PERMIT_LIST block", check)
	}
	if check := CheckCommand("echo $HOME", "", policy); check.Allowed || !strings.Contains(check.Reason, "PERMIT_LIST") {
		t.Errorf("echo $HOME = %+v, want PERMIT_LIST block (shell expansion)", check)
	}
	if check := CheckCommand("echo hello", "~/.ssh", permitListPolicy("/tmp")); check.Allowed || !contains(check.BlockedPaths, "~/.ssh") {
		t.Errorf("denied cwd = %+v, want blocked before executing", check)
	}
}

func TestCheckCommandAlwaysDeniedPathsInPermitAll(t *testing.T) {
	if check := CheckCommand("cat ~/.ssh/id_rsa", "", permitAllPolicy()); check.Allowed || !containsSubstring(check.BlockedPaths, ".ssh") {
		t.Errorf("always-denied .ssh = %+v, want blocked even in permit_all", check)
	}
	if check := CheckCommand("ls ~/Documents/", "", permitAllPolicy()); !check.Allowed {
		t.Errorf("ls ~/Documents blocked: %+v", check)
	}
}

func TestCheckCommandEdgeCases(t *testing.T) {
	check := CheckCommand("cp /tmp/safe.txt ~/.ssh/bad.txt", "", permitListPolicy())
	if check.Allowed || !containsSubstring(check.BlockedPaths, ".ssh") {
		t.Errorf("one blocked path must block the entire command: %+v", check)
	}
	if check := CheckCommand("cat ~/.ssh/id_rsa | head > /tmp/out.txt", "", permitAllPolicy()); check.Allowed {
		t.Errorf("pipe+redirect with denied path allowed: %+v", check)
	}
	if check := CheckCommand("cd ~/.ssh", "", permitAllPolicy()); check.Allowed {
		t.Errorf("cd into blocked directory allowed: %+v", check)
	}
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func containsSubstring(list []string, substring string) bool {
	for _, item := range list {
		if strings.Contains(item, substring) {
			return true
		}
	}
	return false
}
