package main

import (
	"regexp"
	"strings"
	"testing"
)

func TestGoOutlineLines(t *testing.T) {
	src := []byte(`package sample

type Server struct {
	Name string
}

type Greeter interface {
	Greet() string
}

func (s *Server) Start(port int) error { return nil }

func NewServer() *Server { return &Server{} }
`)
	lines, pkg, err := goOutlineLines(src)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if pkg != "sample" {
		t.Fatalf("pacote = %q, want sample", pkg)
	}

	joined := ""
	for _, l := range lines {
		joined += l.String() + "\n"
	}
	for _, want := range []string{"struct Server", "interface Greeter", "func (*Server) Start(...)", "func NewServer(...)"} {
		if !strings.Contains(joined, want) {
			t.Errorf("faltando %q em:\n%s", want, joined)
		}
	}
}

func TestGoOutlineLinesInvalid(t *testing.T) {
	if _, _, err := goOutlineLines([]byte("not valid go")); err == nil {
		t.Fatal("esperava erro de parse")
	}
}

func TestRegexOutlinePython(t *testing.T) {
	src := `class Foo:
    def bar(self):
        pass

def baz(x):
    return x
`
	re := regexp.MustCompile(`(?m)^(?:class\s+\w+|def\s+\w+[\w\s,=*()]*:)`)
	lines := regexOutline(strings.NewReader(src), re)

	// O padrão é ancorado no início da linha, então métodos indentados não são listados.
	if len(lines) != 2 {
		t.Fatalf("esperava 2 linhas, got %d: %v", len(lines), lines)
	}
	if lines[0].Line != 1 || !strings.Contains(lines[0].Text, "class Foo") {
		t.Errorf("linha 0 inesperada: %v", lines[0])
	}
	if lines[1].Line != 5 || !strings.Contains(lines[1].Text, "def baz") {
		t.Errorf("linha 1 inesperada: %v", lines[1])
	}
}
