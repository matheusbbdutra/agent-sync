package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SkeletonResult é a saída de Skeletonize: arquivo original, esqueleto
// estrutural e taxa de redução (% chars economizados).
type SkeletonResult struct {
	Path          string `json:"path"`
	Language      string `json:"language"`
	Lines         int    `json:"lines"`
	Bytes         int    `json:"bytes"`
	SkeletonChars int    `json:"skeleton_chars"`
	ReductionPct  int    `json:"reduction_pct"` // 0-100; 100 = só headers
	FromCache     bool   `json:"from_cache"`
	Fingerprint   string `json:"fingerprint"`
	Skeleton      string `json:"skeleton"`
}

const (
	skeletonMinBytes     = 8 * 1024 // abaixo disso, devolve conteúdo inteiro
	skeletonCacheVersion = 1        // invalidar cache se formato mudar
)

// Skeletonize devolve um esqueleto estrutural de `path`: signatures +
// imports + headers de classe/função. Sem diff/delta — só first-read
// skeleton, alinhado ao `structure_map.py` do token-optimizer externo.
// Cache por fingerprint (mtime+size+hash[:8]) em ~/.cache/agent-sync/skeletons/.
//
// Fallback gracioso: se o arquivo for < skeletonMinBytes OU a linguagem
// não for suportada, devolve o conteúdo inteiro wrapped em SkeletonResult
// com ReductionPct=0.
func Skeletonize(path string, force bool) (*SkeletonResult, error) {
	cleanPath := filepath.Clean(path)
	info, err := os.Stat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("ctx-window: stat %s: %w", cleanPath, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("ctx-window: %s is a directory", cleanPath)
	}
	lang := detectLanguage(cleanPath)
	body, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("ctx-window: read %s: %w", cleanPath, err)
	}
	fp := fileFingerprint(cleanPath, info, body)
	res := &SkeletonResult{
		Path:        cleanPath,
		Language:    lang,
		Lines:       bytesLines(body),
		Bytes:       len(body),
		Fingerprint: fp,
	}

	// cache lookup (a menos que --force)
	if !force {
		if cached, ok := readCachedSkeleton(fp); ok {
			res.Skeleton = cached
			res.FromCache = true
			res.SkeletonChars = len(cached)
			res.ReductionPct = reductionPct(len(body), len(cached))
			return res, nil
		}
	}

	// abaixo do mínimo → devolve conteúdo inteiro
	if len(body) < skeletonMinBytes {
		res.Skeleton = string(body)
		res.SkeletonChars = len(body)
		res.ReductionPct = 0
		_ = writeCachedSkeleton(fp, res.Skeleton) // cache para hit futuro
		return res, nil
	}

	// linguagem não suportada → devolve conteúdo inteiro
	skel := runSkeletonizer(lang, string(body))
	if skel == "" {
		res.Skeleton = string(body)
		res.SkeletonChars = len(body)
		res.ReductionPct = 0
		_ = writeCachedSkeleton(fp, res.Skeleton)
		return res, nil
	}

	res.Skeleton = skel
	res.SkeletonChars = len(skel)
	res.ReductionPct = reductionPct(len(body), len(skel))
	_ = writeCachedSkeleton(fp, skel) // best-effort
	return res, nil
}

// fileFingerprint combina mtime + size + sha256[:8] para cache estável.
// mtime_ns é suficiente para invalidação na maioria dos casos; o sha256
// cobre colisões raras (mesmo size+mtime em arquivos distintos).
func fileFingerprint(path string, info os.FileInfo, body []byte) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%d\x00%d\x00", path, info.Size(), info.ModTime().UnixNano())
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil)[:8])
}

// detectLanguage mapeia extensão → chave de skeletonizer.
// Suportadas: py, ts, js, go. Outras: "" (fallback devolve conteúdo inteiro).
func detectLanguage(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".py", ".pyi":
		return "py"
	case ".ts", ".tsx":
		return "ts"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "js"
	case ".go":
		return "go"
	default:
		return ""
	}
}

// runSkeletonizer despacha para o skeletonizer específico da linguagem.
// "" indica "não suportada" e o caller faz fallback.
func runSkeletonizer(lang, body string) string {
	lines := strings.Split(body, "\n")
	switch lang {
	case "py":
		return skeletonPython(lines)
	case "ts", "js":
		return skeletonTSJS(lines, lang)
	case "go":
		return skeletonGo(lines)
	default:
		return ""
	}
}

// skeletonPython extrai: imports, defs (com número de linha), classes, decorators.
func skeletonPython(lines []string) string {
	var out []string
	out = append(out, "# python skeleton (definitions + imports)")
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "from "):
			out = append(out, fmt.Sprintf("L%d: %s", i+1, line))
		case strings.HasPrefix(line, "def ") || strings.HasPrefix(line, "async def "):
			out = append(out, fmt.Sprintf("L%d: %s", i+1, line))
		case strings.HasPrefix(line, "class "):
			out = append(out, fmt.Sprintf("L%d: %s", i+1, line))
		case strings.HasPrefix(line, "@"):
			out = append(out, fmt.Sprintf("L%d: %s", i+1, line))
		case isConstDecl(line):
			out = append(out, fmt.Sprintf("L%d: %s", i+1, line))
		}
	}
	return strings.Join(out, "\n")
}

// skeletonTSJS extrai: imports, exports (function/class/const/interface/type).
func skeletonTSJS(lines []string, lang string) string {
	label := "ts"
	if lang == "js" {
		label = "js"
	}
	var out []string
	out = append(out, fmt.Sprintf("# %s skeleton (imports + exports + signatures)", label))
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "import "):
			out = append(out, fmt.Sprintf("L%d: %s", i+1, line))
		case strings.HasPrefix(line, "export "):
			out = append(out, fmt.Sprintf("L%d: %s", i+1, line))
		case strings.HasPrefix(line, "function "):
			out = append(out, fmt.Sprintf("L%d: %s", i+1, line))
		case strings.HasPrefix(line, "class "):
			out = append(out, fmt.Sprintf("L%d: %s", i+1, line))
		case strings.HasPrefix(line, "interface ") || strings.HasPrefix(line, "type "):
			out = append(out, fmt.Sprintf("L%d: %s", i+1, line))
		case strings.HasPrefix(line, "const ") && strings.Contains(line, "=") && strings.Contains(line, "("):
			out = append(out, fmt.Sprintf("L%d: %s", i+1, line))
		}
	}
	return strings.Join(out, "\n")
}

// skeletonGo extrai: package, imports, func, type, var/const top-level.
func skeletonGo(lines []string) string {
	var out []string
	out = append(out, "# go skeleton (package + imports + signatures)")
	inBlock := 0
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		// sair de bloco: }, ), ] no início da linha (heurística leve)
		if strings.HasPrefix(line, "}") || strings.HasPrefix(line, ")") {
			inBlock = max(inBlock-1, 0)
			continue
		}
		if inBlock > 0 {
			// contar abertura de bloco dentro
			inBlock += strings.Count(line, "{") - strings.Count(line, "}")
			if inBlock < 0 {
				inBlock = 0
			}
			continue
		}
		switch {
		case strings.HasPrefix(line, "package "):
			out = append(out, fmt.Sprintf("L%d: %s", i+1, line))
		case strings.HasPrefix(line, "import "):
			out = append(out, fmt.Sprintf("L%d: %s", i+1, line))
		case strings.HasPrefix(line, "func "):
			out = append(out, fmt.Sprintf("L%d: %s", i+1, line))
		case strings.HasPrefix(line, "type ") && (strings.Contains(line, " struct") || strings.Contains(line, " interface")):
			out = append(out, fmt.Sprintf("L%d: %s", i+1, line))
		case strings.HasPrefix(line, "var ") || strings.HasPrefix(line, "const "):
			out = append(out, fmt.Sprintf("L%d: %s", i+1, line))
		}
		inBlock += strings.Count(line, "{") - strings.Count(line, "}")
	}
	return strings.Join(out, "\n")
}

func isConstDecl(line string) bool {
	// UPPER_CASE_NAME = ...  (heurística leve para constantes Python)
	if !strings.Contains(line, "=") {
		return false
	}
	head := strings.SplitN(line, "=", 2)[0]
	head = strings.TrimSpace(head)
	parts := strings.Fields(head)
	if len(parts) == 0 {
		return false
	}
	name := parts[len(parts)-1]
	if len(name) < 2 {
		return false
	}
	for _, r := range name {
		if r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			continue
		}
		return false
	}
	// pelo menos uma letra maiúscula (não é só número)
	hasUpper := false
	for _, r := range name {
		if r >= 'A' && r <= 'Z' {
			hasUpper = true
			break
		}
	}
	return hasUpper
}

func bytesLines(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	return strings.Count(string(b), "\n") + 1
}

func reductionPct(orig, reduced int) int {
	if orig <= 0 {
		return 0
	}
	pct := (orig - reduced) * 100 / orig
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
}

func skeletonCacheRoot() (string, error) {
	root := cacheRoot
	if root == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			home, herr := os.UserHomeDir()
			if herr != nil {
				return "", fmt.Errorf("ctx-window: no cache/home directory available: %w", err)
			}
			cache = filepath.Join(home, ".cache")
		}
		root = filepath.Join(cache, "agent-sync")
	}
	dir := filepath.Join(root, "skeletons")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("ctx-window: mkdir skeletons dir: %w", err)
	}
	return dir, nil
}

func readCachedSkeleton(fp string) (string, bool) {
	dir, err := skeletonCacheRoot()
	if err != nil {
		return "", false
	}
	path := filepath.Join(dir, fmt.Sprintf("v%d_%s.txt", skeletonCacheVersion, fp))
	body, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(body), true
}

func writeCachedSkeleton(fp, content string) error {
	dir, err := skeletonCacheRoot()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, fmt.Sprintf("v%d_%s.txt", skeletonCacheVersion, fp))
	return os.WriteFile(path, []byte(content), 0o600)
}

// Write imprime o SkeletonResult em formato humano (default) ou JSON.
func (r *SkeletonResult) Write(w io.Writer, asJSON bool) error {
	if asJSON {
		return writeJSON(w, r)
	}
	fmt.Fprintf(w, "skeleton of %s\n", r.Path)
	fmt.Fprintf(w, "  language:    %s\n", r.Language)
	fmt.Fprintf(w, "  lines:       %d\n", r.Lines)
	fmt.Fprintf(w, "  bytes:       %d\n", r.Bytes)
	fmt.Fprintf(w, "  skeleton:    %d chars\n", r.SkeletonChars)
	fmt.Fprintf(w, "  reduction:   %d%%\n", r.ReductionPct)
	fmt.Fprintf(w, "  from cache:  %v\n", r.FromCache)
	fmt.Fprintf(w, "  fingerprint: %s\n", r.Fingerprint)
	if !r.FromCache {
		fmt.Fprintf(w, "  cached at:   %s\n", time.Now().UTC().Format(time.RFC3339))
	}
	fmt.Fprintln(w, "\n--- skeleton ---")
	fmt.Fprintln(w, r.Skeleton)
	return nil
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
