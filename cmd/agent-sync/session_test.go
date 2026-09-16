package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func sessionGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", args[0], err, output)
	}
	return strings.TrimSpace(string(output))
}

func TestSessionSyncsAtStartAndEnd(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	if output, err := exec.Command("git", "init", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v: %s", err, output)
	}
	repo := filepath.Join(root, "checkout")
	if output, err := exec.Command("git", "clone", remote, repo).CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v: %s", err, output)
	}
	sessionGit(t, repo, "config", "user.name", "Teste")
	sessionGit(t, repo, "config", "user.email", "teste@example.invalid")
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("inicial\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sessionGit(t, repo, "add", "README")
	sessionGit(t, repo, "commit", "-m", "inicial")
	sessionGit(t, repo, "push", "-u", "origin", "HEAD")

	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	stubs := map[string]string{
		"agent-sync":  "#!/bin/sh\nprintf 'apply:%s\\n' \"$AGENT_SYNC_HOME\" >> \"$EVENT_LOG\"\n",
		"memory-sync": "#!/bin/sh\nprintf 'memory:%s\\n' \"$2\" >> \"$EVENT_LOG\"\n",
		"fake-cli":    "#!/bin/sh\nprintf 'cli\\n' >> \"$EVENT_LOG\"\nprintf 'alterado\\n' > \"$TEST_REPO/README\"\ngit -C \"$TEST_REPO\" add README\ngit -C \"$TEST_REPO\" commit -m melhoria >/dev/null\n",
	}
	for name, body := range stubs {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	log := filepath.Join(root, "events.log")
	script := filepath.Join("..", "..", "scripts", "agent-sync-session.sh")
	cmd := exec.Command("bash", script, repo, "fake-cli")
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "EVENT_LOG="+log, "TEST_REPO="+repo)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sessão: %v: %s", err, output)
	}
	events, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	want := "apply:" + repo + "\nmemory:start\ncli\nmemory:end\n"
	if string(events) != want {
		t.Fatalf("ordem incorreta: %q", events)
	}
	localSHA := sessionGit(t, repo, "rev-parse", "HEAD")
	remoteSHA := sessionGit(t, remote, "rev-parse", "HEAD")
	if localSHA != remoteSHA {
		t.Fatalf("commit não enviado: %s != %s", localSHA, remoteSHA)
	}
}
