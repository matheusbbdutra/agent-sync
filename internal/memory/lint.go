// Package memory: lint subcommand (A-61).
//
// `agent-sync memory lint` faz 3 checks sobre ~/.cache/agent-sync/memory_pages.jsonl:
//   - frontmatter invalido (YAML malformado, scope fora de project|global, expires_at mal)
//   - dangling refs (page A referencia path: de page B que nao existe)
//   - orphans (page no JSONL que nenhum path: referencia)
//
// Cross-module para tools/internal/agentmemory NAO eh usado (A-59 fixou que JSONL
// append-only eh fonte canonica; Store SQLite eh secundario). Lint opera
// puramente no JSONL para zero deps externas.
//
// Origem: ai-memory memory_lint (D-72). Zero-LLM (D-6).
package memory

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// LintIssue categoriza um problema encontrado. Type alinhado com o que a UI/JSON
// pode diferenciar (filtragem, exit code, etc).
type LintIssue struct {
	Page   string `json:"page"`   // path da page afetada (vazia para orfão global)
	Type   string `json:"type"`   // "frontmatter_invalid" | "dangling_ref" | "orphan"
	Detail string `json:"detail"` // mensagem legivel
}

// LintResult agrega contagem + issues. JSON estavel para tooling.
type LintResult struct {
	Pages  int         `json:"pages"`
	Issues []LintIssue `json:"issues"`
}

var (
	// fmKeyRegex casa `chave: valor` no frontmatter YAML simples.
	fmKeyRegex = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_-]*):\s*(.*?)\s*$`)
	// refRegex casa `path: foo/bar` em qualquer posicao (fronteira de palavra).
	// Pragmatic: casa qualquer coisa que pareca um path com slash.
	refRegex = regexp.MustCompile("(?:^|\\s)`path:\\s*([a-zA-Z0-9_\\-./]+)`")
)

// extractFrontmatter faz parse simples do frontmatter YAML entre --- ... ---.
// Retorna map chave→valor + ok. NAO eh parser YAML completo: so entende
//  `chave: valor` por linha. Compromise com A-56/A-61 (ver command.go:278-281).
func extractFrontmatter(body string) (map[string]string, bool) {
	if !strings.HasPrefix(body, "---\n") {
		return nil, false
	}
	rest := body[4:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, false
	}
	fm := map[string]string{}
	for _, line := range strings.Split(rest[:end], "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := fmKeyRegex.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		fm[m[1]] = m[2]
	}
	return fm, true
}

// isValidExpiresAt aceita RFC3339 ("2026-12-31T23:59:59Z") ou date-only
// ("2026-12-31"). Mesmo padrao de validateExpiresAt em command.go:294.
func isValidExpiresAt(s string) bool {
	if _, err := time.Parse(time.RFC3339, s); err == nil {
		return true
	}
	if _, err := time.Parse("2006-01-02", s); err == nil {
		return true
	}
	return false
}

// pageHasReferences retorna os paths que uma page referencia via `path: X`
// no frontmatter ou body. Vazio se nenhum.
func pageReferences(body, fmPath string) []string {
	refs := []string{}
	if fmPath != "" {
		refs = append(refs, fmPath)
	}
	for _, m := range refRegex.FindAllStringSubmatch(body, -1) {
		if len(m) > 1 && m[1] != "" {
			refs = append(refs, m[1])
		}
	}
	return refs
}

// lintOnePage roda os checks estruturais (frontmatter, refs) em uma page.
// Orfao eh calculado depois (cross-page).
func lintOnePage(path, fmPath, body string) []LintIssue {
	var issues []LintIssue
	fm, ok := extractFrontmatter(body)
	if ok {
		if scope, hasScope := fm["scope"]; hasScope && scope != "project" && scope != "global" {
			issues = append(issues, LintIssue{
				Page:   path,
				Type:   "frontmatter_invalid",
				Detail: fmt.Sprintf("scope=%q deve ser project|global", scope),
			})
		}
		if exp, hasExp := fm["expires_at"]; hasExp && !isValidExpiresAt(exp) {
			issues = append(issues, LintIssue{
				Page:   path,
				Type:   "frontmatter_invalid",
				Detail: fmt.Sprintf("expires_at=%q deve ser RFC3339 ou YYYY-MM-DD", exp),
			})
		}
		if p, hasPath := fm["path"]; hasPath && p == "" {
			issues = append(issues, LintIssue{
				Page:   path,
				Type:   "frontmatter_invalid",
				Detail: "frontmatter path: vazio",
			})
		}
	}
	// dangling: refs que apontam para path que nao existe no map de pages
	// eh computado em RunLint (precisa do map global).
	return issues
}

// RunLint executa os 3 checks no JSONL memory_pages. Retorna LintResult
// (sempre, mesmo se exit 1). Erro de I/O ou JSON malformado eh retornado
// separadamente.
func RunLint(jsonlPath string, out io.Writer, asJSON bool) (LintResult, error) {
	f, err := os.Open(jsonlPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Sem pages ainda: vazio eh valido.
			res := LintResult{Pages: 0, Issues: []LintIssue{}}
			if asJSON {
				_ = writeLintJSON(out, res)
			} else {
				fmt.Fprintln(out, "memory lint: 0 pages (nenhuma ainda) - OK")
			}
			return res, nil
		}
		return LintResult{}, fmt.Errorf("abrir %s: %w", jsonlPath, err)
	}
	defer f.Close()

	// 1) Parse JSONL, indexar pages por path.
	pages := map[string]string{} // path -> body
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var malformed int
	for sc.Scan() {
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		var e pageEntry
		if err := json.Unmarshal([]byte(text), &e); err != nil {
			malformed++
			continue
		}
		if e.Path == "" {
			continue
		}
		pages[e.Path] = e.Body
	}
	if err := sc.Err(); err != nil {
		return LintResult{}, fmt.Errorf("scan %s: %w", jsonlPath, err)
	}

	// 2) Roda checks per-page + dangling refs.
	allRefs := map[string][]string{} // path -> lista de refs
	var issues []LintIssue
	for path, body := range pages {
		fm, _ := extractFrontmatter(body)
		fmPath := ""
		if fm != nil {
			fmPath = fm["path"]
		}
		issues = append(issues, lintOnePage(path, fmPath, body)...)
		allRefs[path] = pageReferences(body, fmPath)
	}
	// dangling: cada ref deve apontar para um path conhecido
	for _, path := range sortedKeys(pages) {
		for _, ref := range allRefs[path] {
			if _, ok := pages[ref]; !ok {
				issues = append(issues, LintIssue{
					Page:   path,
					Type:   "dangling_ref",
					Detail: fmt.Sprintf("path: %q nao existe no JSONL", ref),
				})
			}
		}
	}
	// orphans: pages que nenhum path: referencia (caminho inverso)
	referenced := map[string]bool{}
	for _, refs := range allRefs {
		for _, r := range refs {
			referenced[r] = true
		}
	}
	for _, path := range sortedKeys(pages) {
		if !referenced[path] {
			issues = append(issues, LintIssue{
				Page:   path,
				Type:   "orphan",
				Detail: "nenhuma outra page referencia via path:",
			})
		}
	}
	// hint sobre JSON malformado
	if malformed > 0 {
		issues = append(issues, LintIssue{
			Type:   "frontmatter_invalid",
			Detail: fmt.Sprintf("%d linhas malformadas ignoradas no JSONL", malformed),
		})
	}

	res := LintResult{Pages: len(pages), Issues: issues}
	if asJSON {
		_ = writeLintJSON(out, res)
	} else {
		writeLintText(out, res)
	}
	return res, nil
}

func writeLintText(out io.Writer, res LintResult) {
	fmt.Fprintf(out, "memory lint: %d pages\n", res.Pages)
	if len(res.Issues) == 0 {
		fmt.Fprintln(out, "  OK - nenhum problema encontrado")
		return
	}
	byType := map[string]int{}
	for _, iss := range res.Issues {
		byType[iss.Type]++
	}
	for t, n := range byType {
		fmt.Fprintf(out, "  %s: %d\n", t, n)
	}
	for _, iss := range res.Issues {
		page := iss.Page
		if page == "" {
			page = "(global)"
		}
		fmt.Fprintf(out, "    [%s] %s — %s\n", iss.Type, page, iss.Detail)
	}
}

func writeLintJSON(out io.Writer, res LintResult) error {
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(data))
	return err
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// sort local sem importar sort para evitar alocacao
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

// resolveJSONLPath monta o path canonico do JSONL seguindo o mesmo padrao
// de resolveCacheDir (command.go:164) mas com override explicito para test.
func resolveJSONLPath(override string) (string, error) {
	if override != "" {
		return filepath.Abs(override)
	}
	dir, err := resolveCacheDir("")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "memory_pages.jsonl"), nil
}

// lintFlags agrega flags do subcommand lint.
type lintFlags struct {
	fs       *flag.FlagSet
	asJSON   bool
	cacheDir string
}

func newLintFlags(name string) *lintFlags {
	f := &lintFlags{}
	f.fs = flag.NewFlagSet(name, flag.ContinueOnError)
	f.fs.BoolVar(&f.asJSON, "json", false, "saida em JSON")
	f.fs.StringVar(&f.cacheDir, "cache-dir", "", "Diretorio do JSONL (default: ~/.cache/agent-sync)")
	return f
}

// runMemoryLint implementa `agent-sync memory lint`. A-61: roda os 3 checks
// sobre memory_pages.jsonl e sai com codigo 1 se encontrar problemas.
func runMemoryLint(args []string) error {
	f := newLintFlags("agent-sync memory lint")
	if err := f.fs.Parse(args); err != nil {
		return err
	}
	dir := f.cacheDir
	if dir == "" {
		dir = "" // resolveJSONLPath resolve via UserCacheDir
	}
	jsonlPath, err := resolveJSONLPath(dir)
	if err != nil {
		return err
	}
	res, err := RunLint(jsonlPath, os.Stdout, f.asJSON)
	if err != nil {
		return err
	}
	if len(res.Issues) > 0 {
		os.Exit(1)
	}
	return nil
}