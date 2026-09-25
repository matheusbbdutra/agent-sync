package repomap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// helper: cria um repositório temporário com alguns arquivos rastreáveis.
type fakeRepo struct {
	root  string
	cache string
	files map[string]string
}

func newFakeRepo(t *testing.T) *fakeRepo {
	t.Helper()
	dir := t.TempDir()
	cache := filepath.Join(dir, ".agent-sync", "cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatalf("mkdir cache: %v", err)
	}
	r := &fakeRepo{root: dir, cache: cache, files: map[string]string{}}
	t.Cleanup(func() {
		// best-effort: nada a fazer, t.TempDir limpa
	})
	return r
}

func (r *fakeRepo) write(t *testing.T, rel, body string) {
	t.Helper()
	full := filepath.Join(r.root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	r.files[rel] = body
}

func TestUpdateIdempotente(t *testing.T) {
	r := newFakeRepo(t)
	r.write(t, "main.go", "package main\n\nfunc Hello() string { return \"hi\" }\n")
	r.write(t, "lib/util.go", "package lib\n\nfunc Add(a, b int) int { return a + b }\n")

	c1, s1, err := Update(r.root, r.cache)
	if err != nil {
		t.Fatalf("update 1: %v", err)
	}
	if s1.Reparsed != 2 || s1.Reused != 0 {
		t.Errorf("primeira passada esperava 2 re-parseados / 0 reusados, obtido %+v", s1)
	}
	if _, ok := c1.Files["main.go"]; !ok {
		t.Errorf("main.go ausente do cache")
	}

	c2, s2, err := Update(r.root, r.cache)
	if err != nil {
		t.Fatalf("update 2: %v", err)
	}
	if s2.Reparsed != 0 || s2.Reused != 2 {
		t.Errorf("segunda passada esperava 0 re-parseados / 2 reusados, obtido %+v", s2)
	}
	if len(c2.Files) != 2 {
		t.Errorf("cache deveria ter 2 arquivos, tem %d", len(c2.Files))
	}
}

func TestUpdateInvalidaPorMtime(t *testing.T) {
	r := newFakeRepo(t)
	r.write(t, "a.go", "package a\n")
	if _, _, err := Update(r.root, r.cache); err != nil {
		t.Fatal(err)
	}

	past := time.Unix(0, 0)
	if err := os.Chtimes(filepath.Join(r.root, "a.go"), past, past); err != nil {
		t.Fatal(err)
	}

	_, s, err := Update(r.root, r.cache)
	if err != nil {
		t.Fatal(err)
	}
	if s.Reparsed != 1 || s.Reused != 0 {
		t.Errorf("esperava 1 re-parseado / 0 reusado após mudança de mtime, obtido %+v", s)
	}
}

func TestUpdateDetectaRemocao(t *testing.T) {
	r := newFakeRepo(t)
	r.write(t, "x.go", "package x\n")
	r.write(t, "y.go", "package y\n")
	if _, _, err := Update(r.root, r.cache); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(filepath.Join(r.root, "x.go")); err != nil {
		t.Fatal(err)
	}
	c, s, err := Update(r.root, r.cache)
	if err != nil {
		t.Fatal(err)
	}
	if s.Removed != 1 {
		t.Errorf("esperava 1 removido, obtido %+v", s)
	}
	if _, ok := c.Files["x.go"]; ok {
		t.Errorf("x.go deveria ter sido removido do cache")
	}
}

func TestExtractGo(t *testing.T) {
	src := []byte(`package sample

type User struct{ Name string }
type Repo interface{ Get() string }

func NewUser() *User { return &User{} }

func (u *User) GetName() string { return u.Name }
`)
	syms, imps, calls, _, _ := extractStructured("sample.go", src)
	for _, want := range []string{"User", "Repo", "NewUser", "User.GetName"} {
		if !contains(syms, want) {
			t.Errorf("esperava símbolo %q em %v", want, syms)
		}
	}
	if len(imps) != 0 {
		t.Errorf("não esperava imports, obteve %v", imps)
	}
	if !contains(calls, "NewUser") {
		t.Errorf("esperava NewUser em calls: %v", calls)
	}
}

func TestExtractGoImports(t *testing.T) {
	src := []byte(`package sample

import (
	"fmt"
	"github.com/x/y"
)

func main() { fmt.Println("hi") }
`)
	_, imps, _, _, _ := extractStructured("main.go", src)
	for _, want := range []string{"fmt", "github.com/x/y"} {
		if !contains(imps, want) {
			t.Errorf("esperava import %q em %v", want, imps)
		}
	}
}

func TestExtractPy(t *testing.T) {
	src := []byte(`from foo.bar import Baz

class Service:
    pass

def calculate(x):
    return x + 1
`)
	syms, imps, _, _, _ := extractStructured("svc.py", src)
	for _, want := range []string{"Service", "calculate"} {
		if !contains(syms, want) {
			t.Errorf("esperava símbolo %q em %v", want, syms)
		}
	}
	if !contains(imps, "foo.bar") {
		t.Errorf("esperava import foo.bar em %v", imps)
	}
}

func TestExtractTS(t *testing.T) {
	src := []byte(`import { Foo } from "./foo";
const bar = require("./bar");

export interface IRepo {}
export type TRepo = string;
export class Repo { get() { return 1; } }
export function makeRepo(): Repo { return new Repo(); }
`)
	syms, imps, _, _, _ := extractStructured("repo.ts", src)
	for _, want := range []string{"IRepo", "TRepo", "Repo", "makeRepo", "bar"} {
		if !contains(syms, want) {
			t.Errorf("esperava símbolo %q em %v", want, syms)
		}
	}
	for _, want := range []string{"./foo", "./bar"} {
		if !contains(imps, want) {
			t.Errorf("esperava import %q em %v", want, imps)
		}
	}
}

func TestExtractPHP(t *testing.T) {
	src := []byte(`<?php
namespace App;
use Foo\Bar;

final class User {}
interface Repository {}
trait Timestampable {}

public function makeUser(): User { return new User(); }
`)
	syms, imps, _, _, _ := extractStructured("user.php", src)
	for _, want := range []string{"User", "Repository", "Timestampable", "makeUser"} {
		if !contains(syms, want) {
			t.Errorf("esperava símbolo %q em %v", want, syms)
		}
	}
	if !contains(imps, "Foo/Bar") {
		t.Errorf("esperava Foo/Bar em %v", imps)
	}
}

func TestFocusMostraImporters(t *testing.T) {
	r := newFakeRepo(t)
	r.write(t, "lib/lib.go", "package lib\n\nfunc Lib() {}\n")
	r.write(t, "main.go", "package main\n\nimport \"lib\"\n\nfunc main() { lib.Lib() }\n")
	if _, _, err := Update(r.root, r.cache); err != nil {
		t.Fatal(err)
	}
	c, err := Load(r.cache)
	if err != nil || c == nil {
		t.Fatalf("load cache: %v", err)
	}
	out := Focus(c, "lib/lib.go", 1)
	if !strings.Contains(out, "Symbols") {
		t.Errorf("focus deveria listar symbols, obtido:\n%s", out)
	}
	if !strings.Contains(out, "Importers") {
		t.Errorf("focus deveria listar Importers, obtido:\n%s", out)
	}
	if !strings.Contains(out, "main.go") {
		t.Errorf("focus deveria apontar main.go como importer, obtido:\n%s", out)
	}
}

func TestSummaryOrdenaPorScore(t *testing.T) {
	r := newFakeRepo(t)
	r.write(t, "a.go", "package a\nfunc A() {}\n")
	r.write(t, "b.go", "package b\n\nimport \"a\"\n\nfunc B() { A(); A(); A() }\n")
	r.write(t, "c.go", "package c\n\nimport \"a\"\n\nfunc C() { A() }\n")
	if _, _, err := Update(r.root, r.cache); err != nil {
		t.Fatal(err)
	}
	c, err := Load(r.cache)
	if err != nil || c == nil {
		t.Fatal(err)
	}
	out := Summary(c, 0)
	if !strings.Contains(out, "Top hubs") {
		t.Errorf("summary deveria começar com 'Top hubs', obtido:\n%s", out)
	}
	if !strings.Contains(out, "A") {
		t.Errorf("summary deveria listar A, obtido:\n%s", out)
	}
}

func TestFocusResolvePorSufixo(t *testing.T) {
	r := newFakeRepo(t)
	r.write(t, "internal/svc/svc.go", "package svc\n\nfunc Run() {}\n")
	if _, _, err := Update(r.root, r.cache); err != nil {
		t.Fatal(err)
	}
	c, err := Load(r.cache)
	if err != nil || c == nil {
		t.Fatal(err)
	}
	out := Focus(c, "svc.go", 1)
	if !strings.Contains(out, "internal/svc/svc.go") {
		t.Errorf("focus deveria resolver por sufixo, obtido:\n%s", out)
	}
}

func TestLoadCacheInexistenteRetornaNil(t *testing.T) {
	r := newFakeRepo(t)
	c, err := Load(r.cache)
	if err != nil {
		t.Fatalf("load sem cache nao deveria falhar: %v", err)
	}
	if c != nil {
		t.Errorf("esperava cache nil quando ausente, obteve %+v", c)
	}
}

func TestSaveELoadRoundtrip(t *testing.T) {
	r := newFakeRepo(t)
	c := NewCache(r.root)
	c.Files["x.go"] = &FileEntry{
		MTimeNs: 1, Size: 2, Hash: "abc",
		Symbols: []string{"Foo"}, Imports: []string{"bar"}, Calls: []string{"Baz"},
	}
	c.Edges["x.go::Foo"] = []string{"Baz"}
	if err := Save(r.cache, c); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(r.cache)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded == nil || loaded.Files["x.go"] == nil {
		t.Fatalf("roundtrip falhou: %+v", loaded)
	}
	if loaded.Files["x.go"].Symbols[0] != "Foo" {
		t.Errorf("esperava Foo, obteve %v", loaded.Files["x.go"].Symbols)
	}
}

func contains(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

func TestExtractTables(t *testing.T) {
	sqlSrc := []byte(`
		CREATE TABLE users (id INT PRIMARY KEY);
		ALTER TABLE orders ADD COLUMN user_id INT;
		SELECT * FROM payments JOIN invoices ON payments.invoice_id = invoices.id;
		INSERT INTO audit_logs (action) VALUES ('test');
	`)
	tables := extractTables(sqlSrc)
	for _, want := range []string{"users", "orders", "payments", "invoices", "audit_logs"} {
		if !contains(tables, want) {
			t.Errorf("esperava tabela %q em %v", want, tables)
		}
	}
}

func TestExtractEnvVars(t *testing.T) {
	goSrc := []byte(`
		port := os.Getenv("PORT")
		dbUrl, _ := os.LookupEnv("DATABASE_URL")
	`)
	envs := extractEnvVars(goSrc)
	for _, want := range []string{"PORT", "DATABASE_URL"} {
		if !contains(envs, want) {
			t.Errorf("esperava env %q em %v", want, envs)
		}
	}

	tsSrc := []byte(`
		const api = process.env.API_KEY;
		const secret = process.env["SECRET_TOKEN"];
	`)
	tsEnvs := extractEnvVars(tsSrc)
	for _, want := range []string{"API_KEY", "SECRET_TOKEN"} {
		if !contains(tsEnvs, want) {
			t.Errorf("esperava env %q em %v", want, tsEnvs)
		}
	}
}

func TestBlastRadius(t *testing.T) {
	c := NewCache("/test")
	c.Files["pkg/db.go"] = &FileEntry{
		Tables: []string{"users"},
	}
	c.Files["pkg/auth.go"] = &FileEntry{
		Imports: []string{"pkg/db"},
		Tables:  []string{"users"},
	}

	level, count := calculateBlastRadius(c, "pkg/db.go", []string{"pkg/auth.go"}, []string{"users"})
	if level != "low" {
		t.Errorf("esperava level low para 1 importer + 1 table share, obteve %s", level)
	}
	if count != 2 { // 1 importer + 1 table share
		t.Errorf("esperava count 2, obteve %d", count)
	}

	// Test high for main.go
	levelMain, _ := calculateBlastRadius(c, "cmd/main.go", nil, nil)
	if levelMain != "high" {
		t.Errorf("esperava level high para main.go, obteve %s", levelMain)
	}
}

func TestDetectTestCommand(t *testing.T) {
	cmd1 := detectTestCommand("tools/internal/repomap/repomap.go", []string{"tools/internal/repomap/repomap_test.go"})
	if cmd1 != "cd tools && go test ./internal/repomap/..." {
		t.Errorf("esperava comando de teste cirúrgico para tools, obteve: %q", cmd1)
	}

	cmd2 := detectTestCommand("cmd/agent-sync/main.go", []string{"cmd/agent-sync/main_test.go"})
	if cmd2 != "go test ./cmd/agent-sync/..." {
		t.Errorf("esperava comando de teste para cmd/agent-sync, obteve: %q", cmd2)
	}

	cmd3 := detectTestCommand("src/component.tsx", []string{"src/component.test.tsx"})
	if cmd3 != "npm test -- src/component.test.tsx" {
		t.Errorf("esperava npm test para tsx, obteve: %q", cmd3)
	}
}

func TestBriefPacketWithNewFields(t *testing.T) {
	c := NewCache("/test")
	c.Files["pkg/service.go"] = &FileEntry{
		Symbols: []string{"DoWork"},
		Imports: []string{"os"},
		Tables:  []string{"accounts"},
		EnvVars: []string{"API_KEY"},
	}
	c.Files["pkg/service_test.go"] = &FileEntry{
		Symbols: []string{"TestDoWork"},
	}

	out := Brief(c, "pkg/service.go", 0)
	if !strings.Contains(out, `"blast_radius":`) {
		t.Errorf("esperava campo blast_radius no Brief: %s", out)
	}
	if !strings.Contains(out, `"test_command": "go test ./pkg/..."`) {
		t.Errorf("esperava test_command correto no Brief: %s", out)
	}
	if !strings.Contains(out, `"accounts"`) {
		t.Errorf("esperava tabela accounts no Brief: %s", out)
	}
	if !strings.Contains(out, `"API_KEY"`) {
		t.Errorf("esperava env API_KEY no Brief: %s", out)
	}
}

