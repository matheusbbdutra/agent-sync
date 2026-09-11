package main

import (
	"strings"
	"testing"
)

func TestSanitizeQueryBlocksMutations(t *testing.T) {
	cases := []string{
		"UPDATE users SET name = 'x' WHERE id = 1",
		"delete from users",
		"DROP TABLE users",
		"ALTER TABLE users ADD COLUMN age int",
		"TRUNCATE TABLE logs",
		"INSERT INTO users (id) VALUES (1)",
		"GRANT ALL ON users TO admin",
	}
	for _, query := range cases {
		if _, _, err := sanitizeQuery(query, 20); err == nil {
			t.Errorf("esperava bloqueio para %q", query)
		}
	}
}

func TestSanitizeQueryAllowsSelect(t *testing.T) {
	got, notices, err := sanitizeQuery("SELECT id, name FROM users", 20)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !strings.HasSuffix(strings.TrimSpace(got), "LIMIT 20;") {
		t.Fatalf("esperava LIMIT injetado, got %q", got)
	}
	if len(notices) != 1 {
		t.Fatalf("esperava 1 aviso de LIMIT, got %v", notices)
	}
}

func TestSanitizeQueryPreservesExistingLimit(t *testing.T) {
	got, notices, err := sanitizeQuery("SELECT id FROM users LIMIT 5", 20)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got != "SELECT id FROM users LIMIT 5" {
		t.Fatalf("não deveria alterar query com LIMIT, got %q", got)
	}
	if len(notices) != 0 {
		t.Fatalf("esperava nenhum aviso, got %v", notices)
	}
}

func TestSanitizeQueryWarnsSelectStar(t *testing.T) {
	_, notices, err := sanitizeQuery("SELECT * FROM users", 10)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	found := false
	for _, n := range notices {
		if strings.Contains(n, "SELECT *") {
			found = true
		}
	}
	if !found {
		t.Fatalf("esperava aviso de SELECT *, got %v", notices)
	}
}
