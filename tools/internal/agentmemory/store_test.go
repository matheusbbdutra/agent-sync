package agentmemory

import (
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUpsertGetList(t *testing.T) {
	s := openTestStore(t)

	if err := s.Upsert(Memory{
		Agent: "claude-code", Type: "project", Name: "foo",
		Description: "descrição foo", Content: "conteúdo original",
	}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := s.Get("foo")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || got.Content != "conteúdo original" {
		t.Fatalf("Get retornou %+v, esperado conteúdo original", got)
	}

	// upsert deve atualizar, não duplicar (chave única type+name)
	if err := s.Upsert(Memory{
		Agent: "antigravity", Type: "project", Name: "foo",
		Description: "descrição foo", Content: "conteúdo atualizado",
	}); err != nil {
		t.Fatalf("Upsert (update): %v", err)
	}

	items, err := s.List("", "", 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("List retornou %d itens, esperado 1 (upsert deveria substituir)", len(items))
	}
	if items[0].Content != "conteúdo atualizado" || items[0].Agent != "antigravity" {
		t.Fatalf("List retornou %+v, esperado conteúdo/agente atualizados", items[0])
	}
}

func TestSearchFTS(t *testing.T) {
	s := openTestStore(t)

	if err := s.Upsert(Memory{
		Agent: "claude-code", Type: "feedback", Name: "regra-testes",
		Description: "não escrever testes fake", Content: "sempre validar contra implementação real",
	}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := s.Upsert(Memory{
		Agent: "codex", Type: "reference", Name: "outra-memoria",
		Description: "aponta pro linear", Content: "bugs de pipeline ficam no projeto INGEST",
	}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	items, err := s.Search("testes", "", "", 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(items) != 1 || items[0].Name != "regra-testes" {
		t.Fatalf("Search retornou %+v, esperado só 'regra-testes'", items)
	}

	none, err := s.Search("inexistente-xyz", "", "", 0)
	if err != nil {
		t.Fatalf("Search (sem match): %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("Search deveria retornar vazio, retornou %d", len(none))
	}
}

func TestDeleteSoRemoveScratch(t *testing.T) {
	s := openTestStore(t)

	if err := s.Upsert(Memory{Agent: "claude-code", Type: "project", Name: "decisao-real", Description: "d", Content: "c"}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := s.Upsert(Memory{Agent: "opencode", Type: "project", Name: "rascunho", Description: "d", Content: "c", Scratch: true}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	if _, err := s.Delete("decisao-real"); err != ErrNotScratch {
		t.Fatalf("Delete em memória não-scratch deveria retornar ErrNotScratch, retornou %v", err)
	}
	if got, _ := s.Get("decisao-real"); got == nil {
		t.Fatalf("memória não-scratch foi removida indevidamente")
	}

	deleted, err := s.Delete("rascunho")
	if err != nil {
		t.Fatalf("Delete (scratch): %v", err)
	}
	if !deleted {
		t.Fatalf("Delete deveria confirmar remoção de memória scratch")
	}
	if got, _ := s.Get("rascunho"); got != nil {
		t.Fatalf("memória scratch deveria ter sido removida")
	}

	deleted, err = s.Delete("nao-existe")
	if err != nil {
		t.Fatalf("Delete (inexistente): %v", err)
	}
	if deleted {
		t.Fatalf("Delete de nome inexistente não deveria confirmar remoção")
	}
}

func TestListFiltraPorAgenteETipo(t *testing.T) {
	s := openTestStore(t)
	s.Upsert(Memory{Agent: "claude-code", Type: "project", Name: "a", Description: "d", Content: "c"})
	s.Upsert(Memory{Agent: "opencode", Type: "feedback", Name: "b", Description: "d", Content: "c"})

	items, err := s.List("opencode", "", 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 || items[0].Name != "b" {
		t.Fatalf("List por agente retornou %+v", items)
	}

	items, err = s.List("", "project", 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 || items[0].Name != "a" {
		t.Fatalf("List por tipo retornou %+v", items)
	}
}

func TestProjectScopePreventsNameCollisionAndFilters(t *testing.T) {
	store := openTestStore(t)
	for _, project := range []string{"one", "two"} {
		if err := store.Upsert(Memory{Agent: "codex", Type: "project", Name: "decision", Description: "scope", Content: "shared word", ProjectID: project, ProjectPath: "/repo/" + project, PC: "pc-" + project}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Get("decision"); err == nil {
		t.Fatal("nome ambíguo aceito")
	}
	first, err := store.GetScoped("decision", "one")
	if err != nil || first.ProjectPath != "/repo/one" {
		t.Fatalf("get escopado: %+v %v", first, err)
	}
	items, err := store.Search("shared", "", "", 10, ScopeFilter{ProjectID: "two"})
	if err != nil || len(items) != 1 || items[0].PC != "pc-two" {
		t.Fatalf("busca escopada: %+v %v", items, err)
	}
	items, err = store.List("", "", 10, ScopeFilter{PC: "pc-one", ProjectPath: "/repo/one"})
	if err != nil || len(items) != 1 || items[0].ProjectID != "one" {
		t.Fatalf("lista escopada: %+v %v", items, err)
	}
}

func TestApplySchemaMigratesExistingDBWithoutAccessedAtColumn(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// simula um banco criado antes da coluna existir: remove accessed_at e
	// reaplica o schema, que deve recriá-la sem falhar (ALTER TABLE idempotente).
	if _, err := s.db.Exec(`CREATE TABLE memories_legacy AS SELECT id,agent,session_id,pc,project_path,project_id,type,name,description,content,scratch,embedding_json,created_at,updated_at FROM memories`); err != nil {
		t.Fatalf("preparar tabela legada: %v", err)
	}
	if _, err := s.db.Exec(`DROP TABLE memories`); err != nil {
		t.Fatalf("drop memories: %v", err)
	}
	if _, err := s.db.Exec(`ALTER TABLE memories_legacy RENAME TO memories`); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if err := applySchema(s.db); err != nil {
		t.Fatalf("applySchema em banco legado: %v", err)
	}
	if err := s.Upsert(Memory{Agent: "codex", Type: "project", Name: "migrado", Description: "d", Content: "c"}); err != nil {
		t.Fatalf("Upsert pós-migração: %v", err)
	}
	got, err := s.Get("migrado")
	if err != nil || got == nil || got.AccessedAt.IsZero() {
		t.Fatalf("Get pós-migração: %+v %v", got, err)
	}
	s.Close()
}

func TestTouchUpdatesAccessedAt(t *testing.T) {
	s := openTestStore(t)
	if err := s.Upsert(Memory{Agent: "codex", Type: "project", Name: "x", Description: "d", Content: "c"}); err != nil {
		t.Fatal(err)
	}
	before, err := s.Get("x")
	if err != nil || before == nil {
		t.Fatalf("Get inicial: %+v %v", before, err)
	}
	time.Sleep(1100 * time.Millisecond) // RFC3339 tem resolução de segundo
	if err := s.Touch(before.ProjectID, before.Type, before.Name); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	after, err := s.GetScoped("x", before.ProjectID)
	if err != nil || after == nil {
		t.Fatalf("GetScoped pós-touch: %+v %v", after, err)
	}
	if !after.AccessedAt.After(before.AccessedAt) {
		t.Fatalf("accessed_at não avançou: antes=%v depois=%v", before.AccessedAt, after.AccessedAt)
	}
}

func TestSearchPrefersMoreRecentlyAccessedOnTie(t *testing.T) {
	s := openTestStore(t)
	// mesmo termo em ambas para empatar o bm25 textual.
	for _, name := range []string{"antigo", "recente"} {
		if err := s.Upsert(Memory{Agent: "codex", Type: "project", Name: name, Description: "termo comum", Content: "termo comum"}); err != nil {
			t.Fatal(err)
		}
	}
	old, err := s.GetScoped("antigo", "")
	if err != nil || old == nil {
		t.Fatalf("GetScoped antigo: %+v %v", old, err)
	}
	// força accessed_at do "antigo" para muito no passado; "recente" fica com o
	// acesso do Get/Upsert original (agora).
	if _, err := s.db.Exec(`UPDATE memories SET accessed_at = ? WHERE name = 'antigo'`, time.Now().UTC().Add(-60*24*time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatalf("forçar accessed_at antigo: %v", err)
	}
	items, err := s.Search("comum", "", "", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("esperado 2 resultados, veio %d", len(items))
	}
	if items[0].Name != "recente" {
		t.Fatalf("esperado 'recente' primeiro em empate textual, veio %+v", items)
	}
}

func TestPruneScratchRemovesOnlyExpiredScratch(t *testing.T) {
	s := openTestStore(t)
	if err := s.Upsert(Memory{Agent: "codex", Type: "project", Name: "permanente-velha", Description: "d", Content: "c", Scratch: false}); err != nil {
		t.Fatal(err)
	}
	if err := s.Upsert(Memory{Agent: "codex", Type: "project", Name: "scratch-velha", Description: "d", Content: "c", Scratch: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.Upsert(Memory{Agent: "codex", Type: "project", Name: "scratch-recente", Description: "d", Content: "c", Scratch: true}); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-60 * 24 * time.Hour).Format(time.RFC3339)
	if _, err := s.db.Exec(`UPDATE memories SET accessed_at = ? WHERE name IN ('permanente-velha', 'scratch-velha')`, old); err != nil {
		t.Fatalf("forçar accessed_at velho: %v", err)
	}

	n, err := s.PruneScratch(30 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("PruneScratch: %v", err)
	}
	if n != 1 {
		t.Fatalf("esperado remover 1 memória, removeu %d", n)
	}
	if got, _ := s.Get("permanente-velha"); got == nil {
		t.Fatal("PruneScratch removeu memória permanente (scratch=false) — invariante violado")
	}
	if got, _ := s.Get("scratch-velha"); got != nil {
		t.Fatal("scratch expirada não foi removida")
	}
	if got, _ := s.Get("scratch-recente"); got == nil {
		t.Fatal("PruneScratch removeu scratch recente indevidamente")
	}
}
