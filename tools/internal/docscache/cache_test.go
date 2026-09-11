package docscache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKeyDeterministic(t *testing.T) {
	a := Key("https://symfony.com/doc/current/index.html")
	b := Key("https://symfony.com/doc/current/index.html")
	c := Key("https://doctrine-project.org/")
	if a != b {
		t.Fatal("Key deveria ser determinística")
	}
	if a == c {
		t.Fatal("URLs diferentes deveriam gerar chaves diferentes")
	}
}

func TestSaveLoadListSearch(t *testing.T) {
	dir := t.TempDir()
	url := "https://example.com/docs"
	if err := Save(dir, url, "text/html", "<html>corpo</html>", "O ORM usa lazy loading\noutra linha"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	entry, ok := Load(dir, url)
	if !ok {
		t.Fatal("esperava Load encontrar a entrada")
	}
	if entry.URL != url || entry.Text == "" {
		t.Fatalf("entrada inesperada: %+v", entry)
	}

	list, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("esperava 1 entrada, got %d", len(list))
	}

	matches, err := Search(dir, "lazy", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(matches) != 1 || matches[0].URL != url || len(matches[0].Lines) != 1 {
		t.Fatalf("busca inesperada: %+v", matches)
	}
	if _, err := Search(dir, "inexistente", 10); err != nil {
		t.Fatalf("Search sem resultado não deveria errar: %v", err)
	}
}

func TestLoadMissing(t *testing.T) {
	if _, ok := Load(t.TempDir(), "https://n/a"); ok {
		t.Fatal("não deveria encontrar entrada inexistente")
	}
}

func TestListEmptyDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "noise.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	list, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("esperava lista vazia, got %d", len(list))
	}
}
