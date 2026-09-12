package astoutline

import (
	"strings"
	"testing"
)

func TestExtractGo(t *testing.T) {
	src := `package sample

type User struct {
	Name string
}

func (u *User) GetName() string {
	return u.Name
}
`
	out, err := Extract("sample.go", []byte(src))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !strings.Contains(out, "struct User") {
		t.Errorf("esperava conter 'struct User', obtido:\n%s", out)
	}
	if !strings.Contains(out, "func (*User) GetName(...)") {
		t.Errorf("esperava conter 'func (*User) GetName(...)', obtido:\n%s", out)
	}
}

func TestExtractPython(t *testing.T) {
	src := `class OrderService:
    pass

def calculate_total(items):
    pass
`
	out, err := Extract("service.py", []byte(src))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !strings.Contains(out, "class OrderService:") {
		t.Errorf("esperava conter 'class OrderService:', obtido:\n%s", out)
	}
	if !strings.Contains(out, "def calculate_total") {
		t.Errorf("esperava conter 'def calculate_total', obtido:\n%s", out)
	}
}
