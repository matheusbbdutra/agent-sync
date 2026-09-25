package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/matheusdutra/token-tools/internal/agentmemory"
)

func TestBufferRecordAndConsolidate(t *testing.T) {
	sessionID := "test-sess-12345"
	bPath := bufferFilePath(sessionID)
	_ = os.Remove(bPath)
	defer os.Remove(bPath)

	// 1. Grava 2 observações no buffer
	err := runBufferRecord([]string{
		"-session", sessionID,
		"-tool", "bash",
		"-status", "exit_1",
		"-note", "go test falhou: undefined symbol Foo",
	})
	if err != nil {
		t.Fatalf("buffer-record 1 falhou: %v", err)
	}

	err = runBufferRecord([]string{
		"-session", sessionID,
		"-tool", "bash",
		"-status", "exit_0",
		"-note", "go test passou após exportar Foo",
	})
	if err != nil {
		t.Fatalf("buffer-record 2 falhou: %v", err)
	}

	if _, err := os.Stat(bPath); err != nil {
		t.Fatalf("arquivo de buffer não foi criado: %v", err)
	}

	// 2. Abre store temporário em memória / arquivo isolado
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-memory.db")
	store, err := agentmemory.Open(dbPath)
	if err != nil {
		t.Fatalf("abrir test store: %v", err)
	}
	defer store.Close()

	// 3. Executa consolidate
	var out bytes.Buffer
	err = runConsolidate(store, []string{"-session", sessionID}, &out)
	if err != nil {
		t.Fatalf("consolidate falhou: %v", err)
	}

	if !strings.Contains(out.String(), "2 observação(ões) consolidada(s)") {
		t.Errorf("saída inesperada do consolidate: %s", out.String())
	}

	// Buffer deve ter sido removido
	if _, err := os.Stat(bPath); !os.IsNotExist(err) {
		t.Errorf("buffer ainda existe após consolidate: %s", bPath)
	}

	// 4. Executa query para verificar se as memórias foram salvas e são consultáveis
	var queryOut bytes.Buffer
	err = runQuery(store, []string{"-query", "Foo", "-format", "text"}, &queryOut)
	if err != nil {
		t.Fatalf("query falhou: %v", err)
	}

	if !strings.Contains(queryOut.String(), "Foo") {
		t.Errorf("query não encontrou memórias consolidadas: %s", queryOut.String())
	}

	// 5. Query em JSON
	var jsonOut bytes.Buffer
	err = runQuery(store, []string{"-query", "Foo", "-format", "json"}, &jsonOut)
	if err != nil {
		t.Fatalf("query json falhou: %v", err)
	}
	if !strings.Contains(jsonOut.String(), `"Content"`) {
		t.Errorf("query json não retornou campos esperados: %s", jsonOut.String())
	}
}

func TestBufferRecordValidation(t *testing.T) {
	err := runBufferRecord([]string{"-session", "test"})
	if err == nil {
		t.Errorf("esperava erro ao chamar buffer-record sem -tool e -note")
	}
}

// TestMemoryReadPage cobre A-56 (S-0.2, ses_f2b646cd, 2026-09-24):
// memory_read_page sobre get_memory, com formatacao padronizada.
// Valida 4 cenarios: existente com path normalizado, inexistente
// (idempotente), path vazio (recusado), nome valido com description custom.
func TestMemoryReadPage(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-memory.db")
	store, err := agentmemory.Open(dbPath)
	if err != nil {
		t.Fatalf("abrir test store: %v", err)
	}
	defer store.Close()

	sessionID := "ses-a56-test"
	pageName := "adr/A56-test-page"
	pageDesc := "Description customizada"
	pageBody := "Body do A-56 com formatacao nova"

	if err := store.Upsert(agentmemory.Memory{
		Agent: "opencode", SessionID: sessionID, Type: "reference",
		Name: pageName, Description: pageDesc, Content: pageBody, Scratch: true,
	}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Cenario 1: path normalizado ("//./X/")
	res := callTool(store, "memory_read_page", mustJSON(t, map[string]string{"path": "//./" + pageName + "/"}))
	if isErr(res) {
		t.Fatalf("read scratch falhou: %v", res)
	}
	txt := textOf(res)
	for _, want := range []string{"page: " + pageName, "type: reference", "description: " + pageDesc, "agent: opencode", "scratch: true", pageBody} {
		if !strings.Contains(txt, want) {
			t.Errorf("output sem %q. Obtido:\n%s", want, txt)
		}
	}

	// Cenario 2: inexistente idempotente
	res = callTool(store, "memory_read_page", mustJSON(t, map[string]string{"path": "adr/nao-existe-a56"}))
	if isErr(res) {
		t.Errorf("read de inexistente nao deveria ser erro: %v", res)
	}
	if !strings.Contains(textOf(res), "Nenhuma page") {
		t.Errorf("esperava 'Nenhuma page', obteve: %s", textOf(res))
	}

	// Cenario 3: path vazio recusado com msg clara
	res = callTool(store, "memory_read_page", mustJSON(t, map[string]string{"path": "  "}))
	if !isErr(res) {
		t.Errorf("path vazio deveria ser erro: %v", res)
	}
	if !strings.Contains(textOf(res), "obrigatório") {
		t.Errorf("esperava msg de obrigatorio, obteve: %s", textOf(res))
	}
}

// TestMemoryReadSession cobre A-57 (S-0.2, ses_f2b646cd, 2026-09-24):
// memory_read_session sobre ListEventsBySession. Valida 3 cenarios:
// session_id com 2 eventos (retorna 2), session_id sem eventos
// (mensagem de vazio), session_id vazio (recusado).
func TestMemoryReadSession(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-memory.db")
	store, err := agentmemory.Open(dbPath)
	if err != nil {
		t.Fatalf("abrir test store: %v", err)
	}
	defer store.Close()

	sessionID := "ses-a57-test"
	outroSession := "ses-other"

	// Setup: 2 eventos na sessao alvo + 1 evento em outra sessao.
	// Cada evento usa name unico (sufixo nanos) — UNIQUE INDEX
	// memories_project_type_name obriga (project_id, type, name) distintos
	// (verificado em tools/internal/agentmemory/store.go:131).
	mustUpsert := func(sessID, kind, tag, content string) {
		t.Helper()
		if err := store.Upsert(agentmemory.Memory{
			Agent: "opencode", SessionID: sessID, Type: "event",
			Name: fmt.Sprintf("%s-20260924T180000Z-%s", kind, tag),
			Description: content, Content: content, Scratch: true,
		}); err != nil {
			t.Fatalf("setup %s/%s: %v", kind, tag, err)
		}
	}
	mustUpsert(sessionID, "decision", "uuid-1", "decisao A-57 numero 1")
	mustUpsert(sessionID, "guard_nudge", "uuid-2", "nudge qualquer")
	mustUpsert(outroSession, "decision", "uuid-3", "decisao de outra sessao")

	// Cenario 1: session_id com 2 eventos -> retorna 2 (ignora o de outra sessao).
	res := callTool(store, "memory_read_session", mustJSON(t, map[string]string{"session_id": sessionID}))
	if isErr(res) {
		t.Fatalf("read session falhou: %v", res)
	}
	out := textOf(res)
	for _, want := range []string{"decisao A-57 numero 1", "nudge qualquer"} {
		if !strings.Contains(out, want) {
			t.Errorf("output sem %q. Obtido:\n%s", want, out)
		}
	}
	if strings.Contains(out, "decisao de outra sessao") {
		t.Errorf("output vazou evento de outra sessao: %s", out)
	}

	// Cenario 2: session_id sem eventos -> mensagem de vazio.
	res = callTool(store, "memory_read_session", mustJSON(t, map[string]string{"session_id": "ses-vazia-xyz"}))
	if isErr(res) {
		t.Errorf("session vazia nao deveria ser erro: %v", res)
	}
	if !strings.Contains(textOf(res), "Nenhum evento") {
		t.Errorf("esperava 'Nenhum evento', obteve: %s", textOf(res))
	}

	// Cenario 3: session_id vazio -> recusado.
	res = callTool(store, "memory_read_session", mustJSON(t, map[string]string{"session_id": "  "}))
	if !isErr(res) {
		t.Errorf("session_id vazio deveria ser erro: %v", res)
	}
	if !strings.Contains(textOf(res), "obrigatório") {
		t.Errorf("esperava msg 'obrigatorio', obteve: %s", textOf(res))
	}
}

// TestMemoryDeletePage cobre A-60 (S-0.2, ses_f2b646cd, 2026-09-24):
// memory_delete_page sobre delete_memory, com normalização de path.
// Valida 4 cenários: scratch removível, path com ./ e / finais,
// name inexistente (idempotente), permanente (admissão).
func TestMemoryDeletePage(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-memory.db")
	store, err := agentmemory.Open(dbPath)
	if err != nil {
		t.Fatalf("abrir test store: %v", err)
	}
	defer store.Close()

	sessionID := "ses-a60-test"
	scratchName := "adr/A60-test-scratch"
	permName := "adr/A60-test-perm"

	mustStore := func(m agentmemory.Memory, label string) {
		t.Helper()
		if err := store.Upsert(m); err != nil {
			t.Fatalf("setup %s: %v", label, err)
		}
	}

	// Cenário 1: scratch removível com path "./foo/bar/"
	mustStore(agentmemory.Memory{
		Agent: "opencode", SessionID: sessionID, Type: "reference",
		Name: scratchName, Description: "scratch", Content: "x", Scratch: true,
	}, "scratch")
	res := callTool(store, "memory_delete_page", mustJSON(t, map[string]string{"path": "./" + scratchName + "/"}))
	if isErr(res) {
		t.Fatalf("delete scratch falhou: %v", res)
	}
	if !strings.Contains(textOf(res), "removida") {
		t.Errorf("esperava 'removida', obteve: %s", textOf(res))
	}

	// Cenário 2: name inexistente → idempotente (sem erro)
	res = callTool(store, "memory_delete_page", mustJSON(t, map[string]string{"path": "adr/nao-existe-xyz"}))
	if isErr(res) {
		t.Errorf("delete de name inexistente nao deveria ser erro: %v", res)
	}
	if !strings.Contains(textOf(res), "Nenhuma page") {
		t.Errorf("esperava 'Nenhuma page', obteve: %s", textOf(res))
	}

	// Cenário 3: permanente (scratch=false) → admitida, recusa
	mustStore(agentmemory.Memory{
		Agent: "opencode", SessionID: sessionID, Type: "decision",
		Name: permName, Description: "perm", Content: "y", Scratch: false,
	}, "perm")
	res = callTool(store, "memory_delete_page", mustJSON(t, map[string]string{"path": permName}))
	if !isErr(res) {
		t.Errorf("delete de permanente deveria falhar, obteve sucesso: %v", res)
	}
	if !strings.Contains(textOf(res), "não é scratch") {
		t.Errorf("esperava msg de admissao, obteve: %s", textOf(res))
	}

	// Cenário 4: path vazio → recusa com msg clara
	res = callTool(store, "memory_delete_page", mustJSON(t, map[string]string{"path": "  "}))
	if !isErr(res) {
		t.Errorf("path vazio deveria ser erro, obteve: %v", res)
	}
	if !strings.Contains(textOf(res), "obrigatório") {
		t.Errorf("esperava msg de obrigatorio, obteve: %s", textOf(res))
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func isErr(res map[string]any) bool {
	if res == nil {
		return true
	}
	_, ok := res["isError"]
	return ok
}

func textOf(res map[string]any) string {
	cs, _ := res["content"].([]map[string]any)
	if len(cs) == 0 {
		return ""
	}
	s, _ := cs[0]["text"].(string)
	return s
}

// TestMemoryQuery cobre A-54 (S-0.2, ses_f2b12742, 2026-09-24):
// memory_query como alias de search_memory. Valida 3 cenarios:
// query com hit (delega + formata), query sem hit (mensagem
// 'Nenhuma memória'), query vazia (recusado).
func TestMemoryQuery(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-memory.db")
	store, err := agentmemory.Open(dbPath)
	if err != nil {
		t.Fatalf("abrir test store: %v", err)
	}
	defer store.Close()

	if err := store.Upsert(agentmemory.Memory{
		Agent: "opencode", Type: "reference",
		Name: "a54-test-page", Description: "Pagina de teste",
		Content: "conteudo unico palavrachave xyzzy", Scratch: true,
	}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Cenario 1: query com hit — delega para runSearchMemory
	res := callTool(store, "memory_query", mustJSON(t, map[string]string{"query": "xyzzy"}))
	if isErr(res) {
		t.Fatalf("query com hit falhou: %v", res)
	}
	txt := textOf(res)
	for _, want := range []string{"a54-test-page", "Pagina de teste", "palavrachave xyzzy"} {
		if !strings.Contains(txt, want) {
			t.Errorf("output sem %q. Obtido:\n%s", want, txt)
		}
	}

	// Cenario 2: query sem hit — mensagem padrao
	res = callTool(store, "memory_query", mustJSON(t, map[string]string{"query": "termo-inexistente-a54"}))
	if isErr(res) {
		t.Errorf("query sem hit nao deveria ser erro: %v", res)
	}
	if !strings.Contains(textOf(res), "Nenhuma memória") {
		t.Errorf("esperava 'Nenhuma memória', obteve: %s", textOf(res))
	}

	// Cenario 3: query vazia — recusada com mensagem clara
	res = callTool(store, "memory_query", mustJSON(t, map[string]string{"query": "  "}))
	if !isErr(res) {
		t.Errorf("query vazia deveria ser erro: %v", res)
	}
	if !strings.Contains(textOf(res), "obrigatório") {
		t.Errorf("esperava msg de obrigatorio, obteve: %s", textOf(res))
	}
}

func TestConsolidateEmptyBuffer(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-memory.db")
	store, err := agentmemory.Open(dbPath)
	if err != nil {
		t.Fatalf("abrir test store: %v", err)
	}
	defer store.Close()

	var out bytes.Buffer
	err = runConsolidate(store, []string{"-session", "sess-inexistente-" + time.Now().Format("150405")}, &out)
	if err != nil {
		t.Fatalf("consolidate em buffer vazio não deveria falhar: %v", err)
	}
	if !strings.Contains(out.String(), "Buffer vazio ou ausente") {
		t.Errorf("esperava mensagem de buffer vazio, obteve: %s", out.String())
	}
}

// TestRequireEvidence cobre o gate numerico do store_memory (A-62,
// ses_f2ad21a27ffeasIkrIiVwQc53m, 2026-09-24). Validacao empirica do
// preference/regra-2-pass-write: cardinal solto + unidade contavel, sem
// prefixo ~ nem placeholder N, exige campo `evidence` no schema.
//
// Tabela-driven cobre os cenarios documentados no doc de A-62 (linhas
// 400-408 do main.go) + edge cases que o codigo trata explicitamente
// (linha N, v2.0, data ISO, prefixo ~). Falso negativo conhecido
// documentado em evidencia (linha 317): "tokens" nao esta na lista de
// unidades contaveis por YAGNI.
func TestRequireEvidence(t *testing.T) {
	cases := []struct {
		name           string
		content        string
		evidence       string
		wantMissing    bool
		wantValue      int
		wantUnit       string
	}{
		{
			name:        "cardinal + commits dispara sem evidence",
			content:     "fiz 4 commits hoje",
			evidence:    "",
			wantMissing: true,
			wantValue:   4,
			wantUnit:    "commits",
		},
		{
			name:        "cardinal + commits passa com evidence",
			content:     "fiz 4 commits hoje",
			evidence:    "git log --since=2026-09-24 --oneline | wc -l -> 4",
			wantMissing: false,
		},
		{
			name:        "cardinal + linhas dispara sem evidence",
			content:     "adicionei 150 linhas em command.go",
			evidence:    "",
			wantMissing: true,
			wantValue:   150,
			wantUnit:    "linhas",
		},
		{
			name:        "sem cardinal contavel nao dispara",
			content:     "decisao sobre schema do memory-mcp",
			evidence:    "",
			wantMissing: false,
		},
		{
			name:        "line ref linha N e exempted",
			content:     "ver linha 42 do arquivo command.go",
			evidence:    "",
			wantMissing: false,
		},
		{
			name:        "version semantica e exempted",
			content:     "schema em versao 2.0 do jsonschema",
			evidence:    "",
			wantMissing: false,
		},
		{
			name:        "data ISO no inicio e exempted",
			content:     "2026-09-24 sessao sobre A-62 gate numerico",
			evidence:    "",
			wantMissing: false,
		},
		{
			name:        "placeholder N no inicio e exempted",
			content:     "N commits ate o final da sprint",
			evidence:    "",
			wantMissing: false,
		},
		{
			name:        "prefixo til antes do cardinal e exempted",
			content:     "estimativa: ~150 linhas no comando",
			evidence:    "",
			wantMissing: false,
		},
		{
			// Falso negativo intencional (YAGNI, documentado em main.go:317):
			// "tokens" nao esta na lista de unidades contaveis, entao o gate
			// NAO dispara e a evidencia NAO e exigida. Comportamento deliberado
			// - se o agente quiser ser rigoroso com tokens, adiciona evidence
			// por conta propria (a regra 2-pass-write recomenda sempre que
			// houver numero nao-trivial).
			name:        "tokens NAO esta na lista - gate nao dispara (YAGNI)",
			content:     "usei 100 tokens no teste",
			evidence:    "",
			wantMissing: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotMatch, gotMissing := requireEvidence(tc.content, tc.evidence)
			if gotMissing != tc.wantMissing {
				t.Errorf("requireEvidence(%q, %q) missing=%v, want %v (match=%+v)",
					tc.content, tc.evidence, gotMissing, tc.wantMissing, gotMatch)
			}
			if tc.wantMissing && gotMissing {
				if gotMatch.Value != tc.wantValue {
					t.Errorf("Value=%d, want %d", gotMatch.Value, tc.wantValue)
				}
				if gotMatch.Unit != tc.wantUnit {
					t.Errorf("Unit=%q, want %q", gotMatch.Unit, tc.wantUnit)
				}
			}
		})
	}
}

// openTestStoreFromTempDir cria um Store SQLite em arquivo temporário isolado
// (evita colisão com a ~/.cache/agent-sync/memory.db real). Usado pelos
// testes de runPrune/runStats introduzidos em A-67.
func openTestStoreFromTempDir(t *testing.T) *agentmemory.Store {
	t.Helper()
	tmpDir := t.TempDir()
	store, err := agentmemory.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Open test store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// TestRunPruneDryRun valida que --dry-run (default ON) mostra preview sem
// deletar; --confirm remove de fato (A-67).
//
// Cuidado: store.Get() chama touchHit (atualiza accessed_at=now), entao
// validacoes intermediarias com Get podem invalidar accessed_at artificial.
// Por isso so chamamos Get() no final, apos os 3 sub-testes.
func TestRunPruneDryRun(t *testing.T) {
	store := openTestStoreFromTempDir(t)
	// 1 scratch velha + 1 scratch recente + 1 permanente velha
	for _, m := range []agentmemory.Memory{
		{Agent: "codex", Type: "event", Name: "old-scratch", Description: "d", Content: "c", Scratch: true},
		{Agent: "codex", Type: "event", Name: "recent-scratch", Description: "d", Content: "c", Scratch: true},
		{Agent: "codex", Type: "project", Name: "old-permanent", Description: "d", Content: "c", Scratch: false},
	} {
		if err := store.Upsert(m); err != nil {
			t.Fatalf("Upsert %s: %v", m.Name, err)
		}
	}
	setAccessedAt := func(name string, ago time.Duration) {
		t.Helper()
		old := time.Now().UTC().Add(-ago).Format(time.RFC3339)
		if _, err := store.DB().Exec(`UPDATE memories SET accessed_at = ? WHERE name = ?`, old, name); err != nil {
			t.Fatalf("UPDATE accessed_at %s: %v", name, err)
		}
	}
	setAccessedAt("old-scratch", 60*24*time.Hour)
	setAccessedAt("old-permanent", 60*24*time.Hour)

	var out bytes.Buffer

	// 1) Default = dry-run: NAO deleta, mas mostra preview.
	if err := runPrune(store, []string{"-older-than", "720h"}, &out); err != nil {
		t.Fatalf("runPrune dry-run: %v", err)
	}
	if !strings.Contains(out.String(), "1 memória(s) scratch") {
		t.Fatalf("preview deveria mostrar 1 entrada, obtive: %s", out.String())
	}
	if !strings.Contains(out.String(), "old-scratch") {
		t.Fatalf("preview deveria listar old-scratch: %s", out.String())
	}
	out.Reset()

	// 2) --dry-run=false sem --confirm: tambem nao deleta (seguranca em camadas).
	if err := runPrune(store, []string{"-older-than", "720h", "-dry-run=false"}, &out); err != nil {
		t.Fatalf("runPrune dry-run=false: %v", err)
	}
	out.Reset()

	// 3) --dry-run=false --confirm: remove de fato.
	// Re-set accessed_at (passos 1-2 podem ter alterado via side-effect de query?).
	// Nao, SELECT nao altera; seguro re-set para garantir isolamento.
	setAccessedAt("old-scratch", 60*24*time.Hour)
	if err := runPrune(store, []string{"-older-than", "720h", "-dry-run=false", "-confirm"}, &out); err != nil {
		t.Fatalf("runPrune confirm: %v", err)
	}
	if !strings.Contains(out.String(), "1 memória(s) scratch removida(s)") {
		t.Fatalf("--confirm deveria remover 1: %s", out.String())
	}
	out.Reset()

	// Validacoes finais (Get so agora para nao invalidar accessed_at).
	if got, _ := store.Get("old-scratch"); got != nil {
		t.Fatal("--confirm deveria ter removido old-scratch")
	}
	if got, _ := store.Get("old-permanent"); got == nil {
		t.Fatal("--confirm removeu permanente (invariante violada)")
	}
	if got, _ := store.Get("recent-scratch"); got == nil {
		t.Fatal("--confirm removeu scratch recente (nao devia)")
	}
}

// TestRunStats valida que runStats imprime agregacoes (text) e JSON estruturado.
func TestRunStats(t *testing.T) {
	store := openTestStoreFromTempDir(t)
	for _, m := range []agentmemory.Memory{
		{Agent: "claude-code", Type: "event", Name: "e1", Description: "d", Content: "c", Scratch: true},
		{Agent: "codex", Type: "event", Name: "e2", Description: "d", Content: "c", Scratch: true},
		{Agent: "claude-code", Type: "project", Name: "p1", Description: "d", Content: "c", Scratch: false},
	} {
		if err := store.Upsert(m); err != nil {
			t.Fatalf("Upsert %s: %v", m.Name, err)
		}
	}

	// text
	var out bytes.Buffer
	if err := runStats(store, []string{}, &out); err != nil {
		t.Fatalf("runStats text: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "total:     3") {
		t.Fatalf("text deveria ter total=3: %s", text)
	}
	if !strings.Contains(text, "scratch:   2") || !strings.Contains(text, "permanent: 1") {
		t.Fatalf("text deveria ter scratch=2 permanent=1: %s", text)
	}
	if !strings.Contains(text, "event:") || !strings.Contains(text, "project:") {
		t.Fatalf("text deveria ter by_type: %s", text)
	}
	out.Reset()

	// JSON
	if err := runStats(store, []string{"-json"}, &out); err != nil {
		t.Fatalf("runStats json: %v", err)
	}
	var got agentmemory.Stats
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("JSON unmarshal: %v (output=%s)", err, out.String())
	}
	if got.Total != 3 {
		t.Fatalf("JSON Total=%d, want 3", got.Total)
	}
	if got.ByScratch["scratch"] != 2 || got.ByScratch["permanent"] != 1 {
		t.Fatalf("JSON ByScratch=%+v", got.ByScratch)
	}
	if got.ByType["event"] != 2 || got.ByType["project"] != 1 {
		t.Fatalf("JSON ByType=%+v", got.ByType)
	}
}
