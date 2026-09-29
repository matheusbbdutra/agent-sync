package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestBashBin(t *testing.T) {
	cases := []struct {
		cmd     string
		wantBin string
		wantOK  bool
	}{
		{"ls -la", "ls", true},
		{"cat foo.go", "cat", true},
		{"/usr/bin/find . -name '*.go'", "find", true},
		{"grep -r error", "grep", true},
		// v2 whitelist — dev tooling
		{"npm install", "npm", true},
		{"git status", "git", true},
		{"go test ./...", "go", true},
		{"pytest tests/", "pytest", true},
		{"docker ps", "docker", true},
		// fora da whitelist
		{"kubectl get pods", "kubectl", true}, // na whitelist
		{"curl https://example.com", "curl", false},
		{"ssh user@host", "ssh", false},
		{"", "", false},
		{"   ", "", false},
	}
	for _, c := range cases {
		bin, ok := bashBin(c.cmd)
		if bin != c.wantBin || ok != c.wantOK {
			t.Errorf("bashBin(%q)=(%q,%v), want (%q,%v)", c.cmd, bin, ok, c.wantBin, c.wantOK)
		}
	}
}

func TestBashUnsafeReason(t *testing.T) {
	cases := []struct {
		cmd     string
		wantHas bool
	}{
		{"ls -la", false},
		{"cat foo.go", false},
		{"ls | grep foo", true},          // pipe
		{"cat foo.go && echo done", true}, // &&
		{"echo $VAR", true},              // $ expansion
		{"sudo apt install", true},        // sudo
		{"curl https://example.com", true}, // curl
		// v2: git/go/cargo/npm agora NA whitelist (não em unsafe)
		{"git status", false},
		{"npm install", false},
		{"go test ./...", false},
		{"cargo test", false},
		// pip/pip3 ainda unsafe (state-changing install)
		{"pip install foo", true},
	}
	for _, c := range cases {
		reason := bashUnsafeReason(c.cmd)
		got := reason != ""
		if got != c.wantHas {
			t.Errorf("bashUnsafeReason(%q)=%q, wantHas=%v", c.cmd, reason, c.wantHas)
		}
	}
}

func TestCompressBashOutputPassesVerbatim(t *testing.T) {
	// comandos não-seguros passam verbatim
	res := CompressBashOutput("ssh user@host", "ssh output\n")
	if res.Applied {
		t.Errorf("ssh should fail-open, but Applied=true: %+v", res)
	}
	if res.Output != "ssh output\n" {
		t.Error("ssh output should pass verbatim")
	}
}

func TestCompressBashOutputCompressesLs(t *testing.T) {
	out := "total 12\nfoo.go\nbar.go\nbaz.go\nqux.go"
	res := CompressBashOutput("ls -la", out)
	if !res.Applied {
		t.Errorf("ls should be compressed, got Applied=false: %+v", res)
	}
	if strings.Contains(res.Output, "total 12") {
		t.Errorf("ls compressor should remove 'total' line, got %q", res.Output)
	}
	if !strings.Contains(res.Output, "foo.go") {
		t.Errorf("ls compressor should keep foo.go, got %q", res.Output)
	}
}

func TestCompressBashOutputHeadTailTrigger(t *testing.T) {
	// output grande (> bashKeepHead + bashKeepTail) deve ser omitido no meio.
	// 50 linhas + trailing newline = 51 entries em strings.Split.
	var b strings.Builder
	for i := 0; i < 50; i++ {
		b.WriteString("line ")
		b.WriteString(string(rune('A' + i%26)))
		b.WriteByte('\n')
	}
	res := CompressBashOutput("cat big.txt", b.String())
	if !res.Applied {
		t.Errorf("cat large should compress, got Applied=false: %+v", res)
	}
	// n=51, head=5, tail=3 → 43 omitidas
	if !strings.Contains(res.Output, "[... 43 lines omitted ...]") {
		t.Errorf("expected '43 lines omitted', got %q", res.Output)
	}
}

func TestCompressBashOutputShortPasses(t *testing.T) {
	// output curto demais para omitir o meio
	res := CompressBashOutput("cat tiny.txt", "a\nb\nc\n")
	if res.Applied {
		t.Errorf("short output should pass verbatim, got Applied=true")
	}
	if res.ReductionPct > 0 {
		t.Errorf("short output should have 0%% reduction, got %d%%", res.ReductionPct)
	}
}

func TestCompressBashOutputRejectsUnsafePipe(t *testing.T) {
	res := CompressBashOutput("ls | grep foo", "any output")
	if res.Applied {
		t.Errorf("ls | grep should fail-open, got Applied=true")
	}
	if !strings.Contains(res.Reason, "unsafe") {
		t.Errorf("expected 'unsafe' in reason, got %q", res.Reason)
	}
}

func TestCompressBashOutputGrepCompresses(t *testing.T) {
	// grep com muitos matches → comprime
	var b strings.Builder
	for i := 0; i < 30; i++ {
		b.WriteString("match: error in file ")
		b.WriteString(string(rune('A' + i%26)))
		b.WriteByte('\n')
	}
	res := CompressBashOutput("grep -r error", b.String())
	if !res.Applied {
		t.Errorf("grep with many matches should compress, got %+v", res)
	}
}

func TestCompressBashOutputEmptyCmd(t *testing.T) {
	res := CompressBashOutput("", "any output")
	if res.Applied {
		t.Error("empty cmd should fail-open")
	}
	if res.Output != "any output" {
		t.Error("empty cmd should pass verbatim")
	}
}

func TestRunBashCompressMissingCmd(t *testing.T) {
	withTempCache(t)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"bash-compress"}, &stdout, &stderr); err == nil {
		t.Fatal("expected error when --cmd is missing")
	}
}

func TestRunBashCompressSafeCmd(t *testing.T) {
	withTempCache(t)
	var stdout, stderr bytes.Buffer
	err := run([]string{"bash-compress", "--cmd", "ls -la", "--content", "foo\nbar\nbaz"}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	for _, want := range []string{"bash-compress:", "applied:", "reduction:"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output: %s", want, out)
		}
	}
}

func TestRunBashCompressUnsafeCmd(t *testing.T) {
	withTempCache(t)
	var stdout, stderr bytes.Buffer
	err := run([]string{"bash-compress", "--cmd", "ssh user@host", "--content", "ssh output"}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"applied":false`) {
		// when --json, output is JSON; when plain, "applied:    false"
		if !strings.Contains(stdout.String(), "applied:") {
			t.Errorf("expected applied=false info, got %s", stdout.String())
		}
	}
}

func TestRunBashCompressMissingContent(t *testing.T) {
	withTempCache(t)
	var stdout, stderr bytes.Buffer
	err := run([]string{"bash-compress", "--cmd", "ls"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when --content/--stdin is missing")
	}
}