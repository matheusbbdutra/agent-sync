package main

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		name   string
		cmd    string
		wantOK bool
	}{
		{"go test valido", "go test ./...", true},
		{"go sem argumento", "go", false},
		{"go com flag sem argumento", "go", false},
		{"go build com path", "go build ./tools/...", true},
		{"cargo run", "cargo run --bin cli", true},
		{"cargo sem args", "cargo", false},
		{"python com script", "python3 tools/check.py", true},
		{"python sem script", "python3", false},
		{"make alvo", "make build", true},
		{"make sem alvo", "make", false},
		{"comando nao coberto", "ls -la /tmp", true},
		{"vazio", "", true},
		{"kubectl sem subcommand", "kubectl", false},
		{"kubectl get pods", "kubectl get pods", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(c.cmd)
			if got.Valid != c.wantOK {
				t.Errorf("Classify(%q).Valid = %v, want %v (reason: %s)", c.cmd, got.Valid, c.wantOK, got.Reason)
			}
		})
	}
}
