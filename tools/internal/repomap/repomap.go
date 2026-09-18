// Package repomap implementa um cache incremental estrutural de um repositório.
//
// O cache vive em <cacheDir>/repomap.json e armazena, para cada arquivo
// rastreado, o par (mtime_ns, size) usado para invalidação determinística,
// além de símbolos declarados, imports e calls extraídos via AST/regex.
//
// Três operações expostas:
//
//	Update   — varre o repositório (git ls-files ou fallback .gitignore),
//	           re-parseia apenas arquivos modificados e regrava o cache.
//	Focus    — devolve um subgrafo textual em torno de um arquivo-alvo.
//	Summary  — devolve os símbolos centrais (hubs de chamadas/imports).
//
// O pacote foi desenhado para ser leve (<30ms no update frio em repos
// pequenos/médios) e tolerante a arquivos malformados: parsing com falha
// não bloqueia o cache dos demais.
package repomap

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	CacheVersion    = 1
	CacheFileName   = "repomap.json"
	defaultMaxBytes = 1 << 20
)

// FileEntry representa o estado de um arquivo dentro do cache.
type FileEntry struct {
	MTimeNs int64    `json:"mtime_ns"`
	Size    int64    `json:"size"`
	Hash    string   `json:"hash"`
	Symbols []string `json:"symbols,omitempty"`
	Imports []string `json:"imports,omitempty"`
	Calls   []string `json:"calls,omitempty"`
}

// Cache é a representação serializada em disco.
type Cache struct {
	Version int                   `json:"version"`
	Root    string                `json:"root,omitempty"`
	Files   map[string]*FileEntry `json:"files"`
	Edges   map[string][]string   `json:"edges,omitempty"`
}

// UpdateStats sumariza o que mudou durante um Update.
type UpdateStats struct {
	Visited   int
	Reused    int
	Reparsed  int
	Removed   int
	EdgesNow  int
	ElapsedMs int64
}

// NewCache devolve um cache vazio para a raiz informada.
func NewCache(root string) *Cache {
	return &Cache{
		Version: CacheVersion,
		Root:    root,
		Files:   map[string]*FileEntry{},
		Edges:   map[string][]string{},
	}
}

// DefaultCacheDir devolve <root>/.agent-sync/cache por padrão, ou o
// diretório absoluto informado.
func DefaultCacheDir(root string) string {
	if root == "" {
		root, _ = os.Getwd()
	}
	return filepath.Join(root, ".agent-sync", "cache")
}

// Load lê o cache de cacheDir/repomap.json. Retorna cache vazio se o
// arquivo não existe; erro apenas em falhas reais de I/O ou JSON.
func Load(cacheDir string) (*Cache, error) {
	path := filepath.Join(cacheDir, CacheFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("ler cache: %w", err)
	}
	var c Cache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("decodificar cache: %w", err)
	}
	if c.Files == nil {
		c.Files = map[string]*FileEntry{}
	}
	if c.Edges == nil {
		c.Edges = map[string][]string{}
	}
	if c.Version == 0 {
		c.Version = CacheVersion
	}
	return &c, nil
}

// Save persiste o cache atomicamente em cacheDir/repomap.json.
func Save(cacheDir string, c *Cache) error {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("criar dir cache: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("serializar cache: %w", err)
	}
	final := filepath.Join(cacheDir, CacheFileName)
	tmp, err := os.CreateTemp(cacheDir, ".repomap-*.json.tmp")
	if err != nil {
		return fmt.Errorf("criar tmp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("escrever tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("fechar tmp: %w", err)
	}
	if err := os.Rename(tmpName, final); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// lsFiles devolve a lista de arquivos rastreados do repositório. Usa
// `git ls-files` se houver .git dentro de root; caso contrário, faz um
// traversal manual respeitando .gitignore e ignorando diretórios comuns.
func lsFiles(root string) ([]string, error) {
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	gitDir := filepath.Join(root, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		out, err := exec.Command("git", "-C", root, "ls-files", "-co", "--exclude-standard").Output()
		if err == nil {
			lines := splitLines(string(out))
			nonEmpty := lines[:0]
			for _, l := range lines {
				if l != "" {
					nonEmpty = append(nonEmpty, l)
				}
			}
			return nonEmpty, nil
		}
	}
	return walkFiles(root)
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func walkFiles(root string) ([]string, error) {
	var paths []string
	gi, _ := loadGitignore(root)
	skip := map[string]bool{
		".git": true, ".agent-sync": true, "node_modules": true,
		"vendor": true, ".venv": true, "__pycache__": true,
	}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrPermission) {
				return nil
			}
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if skip[name] || (gi != nil && gi.match(path, true)) {
				return filepath.SkipDir
			}
			return nil
		}
		if gi != nil && gi.match(path, false) {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

// gitignoreParser faz matching minimalista (suficiente para .gitignore
// gerado por `go mod`, `npm`, etc.).
type gitignoreParser struct {
	patterns []gitignoreRule
}

type gitignoreRule struct {
	pattern  string
	dirOnly  bool
	negate   bool
	anchored bool
}

func loadGitignore(root string) (*gitignoreParser, error) {
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		return nil, err
	}
	p := &gitignoreParser{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		negate := false
		if strings.HasPrefix(line, "!") {
			negate = true
			line = strings.TrimSpace(line[1:])
		}
		dirOnly := strings.HasSuffix(line, "/")
		line = strings.TrimSuffix(line, "/")
		anchored := strings.HasPrefix(line, "/")
		line = strings.TrimPrefix(line, "/")
		if line == "" {
			continue
		}
		p.patterns = append(p.patterns, gitignoreRule{
			pattern:  line,
			dirOnly:  dirOnly,
			negate:   negate,
			anchored: anchored,
		})
	}
	return p, scanner.Err()
}

func (p *gitignoreParser) match(path string, isDir bool) bool {
	if p == nil {
		return false
	}
	clean := strings.TrimPrefix(path, "./")
	matched := false
	for _, rule := range p.patterns {
		if rule.dirOnly && !isDir {
			continue
		}
		ok := globMatch(rule.pattern, clean, rule.anchored)
		if ok {
			matched = !rule.negate
		}
	}
	return matched
}

func globMatch(pattern, name string, anchored bool) bool {
	if anchored {
		return matchSegment(pattern, name)
	}
	idx := 0
	for idx <= len(name) {
		if matchSegment(pattern, name[idx:]) {
			return true
		}
		slash := strings.Index(name[idx:], "/")
		if slash < 0 {
			return false
		}
		idx += slash + 1
	}
	return false
}

func matchSegment(pattern, segment string) bool {
	pi, si := 0, 0
	starIdx := -1
	matchIdx := 0
	for si < len(segment) {
		if pi < len(pattern) && (pattern[pi] == '?' || pattern[pi] == segment[si]) {
			pi++
			si++
			continue
		}
		if pi < len(pattern) && pattern[pi] == '*' {
			starIdx = pi
			matchIdx = si
			pi++
			continue
		}
		if starIdx >= 0 {
			pi = starIdx + 1
			matchIdx++
			si = matchIdx
			continue
		}
		return false
	}
	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}
	return pi == len(pattern)
}

// Update varre o repositório, re-parseia apenas arquivos modificados e
// reescreve o cache. Se cacheDir não contém um cache prévio, um novo é
// criado. Retorna estatísticas úteis para telemetria.
func Update(root, cacheDir string) (*Cache, UpdateStats, error) {
	start := nowUnixNano()
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return nil, UpdateStats{}, err
		}
	}
	cache, err := Load(cacheDir)
	if err != nil {
		return nil, UpdateStats{}, err
	}
	if cache == nil {
		cache = NewCache(root)
	} else if cache.Root == "" {
		cache.Root = root
	}

	files, err := lsFiles(root)
	if err != nil {
		return nil, UpdateStats{}, fmt.Errorf("ls-files: %w", err)
	}

	current := map[string]struct{}{}
	stats := UpdateStats{Visited: len(files)}

	for _, rel := range files {
		abs := filepath.Join(root, rel)
		info, err := os.Stat(abs)
		if err != nil {
			continue
		}
		current[rel] = struct{}{}
		entry, exists := cache.Files[rel]
		if exists && entry.MTimeNs == info.ModTime().UnixNano() && entry.Size == info.Size() {
			stats.Reused++
			continue
		}
		newEntry, parseErr := parseFile(rel, abs, info)
		if parseErr != nil {
			if entry != nil {
				entry.MTimeNs = info.ModTime().UnixNano()
				entry.Size = info.Size()
			}
			continue
		}
		cache.Files[rel] = newEntry
		stats.Reparsed++
	}

	for rel := range cache.Files {
		if _, ok := current[rel]; !ok {
			delete(cache.Files, rel)
			stats.Removed++
		}
	}

	cache.Edges = rebuildEdges(cache.Files)
	stats.EdgesNow = countEdges(cache.Edges)
	stats.ElapsedMs = (nowUnixNano() - start) / int64(1e6)

	if err := Save(cacheDir, cache); err != nil {
		return nil, stats, err
	}
	return cache, stats, nil
}

func nowUnixNano() int64 { return time.Now().UnixNano() }

func parseFile(rel, abs string, info os.FileInfo) (*FileEntry, error) {
	f, err := os.Open(abs)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var buf strings.Builder
	tee := io.TeeReader(f, &buf)
	limited := io.LimitReader(tee, defaultMaxBytes)
	data, _ := io.ReadAll(limited)
	if int64(len(data)) == info.Size() {
		buf.Reset()
		buf.Write(data)
	}
	src := []byte(buf.String())

	entry := &FileEntry{
		MTimeNs: info.ModTime().UnixNano(),
		Size:    info.Size(),
	}
	entry.Hash = hashHead(src, 4096)
	entry.Symbols, entry.Imports, entry.Calls = extractStructured(rel, src)
	return entry, nil
}

func hashHead(src []byte, n int) string {
	if len(src) > n {
		src = src[:n]
	}
	sum := sha1.Sum(src)
	return hex.EncodeToString(sum[:8])
}

// extractStructured devolve (symbols, imports, calls) parseados a partir do
// conteúdo, escolhendo o extrator conforme a extensão.
func extractStructured(rel string, src []byte) (symbols, imports, calls []string) {
	ext := strings.ToLower(filepath.Ext(rel))
	switch ext {
	case ".go":
		symbols, imports = extractGo(src)
	case ".py":
		symbols, imports = extractPy(src)
	case ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx":
		symbols, imports = extractTS(src)
	case ".php":
		symbols, imports = extractPHP(src)
	default:
		return nil, nil, nil
	}
	calls = extractCalls(src)
	symbols = dedup(symbols)
	imports = dedup(imports)
	calls = dedup(calls)
	return
}

func extractGo(src []byte) (symbols, imports []string) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return nil, nil
	}
	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				for _, spec := range d.Specs {
					if is, ok := spec.(*ast.ImportSpec); ok {
						if is.Path != nil {
							imports = append(imports, strings.Trim(is.Path.Value, `"`))
						}
					}
				}
				continue
			}
			for _, spec := range d.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok {
					symbols = append(symbols, ts.Name.Name)
				}
			}
		case *ast.FuncDecl:
			name := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) > 0 {
				t := d.Recv.List[0].Type
				if star, ok := t.(*ast.StarExpr); ok {
					if id, ok := star.X.(*ast.Ident); ok {
						name = id.Name + "." + name
					}
				} else if id, ok := t.(*ast.Ident); ok {
					name = id.Name + "." + name
				}
			}
			symbols = append(symbols, name)
		}
	}
	return
}

var (
	rePyClass  = regexp.MustCompile(`(?m)^(?:@\w+(?:\([^)]*\))?\s*)*class\s+(\w+)`)
	rePyFunc   = regexp.MustCompile(`(?m)^(?:async\s+)?def\s+(\w+)`)
	rePyImport = regexp.MustCompile(`(?m)^\s*(?:from\s+([\w.]+)\s+)?import\s+([\w.,\s*]+(?:as\s+\w+)?)`)
)

func extractPy(src []byte) (symbols, imports []string) {
	for _, m := range rePyClass.FindAllStringSubmatch(string(src), -1) {
		symbols = append(symbols, m[1])
	}
	for _, m := range rePyFunc.FindAllStringSubmatch(string(src), -1) {
		symbols = append(symbols, m[1])
	}
	for _, m := range rePyImport.FindAllStringSubmatch(string(src), -1) {
		if m[1] != "" {
			imports = append(imports, m[1])
		} else if m[2] != "" {
			for _, p := range strings.Split(m[2], ",") {
				p = strings.TrimSpace(p)
				p = strings.SplitN(p, " as ", 2)[0]
				p = strings.SplitN(p, "*", 2)[0]
				if p != "" {
					imports = append(imports, p)
				}
			}
		}
	}
	return
}

var (
	reTSClass         = regexp.MustCompile(`(?m)^(?:export\s+(?:default\s+)?)?(?:abstract\s+)?class\s+(\w+)`)
	reTSInterface     = regexp.MustCompile(`(?m)^(?:export\s+)?interface\s+(\w+)`)
	reTSType          = regexp.MustCompile(`(?m)^(?:export\s+)?type\s+(\w+)`)
	reTSFunc          = regexp.MustCompile(`(?m)^(?:export\s+(?:default\s+)?)?(?:async\s+)?function\s+(\w+)`)
	reTSArrow         = regexp.MustCompile(`(?m)^(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s*)?\(`)
	reTSRequireAssign = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*require\(`)
	reTSImportFrom    = regexp.MustCompile(`(?m)^import\s+(?:type\s+)?(?:\{[^}]+\}|\*\s+as\s+\w+|\w+)\s+from\s+["']([^"']+)["']`)
	reTSImportSide    = regexp.MustCompile(`(?m)^import\s+["']([^"']+)["']`)
	reTSRequire       = regexp.MustCompile(`(?m)require\(\s*["']([^"']+)["']\s*\)`)
)

func extractTS(src []byte) (symbols, imports []string) {
	s := string(src)
	for _, re := range []*regexp.Regexp{reTSClass, reTSInterface, reTSType, reTSFunc, reTSArrow, reTSRequireAssign} {
		for _, m := range re.FindAllStringSubmatch(s, -1) {
			symbols = append(symbols, m[1])
		}
	}
	for _, re := range []*regexp.Regexp{reTSImportFrom, reTSImportSide, reTSRequire} {
		for _, m := range re.FindAllStringSubmatch(s, -1) {
			imports = append(imports, m[1])
		}
	}
	return
}

var (
	rePHPClass = regexp.MustCompile(`(?m)^(?:(?:final|abstract|readonly)\s+)?(?:class|interface|trait|enum)\s+(\w+)`)
	rePHPFunc  = regexp.MustCompile(`(?m)^\s*(?:public|protected|private)?\s*(?:static\s+)?function\s+(\w+)`)
	rePHPUse   = regexp.MustCompile(`(?m)^\s*use\s+([\w\\]+(?:\s+as\s+\w+)?)\s*;`)
)

func extractPHP(src []byte) (symbols, imports []string) {
	s := string(src)
	for _, re := range []*regexp.Regexp{rePHPClass, rePHPFunc} {
		for _, m := range re.FindAllStringSubmatch(s, -1) {
			symbols = append(symbols, m[1])
		}
	}
	for _, m := range rePHPUse.FindAllStringSubmatch(s, -1) {
		parts := strings.Fields(m[1])
		if len(parts) > 0 {
			imports = append(imports, strings.ReplaceAll(parts[0], `\`, "/"))
		}
	}
	return
}

// reCall captura invocações simples: identificador seguido de "(".
var reCall = regexp.MustCompile(`\b([A-Za-z_][\w.]*)\s*\(`)

func extractCalls(src []byte) []string {
	matches := reCall.FindAllStringSubmatch(string(src), -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		id := m[1]
		if _, ok := seen[id]; ok {
			continue
		}
		if isKeyword(id) {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
		if len(out) >= 256 {
			break
		}
	}
	return out
}

func isKeyword(s string) bool {
	switch s {
	case "if", "for", "while", "switch", "return", "func", "def", "class",
		"new", "catch", "throw", "try", "else", "elif", "case", "do",
		"await", "yield", "import", "from", "package":
		return true
	}
	return isBuiltinOrTestNoise(s)
}

func isBuiltinOrTestNoise(s string) bool {
	// Chamadas de teste (ex: t.Fatalf, t.Run, b.ResetTimer, etc.)
	if strings.HasPrefix(s, "t.") || strings.HasPrefix(s, "b.") || strings.HasPrefix(s, "m.") {
		return true
	}
	switch s {
	// Primitivos e built-ins do Go
	case "len", "cap", "append", "make", "new", "delete", "copy", "close", "panic", "recover",
		"byte", "string", "int", "int64", "int32", "uint", "uint64", "float64", "bool", "error":
		return true
	// Primitivos e built-ins de Python / JS / PHP
	case "print", "range", "enumerate", "isinstance", "getattr", "setattr", "hasattr",
		"console.log", "console.error", "console.warn", "require", "typeof", "instanceof",
		"echo", "isset", "empty", "var_dump", "count", "array", "die":
		return true
	}
	return false
}

func dedup(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// rebuildEdges reconstrói o índice de arestas: chave = "<path>::<symbol>"
// ou "<path>::import::<path>" para arestas de import. Para manter o grafo
// raso, não fazemos resolução semântica completa de imports; apenas
// catalogamos quem declara cada símbolo e quem parece importar de um path.
func rebuildEdges(files map[string]*FileEntry) map[string][]string {
	edges := map[string][]string{}
	for path, e := range files {
		for _, sym := range e.Symbols {
			from := path + "::" + sym
			for _, c := range e.Calls {
				if c == sym {
					continue
				}
				edges[from] = appendUnique(edges[from], c)
			}
		}
		for _, imp := range e.Imports {
			from := path + "::import"
			edges[from] = appendUnique(edges[from], imp)
		}
	}
	return edges
}

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

func countEdges(edges map[string][]string) int {
	n := 0
	for _, v := range edges {
		n += len(v)
	}
	return n
}

// Focus renderiza um subgrafo textual em torno de targetPath.
// Retorna sempre algo legível, mesmo quando o cache está vazio.
func Focus(cache *Cache, targetPath string, depth int) string {
	if cache == nil || len(cache.Files) == 0 {
		return "(cache vazio — execute repo-map --update primeiro)"
	}
	if depth <= 0 {
		depth = 1
	}
	targetPath = normalizePath(targetPath)
	entry := cache.Files[targetPath]
	if entry == nil {
		// tenta match por sufixo (ex.: "main.go" informado, mas cache tem "cmd/main.go")
		for k := range cache.Files {
			if strings.HasSuffix(k, "/"+targetPath) || k == targetPath {
				entry = cache.Files[k]
				targetPath = k
				break
			}
		}
	}
	if entry == nil {
		return fmt.Sprintf("(arquivo %q não encontrado no cache)", targetPath)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "📍 Foco: %s\n", targetPath)
	fmt.Fprintf(&b, "   Symbols: %s\n", strings.Join(entry.Symbols, ", "))
	if len(entry.Imports) > 0 {
		fmt.Fprintf(&b, "   Imports: %s\n", strings.Join(entry.Imports, ", "))
	}
	if len(entry.Calls) > 0 {
		fmt.Fprintf(&b, "   Calls:   %s\n", strings.Join(entry.Calls, ", "))
	}

	// quem importa este arquivo?
	importers := reverseImporters(cache, targetPath)
	if len(importers) > 0 {
		fmt.Fprintf(&b, "\n↓ Importers (%d):\n", len(importers))
		for _, p := range importers {
			fmt.Fprintf(&b, "   - %s\n", p)
		}
	}

	if depth >= 2 {
		// segunda camada: arquivos que importam os importers
		seen := map[string]bool{targetPath: true}
		for _, imp := range importers {
			seen[imp] = true
		}
		var second []string
		for _, imp := range importers {
			for _, p := range reverseImporters(cache, imp) {
				if !seen[p] {
					seen[p] = true
					second = append(second, p)
				}
			}
		}
		if len(second) > 0 {
			fmt.Fprintf(&b, "\n↓ Importers (depth=2, %d):\n", len(second))
			for _, p := range second {
				fmt.Fprintf(&b, "   - %s\n", p)
			}
		}
	}
	return b.String()
}

func reverseImporters(cache *Cache, target string) []string {
	var out []string
	seen := map[string]bool{}
	for path, e := range cache.Files {
		for _, imp := range e.Imports {
			if importMatches(imp, target) {
				if !seen[path] {
					seen[path] = true
					out = append(out, path)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

// importMatches é um matcher tolerante: compara sufixo do path (sem .go)
// e nome curto do pacote. Ex.: "github.com/x/y/internal/sync" casa com
// "internal/sync/sync.go".
func importMatches(imp, targetPath string) bool {
	imp = strings.TrimSpace(imp)
	if imp == "" {
		return false
	}
	if strings.HasSuffix(imp, "/"+targetPath) {
		return true
	}
	if strings.Contains(imp, targetPath) {
		return true
	}
	base := strings.TrimSuffix(filepath.Base(targetPath), filepath.Ext(targetPath))
	if base != "" && (imp == base || strings.HasSuffix(imp, "/"+base)) {
		return true
	}
	return false
}

// Summary devolve os N símbolos mais centrais do grafo, ordenados por grau
// (total de ocorrências em Calls + arestas de import).
func Summary(cache *Cache, maxTokens int) string {
	if cache == nil || len(cache.Files) == 0 {
		return "(cache vazio — execute repo-map --update primeiro)"
	}
	type hub struct {
		symbol string
		files  int
		score  int
	}
	score := map[string]*hub{}
	// Coleta todos os símbolos declarados no próprio repositório
	declared := map[string]struct{}{}
	for _, e := range cache.Files {
		for _, s := range e.Symbols {
			declared[s] = struct{}{}
		}
	}

	for _, e := range cache.Files {
		for _, c := range e.Calls {
			// Símbolo com ponto (ex: filepath.Join, os.WriteFile, strings.Contains) que não foi declarado internamente
			if strings.Contains(c, ".") {
				pkg := strings.Split(c, ".")[0]
				// Ignora stdlib e chamadas externas se a struct/pacote não foi declarada localmente
				if _, isDeclared := declared[pkg]; !isDeclared {
					continue
				}
			} else {
				// Símbolo simples que não foi declarado no repositório (ex: main ou identificador solto)
				if _, isDeclared := declared[c]; !isDeclared {
					continue
				}
			}

			h, ok := score[c]
			if !ok {
				h = &hub{symbol: c}
				score[c] = h
			}
			h.score++
		}
	}
	filesWithSym := map[string]map[string]struct{}{}
	for path, e := range cache.Files {
		for _, s := range append([]string{}, append(e.Symbols, e.Calls...)...) {
			if filesWithSym[s] == nil {
				filesWithSym[s] = map[string]struct{}{}
			}
			filesWithSym[s][path] = struct{}{}
		}
	}
	for k, h := range score {
		h.files = len(filesWithSym[k])
	}

	hubs := make([]*hub, 0, len(score))
	for _, h := range score {
		hubs = append(hubs, h)
	}
	sort.Slice(hubs, func(i, j int) bool {
		if hubs[i].score != hubs[j].score {
			return hubs[i].score > hubs[j].score
		}
		return hubs[i].symbol < hubs[j].symbol
	})

	var b strings.Builder
	fmt.Fprintf(&b, "🧭 Top hubs (%d símbolos, %d arquivos):\n", len(hubs), len(cache.Files))
	approxTokens := 0
	for _, h := range hubs {
		line := fmt.Sprintf("   %s → score=%d files=%d\n", h.symbol, h.score, h.files)
		if maxTokens > 0 {
			t := len(strings.Fields(line))
			if approxTokens+t > maxTokens {
				break
			}
			approxTokens += t
		}
		b.WriteString(line)
		if maxTokens > 0 && approxTokens >= maxTokens {
			break
		}
	}
	return b.String()
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	p = filepath.Clean(p)
	return strings.TrimPrefix(p, "./")
}

// Itoa é um helper exposto para formatar elapsed em mensagens CLI.
func Itoa(n int64) string { return strconv.FormatInt(n, 10) }
