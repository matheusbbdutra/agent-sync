package agentmemory

import (
	"path/filepath"
	"testing"
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
