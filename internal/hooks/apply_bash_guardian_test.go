package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// TestLoadBashGuardianPatterns valida que loadBashGuardianPatterns le o
// arquivo hooks/bash-guardian-patterns.txt do repo, ignora comentarios e
// linhas vazias, e expoe os 4 padroes novos (git add, commit, amend, tag).
// Regressao: 2026-09-22 - usuario pediu para git add/commit/tag exigirem
// confirmacao manual antes de mexer no historico de versoes.
func TestLoadBashGuardianPatterns(t *testing.T) {
	baseDir, ok := pathutil.FindBaseDir([]string{"."})
	if !ok {
		t.Fatal("baseDir nao encontrado a partir do diretorio de teste")
	}

	patterns, err := loadBashGuardianPatterns(baseDir)
	if err != nil {
		t.Fatalf("loadBashGuardianPatterns falhou: %v", err)
	}

	want := []string{
		"git add *",
		"git commit *",
		"git commit --amend*",
		"git tag *",
		"git tag -d *",
	}
	for _, w := range want {
		found := false
		for _, p := range patterns {
			if p == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("padrao %q ausente da lista carregada. Padroes atuais: %v", w, patterns)
		}
	}
}

// TestBashGuardianScriptAsksForGitHistoryCommands executa o script
// bash-guardian.antigravity.sh com payloads reais (gemini CLI envia
// tool_input.CommandLine) e valida que git add/commit/tag agora disparam
// decision=ask. Comandos nao-listados devem passar ({}).
//
// Anti-alucinacao (D-24): testado em runtime real com bash 5.x em
// 2026-09-22 antes de adicionar ao patterns.txt.
func TestBashGuardianScriptAsksForGitHistoryCommands(t *testing.T) {
	baseDir, ok := pathutil.FindBaseDir([]string{"."})
	if !ok {
		t.Fatal("baseDir nao encontrado a partir do diretorio de teste")
	}
	scriptPath := filepath.Join(baseDir, "hooks", "bash-guardian.antigravity.sh")
	if _, err := os.Stat(scriptPath); err != nil {
		t.Fatalf("script %s nao encontrado", scriptPath)
	}

	cases := []struct {
		command string
		wantAsk bool
	}{
		// Novos padroes (R-2026-09-22): git add/commit/amend/tag exigem
		// confirmacao manual antes de mexer no historico.
		{"git add file.go", true},
		{"git add .", true},
		{"git commit -m test", true},
		{"git commit --amend", true},
		{"git commit --amend -m fix", true},
		{"git tag v1.0", true},
		{"git tag -d v1.0", true},

		// `git add`/`git commit` puros sem args NAO casam com glob -
		// sao erros da CLI (nao destrutivos), passam direto. Decisao
		// consciente: padroes exigem pelo menos 1 char apos o comando.
		{"git commit", false},
		{"git add", false},

		// Padroes antigos nao regrediram.
		{"git push --force origin main", true},
		{"git reset --hard HEAD~1", true},
		{"git clean -fd", true},

		// Comandos inofensivos passam direto.
		{"ls", false},
		{"echo hello", false},
		{"git status", false},
		{"git diff", false},
	}

	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			payload := `{"tool_input":{"CommandLine":"` + tc.command + `"}}`
			cmd := exec.Command(scriptPath)
			cmd.Stdin = strings.NewReader(payload)
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("execucao do script falhou para %q: %v", tc.command, err)
			}
			got := strings.TrimSpace(string(out))
			isAsk := strings.Contains(got, `"decision":"ask"`)
			if tc.wantAsk && !isAsk {
				t.Errorf("comando %q deveria disparar ask, obteve: %s", tc.command, got)
			}
			if !tc.wantAsk && got != "{}" {
				t.Errorf("comando %q deveria passar ({}), obteve: %s", tc.command, got)
			}
		})
	}
}

// TestSyncBashGuardianAntigravityPreToolUse valida que syncBashGuardianAntigravity
// registra o hook em PreToolUse (formato nested com matcher run_command),
// NAO em PreInvocation, e limpa qualquer entrada orfa em PreInvocation.
func TestSyncBashGuardianAntigravityPreToolUse(t *testing.T) {
	tempBase, ok := pathutil.FindBaseDir([]string{"."})
	if !ok {
		t.Fatal("baseDir nao encontrado")
	}
	tempHooksPath := filepath.Join(t.TempDir(), "hooks.json")
	target := TargetCLI{
		Name:              "antigravity",
		AgentKind:         "antigravity",
		HooksSettingsPath: tempHooksPath,
		HooksFormat:       "antigravity",
		HooksEvent:        "PreInvocation",
	}

	// Pre-popula com entrada orfa em PreInvocation para simular estado legado
	if err := os.WriteFile(tempHooksPath, []byte(`{
		"agent-sync-bash-guardian": {
			"PreInvocation": [
				{"command":"legacy-command","name":"agent-sync-bash-guardian","timeout":10,"type":"command"}
			]
		}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncBashGuardianAntigravity(tempBase, target); err != nil {
		t.Fatalf("syncBashGuardianAntigravity falhou: %v", err)
	}

	root, err := readJSONObject(tempHooksPath)
	if err != nil {
		t.Fatalf("falha ao ler JSON: %v", err)
	}

	group, ok := root[bashGuardianHookName].(map[string]interface{})
	if !ok {
		t.Fatalf("grupo %q ausente em: %v", bashGuardianHookName, root)
	}

	if _, exists := group["PreInvocation"]; exists {
		t.Errorf("PreInvocation orfa nao foi removida: %v", group)
	}

	preEntries, ok := group["PreToolUse"].([]interface{})
	if !ok || len(preEntries) != 1 {
		t.Fatalf("PreToolUse deve ter exatamente 1 entrada nested, obteve: %v", group["PreToolUse"])
	}

	entry := preEntries[0].(map[string]interface{})
	if entry["matcher"] != "run_command" {
		t.Errorf("matcher esperado 'run_command', obteve %q", entry["matcher"])
	}

	hooksList, ok := entry["hooks"].([]interface{})
	if !ok || len(hooksList) != 1 {
		t.Fatalf("hooks list invalida: %v", entry["hooks"])
	}
	hookObj := hooksList[0].(map[string]interface{})
	cmdStr, _ := hookObj["command"].(string)
	if !strings.Contains(cmdStr, "bash-guardian.antigravity.sh") || !strings.Contains(cmdStr, "wrap-hook.sh PreToolUse") {
		t.Errorf("comando inesperado: %q", cmdStr)
	}
}
