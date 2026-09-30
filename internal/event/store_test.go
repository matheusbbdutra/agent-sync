package event

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// writeFakeMemoryMCP cria um memory-mcp fake que:
//   - incrementa um contador de invocações em FAKE_MCP_COUNTER;
//   - drena o stdin (evita EPIPE no handshake);
//   - falha com exit 1 enquanto a invocação <= FAKE_MCP_FAIL_TIMES
//     (emula `exit status 1` por escrita concorrente no memory.db);
//   - depois responde o handshake (id 1) + tools/call (id 2) em JSONL.
func writeFakeMemoryMCP(t *testing.T, failTimes int) (bin string, counter string) {
	t.Helper()
	dir := t.TempDir()
	bin = filepath.Join(dir, "memory-mcp")
	counter = filepath.Join(dir, "counter")
	script := `#!/usr/bin/env bash
set -uo pipefail
n=0
[ -f "$FAKE_MCP_COUNTER" ] && n="$(cat "$FAKE_MCP_COUNTER")"
n=$((n + 1))
printf '%s' "$n" > "$FAKE_MCP_COUNTER"
cat >/dev/null
if [ "$n" -le "$FAKE_MCP_FAIL_TIMES" ]; then
  echo 'database is locked' >&2
  exit 1
fi
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05"}}' \
  '{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"ok"}]}}'
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_MCP_COUNTER", counter)
	t.Setenv("FAKE_MCP_FAIL_TIMES", strconv.Itoa(failTimes))
	t.Setenv("AGENT_SYNC_MEMORY_MCP_BIN", bin)
	t.Setenv("AGENT_SYNC_MEMORY_DB", filepath.Join(dir, "memory.db"))
	return bin, counter
}

func readCounter(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ler contador: %v", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("contador invalido (%q): %v", raw, err)
	}
	return n
}

func TestCallRecordEventRetryAposFalhaTransitoria(t *testing.T) {
	// Falha 2x (concorrencia), sucesso na 3a: A-81.
	_, counter := writeFakeMemoryMCP(t, recordEventAttempts-1)

	args := RecordEventArgs{Agent: "tool", Kind: "task_completed", Note: "retry ok"}
	start := time.Now()
	if err := CallRecordEvent(args, 5*time.Second); err != nil {
		t.Fatalf("CallRecordEvent deveria ter sucesso apos retry: %v", err)
	}
	if got := readCounter(t, counter); got != recordEventAttempts {
		t.Errorf("tentativas=%d, esperado %d", got, recordEventAttempts)
	}
	// Orçamento: nao pode estourar o timeout recebido + backoffs.
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("retry estourou o orcamento total: %s", elapsed)
	}
}

func TestCallRecordEventErroExplicitoAposTentativas(t *testing.T) {
	_, counter := writeFakeMemoryMCP(t, 99)

	err := CallRecordEvent(RecordEventArgs{Agent: "tool", Kind: "task_completed", Note: "sempre falha"}, 5*time.Second)
	if err == nil {
		t.Fatal("esperava erro apos esgotar as tentativas")
	}
	if !strings.Contains(err.Error(), "apos 3 tentativas") {
		t.Errorf("erro sem o numero de tentativas: %v", err)
	}
	if !strings.Contains(err.Error(), "exit status 1") {
		t.Errorf("erro perdeu a causa raiz: %v", err)
	}
	if got := readCounter(t, counter); got != recordEventAttempts {
		t.Errorf("tentativas=%d, esperado %d", got, recordEventAttempts)
	}
}

func TestCallRecordEventBinarioAusenteNaoRetenta(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "counter")
	t.Setenv("FAKE_MCP_COUNTER", counter)
	t.Setenv("AGENT_SYNC_MEMORY_MCP_BIN", filepath.Join(dir, "memory-mcp-inexistente"))

	err := CallRecordEvent(RecordEventArgs{Agent: "tool", Kind: "task_completed", Note: "ausente"}, 2*time.Second)
	if !errors.Is(err, ErrEventStoreUnavailable) {
		t.Fatalf("esperava ErrEventStoreUnavailable, obteve %v", err)
	}
	// Binario ausente e falha permanente: nao pode haver retry.
	if _, statErr := os.Stat(counter); statErr == nil {
		t.Errorf("binario ausente nao deveria spawnar/retrys (contador criado)")
	}
}

// TestActorToAgentNormalizaVocabularioDoMCP guarda contra a divergência que
// motivou o A-82: os schemas usam `claude`/`agy`, o memory-mcp anuncia
// `claude-code`/`antigravity`; antes o valor era passado verbatim e não casava
// com os filtros documentados do MCP.
func TestActorToAgentNormalizaVocabularioDoMCP(t *testing.T) {
	cases := map[string]string{
		"claude":       "claude-code",
		"agy":          "antigravity",
		"cline":        "cline",
		"cli:claude":   "claude-code",
		"cli:agy":      "antigravity",
		"codex":        "codex",
		"user":         "user",
		"tool":         "tool",
		"agent-sync":   "agent-sync",
		"":             "agent-sync",
		"desconhecido": "agent-sync",
		"cli:cli-novo": "cli-novo",
	}
	for in, want := range cases {
		if got := actorToAgent(in); got != want {
			t.Errorf("actorToAgent(%q) = %q, esperado %q", in, got, want)
		}
	}
}
