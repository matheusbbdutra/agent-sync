package agentmemory

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func openTestRemote(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("libsql", "file:"+filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSyncBetweenTwoPCsAndConflict(t *testing.T) {
	ctx := context.Background()
	remote := openTestRemote(t)
	first := openTestStore(t)
	second := openTestStore(t)
	original := Memory{Agent: "codex", Type: "project", Name: "decision", Description: "Decisão", Content: "versão A"}
	if err := first.Upsert(original); err != nil {
		t.Fatal(err)
	}
	if sent, err := first.Push(ctx, remote); err != nil || sent != 1 {
		t.Fatalf("push inicial: %d %v", sent, err)
	}
	if got, err := second.Pull(ctx, remote); err != nil || got != 1 {
		t.Fatalf("pull segundo PC: %d %v", got, err)
	}
	original.Content = "versão B"
	if err := first.Upsert(original); err != nil {
		t.Fatal(err)
	}
	if sent, err := first.Push(ctx, remote); err != nil || sent != 1 {
		t.Fatalf("push atualizado: %d %v", sent, err)
	}
	if got, err := second.Pull(ctx, remote); err != nil || got != 1 {
		t.Fatalf("pull atualizado: %d %v", got, err)
	}
	memory, err := second.Get("decision")
	if err != nil || memory.Content != "versão B" {
		t.Fatalf("conteúdo não atualizado: %+v %v", memory, err)
	}
	original.Content = "versão C"
	if err := first.Upsert(original); err != nil {
		t.Fatal(err)
	}
	if err := second.Upsert(Memory{Agent: "codex", Type: "project", Name: "decision", Description: "Decisão", Content: "versão D"}); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Push(ctx, remote); err != nil {
		t.Fatal(err)
	}
	if _, err := second.Push(ctx, remote); err == nil || !strings.Contains(err.Error(), "conflito") {
		t.Fatalf("esperava conflito: %v", err)
	}
	memory, err = second.Get("decision")
	if err != nil || memory.Content != "versão D" {
		t.Fatalf("conflito alterou dado local: %+v %v", memory, err)
	}
	if err := second.Resolve(ctx, remote, conflictID(memoryKey{"", "project", "decision"}), "remote"); err != nil {
		t.Fatal(err)
	}
	memory, err = second.Get("decision")
	if err != nil || memory.Content != "versão C" {
		t.Fatalf("resolução remota falhou: %+v %v", memory, err)
	}
}

func TestSyncSkipsScratch(t *testing.T) {
	ctx := context.Background()
	remote := openTestRemote(t)
	store := openTestStore(t)
	if err := store.Upsert(Memory{Agent: "codex", Type: "project", Name: "temporary", Content: "rascunho", Scratch: true}); err != nil {
		t.Fatal(err)
	}
	if sent, err := store.Push(ctx, remote); err != nil || sent != 0 {
		t.Fatalf("scratch foi enviado: %d %v", sent, err)
	}
}

func TestOpenRemoteRejectsInvalidURL(t *testing.T) {
	for _, address := range []string{"", "file:/tmp/local.db", "http://example.com", "libsql://example.com?authToken=embedded"} {
		if _, err := OpenRemote(address, "fake-token"); err == nil {
			t.Fatalf("URL aceita: %s", address)
		}
	}
}

func TestPushRefusesKnownSecret(t *testing.T) {
	ctx := context.Background()
	remote := openTestRemote(t)
	store := openTestStore(t)
	if err := store.Upsert(Memory{Agent: "codex", Type: "project", Name: "sensitive", Content: "sk-abcdefghijklmnopqrstuvwxyz123456"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Push(ctx, remote); err == nil || !strings.Contains(err.Error(), "possível segredo") {
		t.Fatalf("esperava recusa de segredo: %v", err)
	}
	var count int
	if err := remote.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_sync_memories_v2`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("segredo enviado ao remoto: %d %v", count, err)
	}
}

func TestPushRestoresMissingRemoteRecord(t *testing.T) {
	ctx := context.Background()
	remote := openTestRemote(t)
	store := openTestStore(t)
	memory := Memory{Agent: "codex", Type: "project", Name: "recover", Content: "valor local"}
	if err := store.Upsert(memory); err != nil {
		t.Fatal(err)
	}
	if sent, err := store.Push(ctx, remote); err != nil || sent != 1 {
		t.Fatalf("envio inicial: %d %v", sent, err)
	}
	if _, err := remote.ExecContext(ctx, `DELETE FROM agent_sync_memories_v2 WHERE type=? AND name=?`, memory.Type, memory.Name); err != nil {
		t.Fatal(err)
	}
	if sent, err := store.Push(ctx, remote); err != nil || sent != 1 {
		t.Fatalf("recuperação: %d %v", sent, err)
	}
	var count int
	if err := remote.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_sync_memories_v2 WHERE type=? AND name=?`, memory.Type, memory.Name).Scan(&count); err != nil || count != 1 {
		t.Fatalf("registro remoto não recuperado: %d %v", count, err)
	}
}

func TestSyncKeepsProjectOriginAcrossPCs(t *testing.T) {
	ctx := context.Background()
	remote := openTestRemote(t)
	first := openTestStore(t)
	second := openTestStore(t)
	memories := []Memory{
		{Agent: "codex", Type: "project", Name: "decision", Content: "alpha", ProjectID: "shared", ProjectPath: "/pc-one/app", PC: "pc-one"},
		{Agent: "codex", Type: "project", Name: "decision", Content: "beta", ProjectID: "other", ProjectPath: "/pc-one/other", PC: "pc-one"},
	}
	for _, memory := range memories {
		if err := first.Upsert(memory); err != nil {
			t.Fatal(err)
		}
	}
	if sent, err := first.Push(ctx, remote); err != nil || sent != 2 {
		t.Fatalf("envio escopado: %d %v", sent, err)
	}
	if received, err := second.Pull(ctx, remote); err != nil || received != 2 {
		t.Fatalf("recebimento escopado: %d %v", received, err)
	}
	got, err := second.GetScoped("decision", "shared")
	if err != nil || got.PC != "pc-one" || got.ProjectPath != "/pc-one/app" {
		t.Fatalf("origem perdida: %+v %v", got, err)
	}
	if err := second.Upsert(Memory{Agent: "codex", Type: "project", Name: "decision", Content: "updated", ProjectID: "shared", ProjectPath: "/pc-two/app", PC: "pc-two"}); err != nil {
		t.Fatal(err)
	}
	if sent, err := second.Push(ctx, remote); err != nil || sent != 1 {
		t.Fatalf("atualização no segundo PC: %d %v", sent, err)
	}
	if received, err := first.Pull(ctx, remote); err != nil || received != 1 {
		t.Fatalf("retorno ao primeiro PC: %d %v", received, err)
	}
	got, err = first.GetScoped("decision", "shared")
	if err != nil || got.PC != "pc-two" || got.ProjectPath != "/pc-two/app" {
		t.Fatalf("última origem perdida: %+v %v", got, err)
	}
}

func TestSyncMigratesLegacyRemoteRows(t *testing.T) {
	ctx := context.Background()
	remote := openTestRemote(t)
	first := openTestStore(t)
	second := openTestStore(t)
	memory := Memory{Agent: "codex", Type: "project", Name: "legacy", Content: "existing"}
	if err := first.Upsert(memory); err != nil {
		t.Fatal(err)
	}
	if sent, err := first.Push(ctx, remote); err != nil || sent != 1 {
		t.Fatalf("envio inicial: %d %v", sent, err)
	}
	if _, err := remote.ExecContext(ctx, `INSERT INTO agent_sync_memories(id,agent,session_id,type,name,description,content,content_hash) VALUES(?,?,?,?,?,?,?,?)`, "legacy-id", memory.Agent, "", memory.Type, memory.Name, "", memory.Content, contentHash(memory)); err != nil {
		t.Fatal(err)
	}
	if _, err := remote.ExecContext(ctx, `DELETE FROM agent_sync_memories_v2 WHERE project_id='' AND type=? AND name=?`, memory.Type, memory.Name); err != nil {
		t.Fatal(err)
	}
	if received, err := second.Pull(ctx, remote); err != nil || received != 1 {
		t.Fatalf("migração remota: %d %v", received, err)
	}
	got, err := second.GetScoped("legacy", "")
	if err != nil || got == nil || got.ProjectID != "" {
		t.Fatalf("legado não preservado: %+v %v", got, err)
	}
}
