package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"foo.py", "py"},
		{"foo.PY", "py"},
		{"foo.pyi", "py"},
		{"foo.ts", "ts"},
		{"foo.tsx", "ts"},
		{"foo.js", "js"},
		{"foo.jsx", "js"},
		{"foo.go", "go"},
		{"foo.md", ""},
		{"foo", ""},
		{"foo.txt", ""},
	}
	for _, c := range cases {
		if got := detectLanguage(c.path); got != c.want {
			t.Errorf("detectLanguage(%q)=%q, want %q", c.path, got, c.want)
		}
	}
}

func TestSkeletonPython(t *testing.T) {
	src := `#!/usr/bin/env python3
"""module docstring"""
import os
import json
from typing import List

MAX_RETRIES = 3
DEFAULT_TIMEOUT = 30

class UserService:
    """handles user CRUD"""
    def __init__(self, db):
        self.db = db

    async def fetch(self, user_id: int) -> dict:
        """fetch user by id"""
        return await self.db.get(user_id)

@decorator
def helper(x: int) -> int:
    return x * 2
`
	skel := skeletonPython(strings.Split(src, "\n"))
	for _, want := range []string{"import os", "from typing", "class UserService", "def __init__", "async def fetch", "@decorator", "MAX_RETRIES", "def helper"} {
		if !strings.Contains(skel, want) {
			t.Errorf("skeletonPython missing %q\n--- output ---\n%s", want, skel)
		}
	}
	// número de linha presente (def __init__ está em L12 conforme fixture acima)
	if !strings.Contains(skel, "L12:") {
		t.Errorf("skeletonPython should include line numbers, got %q", skel)
	}
}

func TestSkeletonTSJS(t *testing.T) {
	src := `import { foo } from 'bar';
import * as utils from './utils';

const ARROW = () => 1;

export function greet(name: string): string {
  return "hi " + name;
}

export class UserRepo {
  constructor(private db: Db) {}
}

interface User {
  id: number;
  name: string;
}

type Handler = (req: Request) => Response;

export const handler: Handler = (req) => {
  return new Response("ok");
};

function internal() {}
`
	skel := skeletonTSJS(strings.Split(src, "\n"), "ts")
	for _, want := range []string{"import { foo }", "export function greet", "export class UserRepo", "interface User", "type Handler", "export const handler", "function internal"} {
		if !strings.Contains(skel, want) {
			t.Errorf("skeletonTSJS missing %q", want)
		}
	}
}

func TestSkeletonGo(t *testing.T) {
	src := `// Package foo does X.
package foo

import (
	"context"
	"fmt"
)

const MaxRetries = 3

var DefaultTimeout = 30 * time.Second

type Config struct {
	Host string
	Port int
}

type Server interface {
	Start(ctx context.Context) error
}

// New returns a new Server.
func New(cfg Config) Server {
	return &server{cfg: cfg}
}

func (s *server) Start(ctx context.Context) error {
	if s.cfg.Host == "" {
		return fmt.Errorf("empty host")
	}
	return nil
}
`
	skel := skeletonGo(strings.Split(src, "\n"))
	for _, want := range []string{"package foo", "import (", "const MaxRetries", "var DefaultTimeout", "type Config struct", "type Server interface", "func New", "func (s *server) Start"} {
		if !strings.Contains(skel, want) {
			t.Errorf("skeletonGo missing %q", want)
		}
	}
}

func TestIsConstDecl(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"MAX_RETRIES = 3", true},
		{"DEFAULT_TIMEOUT = 30", true},
		{"FOO_BAR = 1 + 2", true},
		{"x = 1", false},         // lowercase
		{"_PRIVATE = 1", true},   // _ é char válido; Python aceita como privado por convenção
		{"123 = 1", false},        // começa com dígito — não é identificador válido
		{"name = 'foo'", false},  // lowercase
		{"", false},
		{"no_equals", false},
	}
	for _, c := range cases {
		if got := isConstDecl(c.line); got != c.want {
			t.Errorf("isConstDecl(%q)=%v, want %v", c.line, got, c.want)
		}
	}
}

func TestBytesLines(t *testing.T) {
	cases := []struct {
		body string
		want int
	}{
		{"", 0},
		{"a", 1},
		{"a\n", 2},
		{"a\nb\nc", 3},
		{"a\nb\nc\n", 4},
	}
	for _, c := range cases {
		if got := bytesLines([]byte(c.body)); got != c.want {
			t.Errorf("bytesLines(%q)=%d, want %d", c.body, got, c.want)
		}
	}
}

func TestReductionPct(t *testing.T) {
	cases := []struct {
		orig, reduced, want int
	}{
		{100, 50, 50},
		{100, 0, 100},
		{100, 100, 0},
		{0, 50, 0},    // orig=0 → 0
		{100, 200, 0}, // reduced > orig → clamp 0
		{1000, 333, 66},
	}
	for _, c := range cases {
		if got := reductionPct(c.orig, c.reduced); got != c.want {
			t.Errorf("reductionPct(%d,%d)=%d, want %d", c.orig, c.reduced, got, c.want)
		}
	}
}

func TestSkeletonizeSmallFileReturnsContent(t *testing.T) {
	withTempCache(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "small.py")
	if err := os.WriteFile(path, []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Skeletonize(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.ReductionPct != 0 {
		t.Errorf("small file should have 0%% reduction, got %d", res.ReductionPct)
	}
	if !strings.Contains(res.Skeleton, "x = 1") {
		t.Errorf("small file should return content, got %q", res.Skeleton)
	}
}

func TestSkeletonizeUnsupportedLang(t *testing.T) {
	withTempCache(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "big.md")
	// força >8KB para cair no caminho do skeletonizer
	body := strings.Repeat("# heading\nlorem ipsum dolor sit amet\n", 300)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Skeletonize(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Language != "" {
		t.Errorf(".md should be unsupported, got language=%q", res.Language)
	}
	if res.ReductionPct != 0 {
		t.Errorf("unsupported lang should return content unchanged, got %d%% reduction", res.ReductionPct)
	}
}

func TestSkeletonizePythonReduces(t *testing.T) {
	withTempCache(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "big.py")
	// > 8KB (skeletonMinBytes) com código cheio (defs + bodies longos)
	var b strings.Builder
	for i := 0; i < 200; i++ {
		b.WriteString("def function_" + string(rune('A'+i%26)) + string(rune('A'+(i/26)%26)) + "():\n")
		b.WriteString("    x = " + strings.Repeat("1", 200) + "\n")
		b.WriteString("    y = " + strings.Repeat("2", 200) + "\n")
		b.WriteString("    return x + y\n\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if len(b.String()) < skeletonMinBytes {
		t.Fatalf("fixture should exceed skeletonMinBytes (%d), got %d", skeletonMinBytes, len(b.String()))
	}
	res, err := Skeletonize(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.ReductionPct < 50 {
		t.Errorf("expected ≥50%% reduction, got %d%% (orig=%d, skel=%d)", res.ReductionPct, res.Bytes, res.SkeletonChars)
	}
	if res.FromCache {
		t.Errorf("first call should not be from cache")
	}
}

func TestSkeletonizeCacheHit(t *testing.T) {
	withTempCache(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "cached.py")
	var b strings.Builder
	for i := 0; i < 200; i++ {
		b.WriteString("def f_" + string(rune('A'+i%26)) + string(rune('A'+(i/26)%26)) + "():\n")
		b.WriteString("    return " + strings.Repeat("1", 200) + "\n\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if len(b.String()) < skeletonMinBytes {
		t.Skip("fixture too small for skeleton cache test")
	}
	first, err := Skeletonize(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if first.FromCache {
		t.Fatal("first call should not be cache hit")
	}
	second, err := Skeletonize(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if !second.FromCache {
		t.Error("second call should be cache hit")
	}
	if first.Skeleton != second.Skeleton {
		t.Error("cached skeleton should match original")
	}
}

func TestSkeletonizeForceBypassesCache(t *testing.T) {
	withTempCache(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "forced.py")
	var b strings.Builder
	for i := 0; i < 200; i++ {
		b.WriteString("def h_" + string(rune('A'+i%26)) + string(rune('A'+(i/26)%26)) + "():\n")
		b.WriteString("    return " + strings.Repeat("1", 200) + "\n\n")
	}
	os.WriteFile(path, []byte(b.String()), 0o644)
	if len(b.String()) < skeletonMinBytes {
		t.Skip("fixture too small for skeleton force test")
	}
	Skeletonize(path, false) // warm cache
	res, err := Skeletonize(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.FromCache {
		t.Error("--force should bypass cache")
	}
}

func TestSkeletonizeNotFound(t *testing.T) {
	withTempCache(t)
	if _, err := Skeletonize("/nonexistent/does-not-exist.py", false); err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestSkeletonizeDirectory(t *testing.T) {
	withTempCache(t)
	dir := t.TempDir()
	if _, err := Skeletonize(dir, false); err == nil {
		t.Error("expected error for directory")
	}
}

func TestRunSkeletonPlain(t *testing.T) {
	withTempCache(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "sub.go")
	var b strings.Builder
	for i := 0; i < 50; i++ {
		b.WriteString("func F" + string(rune('A'+i%26)) + "() {}\n\n")
	}
	os.WriteFile(path, []byte(b.String()), 0o644)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"skeleton", path}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	for _, want := range []string{"skeleton of", "language:", "from cache:", "--- skeleton ---"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestRunSkeletonMissingFile(t *testing.T) {
	withTempCache(t)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"skeleton"}, &stdout, &stderr); err == nil {
		t.Fatal("expected error when file is missing")
	}
}