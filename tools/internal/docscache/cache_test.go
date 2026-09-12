package docscache

import (
	"os"
	"path/filepath"
	"strings"
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

func TestExtractSections(t *testing.T) {
	doc := `# Documentação Principal
Introdução sobre a ferramenta.

## Mapeamento Básico
O ORM mapeia entidades para tabelas.
Configurações adicionais aqui.

### Lazy Loading
Carregamento sob demanda economiza memória.`

	sections := ExtractSections(doc)
	if len(sections) != 3 {
		t.Fatalf("esperava 3 seções, obteve %d", len(sections))
	}

	if sections[1].Heading != "## Mapeamento Básico" || sections[1].Anchor != "mapeamento-basico" {
		t.Errorf("seção 1 incorreta: %+v", sections[1])
	}
	if sections[2].Heading != "### Lazy Loading" || sections[2].Anchor != "lazy-loading" {
		t.Errorf("seção 2 incorreta: %+v", sections[2])
	}
}

func TestSaveLoadListSearch(t *testing.T) {
	dir := t.TempDir()
	url := "https://example.com/docs"
	content := `# Exemplo de Doc
Introdução geral.

## Configurações do ORM
O ORM usa lazy loading por padrão.
Segunda linha explicativa.`

	if err := Save(dir, url, "text/html", "<html>corpo</html>", content); err != nil {
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
	if len(matches) != 1 {
		t.Fatalf("esperava 1 match, obteve %d", len(matches))
	}
	if !strings.HasSuffix(matches[0].URL, "#configuracoes-do-orm") {
		t.Errorf("esperava URL com âncora de seção, obteve: %s", matches[0].URL)
	}
	if matches[0].Heading != "## Configurações do ORM" {
		t.Errorf("esperava heading de seção, obteve: %s", matches[0].Heading)
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
