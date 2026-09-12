package main

import (
	"strings"
	"testing"

	"github.com/matheusdutra/token-tools/internal/astoutline"
)

func TestExtractGo(t *testing.T) {
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
	out, err := astoutline.Extract("sample.go", src)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	for _, want := range []string{"struct Server", "interface Greeter", "func (*Server) Start(...)", "func NewServer(...)"} {
		if !strings.Contains(out, want) {
			t.Errorf("faltando %q em:\n%s", want, out)
		}
	}
}

func TestExtractPython(t *testing.T) {
	src := []byte(`class Foo:
    def bar(self):
        pass

def baz(x):
    return x
`)
	out, err := astoutline.Extract("foo.py", src)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !strings.Contains(out, "class Foo:") {
		t.Errorf("esperava conter class Foo, obtido:\n%s", out)
	}
	if !strings.Contains(out, "def baz") {
		t.Errorf("esperava conter def baz, obtido:\n%s", out)
	}
}
