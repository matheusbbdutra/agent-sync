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

// TestPreviewScratchOlderThan valida que o preview é read-only (não deleta) e
// conta apenas scratch expirada (A-67, dry-run antes de prune).
func TestPreviewScratchOlderThan(t *testing.T) {
	s := openTestStore(t)
	// permanente-velha: scratch=false (não conta no preview)
	if err := s.Upsert(Memory{Agent: "codex", Type: "project", Name: "permanente-velha", Description: "d", Content: "c", Scratch: false}); err != nil {
		t.Fatal(err)
	}
	// scratch-velha: scratch=true, accessed_at velho (deve aparecer no preview)
	if err := s.Upsert(Memory{Agent: "claude-code", Type: "event", Name: "scratch-velha", Description: "d", Content: "c", Scratch: true}); err != nil {
		t.Fatal(err)
	}
	// scratch-recente: scratch=true mas accessed_at recente (não deve aparecer)
	if err := s.Upsert(Memory{Agent: "claude-code", Type: "event", Name: "scratch-recente", Description: "d", Content: "c", Scratch: true}); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-60 * 24 * time.Hour).Format(time.RFC3339)
	if _, err := s.db.Exec(`UPDATE memories SET accessed_at = ? WHERE name IN ('permanente-velha', 'scratch-velha')`, old); err != nil {
		t.Fatal(err)
	}

	prev, err := s.PreviewScratchOlderThan(30*24*time.Hour, 10)
	if err != nil {
		t.Fatalf("PreviewScratchOlderThan: %v", err)
	}
	if prev.Total != 1 {
		t.Fatalf("esperava Total=1, obtive %d", prev.Total)
	}
	if len(prev.Entries) != 1 {
		t.Fatalf("esperava 1 entry, obtive %d", len(prev.Entries))
	}
	if prev.Entries[0].Name != "scratch-velha" {
		t.Fatalf("esperava scratch-velha, obtive %s", prev.Entries[0].Name)
	}
	// Confirma read-only: a entrada ainda existe no store.
	if got, _ := s.Get("scratch-velha"); got == nil {
		t.Fatal("Preview alterou o store (read-only esperado)")
	}
}

// TestStats valida agregações por type/agent/scratch + idade (A-67).
func TestStats(t *testing.T) {
	s := openTestStore(t)
	// Store vazio: total=0, mapas vazios.
	st, err := s.Stats()
	if err != nil {
		t.Fatalf("Stats empty: %v", err)
	}
	if st.Total != 0 {
		t.Fatalf("store vazio: esperava Total=0, obtive %d", st.Total)
	}
	if st.ByScratch == nil || len(st.ByScratch) != 0 {
		t.Fatalf("ByScratch deve ser map vazio (não nil), obtive %+v", st.ByScratch)
	}

	// 2 permanentes (tipos diferentes) + 3 scratch (tipos diferentes + agents diferentes)
	inputs := []Memory{
		{Agent: "claude-code", Type: "project", Name: "p1", Description: "d", Content: "c", Scratch: false},
		{Agent: "codex", Type: "feedback", Name: "f1", Description: "d", Content: "c", Scratch: false},
		{Agent: "claude-code", Type: "event", Name: "e1", Description: "d", Content: "c", Scratch: true},
		{Agent: "claude-code", Type: "event", Name: "e2", Description: "d", Content: "c", Scratch: true},
		{Agent: "codex", Type: "event", Name: "e3", Description: "d", Content: "c", Scratch: true},
	}
	for _, m := range inputs {
		if err := s.Upsert(m); err != nil {
			t.Fatalf("Upsert %s: %v", m.Name, err)
		}
	}

	st, err = s.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if st.Total != 5 {
		t.Fatalf("esperava Total=5, obtive %d", st.Total)
	}
	if st.ByScratch["permanent"] != 2 || st.ByScratch["scratch"] != 3 {
		t.Fatalf("ByScratch errado: %+v (esperava permanent=2, scratch=3)", st.ByScratch)
	}
	if st.ByType["project"] != 1 || st.ByType["feedback"] != 1 || st.ByType["event"] != 3 {
		t.Fatalf("ByType errado: %+v", st.ByType)
	}
	if st.ByAgent["claude-code"] != 3 || st.ByAgent["codex"] != 2 {
		t.Fatalf("ByAgent errado: %+v", st.ByAgent)
	}
	if st.OldestUpdateAt == "" || st.NewestUpdateAt == "" {
		t.Fatalf("oldest/newest nao devem estar vazios: %+v", st)
	}
	if st.OldestUpdateAt > st.NewestUpdateAt {
		t.Fatalf("oldest > newest: %s > %s", st.OldestUpdateAt, st.NewestUpdateAt)
	}
}

func TestListEvents(t *testing.T) {
	s := openTestStore(t)

	// Memória comum (não é evento) — não deve aparecer em ListEvents.
	if err := s.Upsert(Memory{
		Agent: "claude-code", Type: "project", Name: "contexto-x",
		Description: "decisão", Content: "use Postgres",
	}); err != nil {
		t.Fatalf("Upsert projeto: %v", err)
	}

	// Três eventos com kinds diferentes.
	for _, ev := range []struct{ kind, note string }{
		{"decision", "optou por Postgres"},
		{"guard_nudge", "context-guard #42"},
		{"task_completed", "implementou schema"},
	} {
		if err := s.Upsert(Memory{
			Agent: "claude-code", Type: "event", Name: ev.kind + "-20260920T150000Z-abcdef01",
			Description: ev.note, Content: ev.note,
			Scratch: true,
		}); err != nil {
			t.Fatalf("Upsert %s: %v", ev.kind, err)
		}
	}

	// Sem filtros: deve trazer 3 eventos (não a memória "contexto-x").
	all, err := s.ListEvents("", "", 0)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("ListEvents retornou %d, esperado 3", len(all))
	}
	for _, m := range all {
		if m.Type != "event" {
			t.Fatalf("ListEvents trouxe memória não-evento: %+v", m)
		}
	}

	// Filtro por kind: só os "guard_nudge".
	onlyNudges, err := s.ListEvents("guard_nudge", "", 0)
	if err != nil {
		t.Fatalf("ListEvents kind: %v", err)
	}
	if len(onlyNudges) != 1 || onlyNudges[0].Name != "guard_nudge-20260920T150000Z-abcdef01" {
		t.Fatalf("ListEvents kind=%q retornou %+v", "guard_nudge", onlyNudges)
	}

	// since futuro: lista vazia.
	future := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	none, err := s.ListEvents("", future, 0)
	if err != nil {
		t.Fatalf("ListEvents since future: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("ListEvents since future trouxe %d, esperado 0", len(none))
	}
}
