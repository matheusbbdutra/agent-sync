package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/matheusdutra/token-tools/internal/docscache"
)

const userAgent = "agent-sync-docs-fetch/1.0 (+https://github.com/matheusdutra/agent-sync)"

var (
	removableBlocks = []string{"script", "style", "noscript", "svg", "head", "template"}
	commentRe       = regexp.MustCompile(`(?s)<!--.*?-->`)
	blockBreakRe    = regexp.MustCompile(`(?i)</(p|div|section|article|li|ul|ol|tr|td|th|table|h[1-6]|header|footer|main|nav|pre|blockquote|dd|dt|dl|figure|figcaption|form|fieldset|aside)>|<br\s*/?>`)
	tagRe           = regexp.MustCompile(`(?is)<[^>]+>`)
	headingRe       = regexp.MustCompile(`(?is)<h([1-6])[^>]*>(.*?)</h[1-6]>`)
	spacesRe        = regexp.MustCompile(`[ \t]{2,}`)
)

type heading struct {
	Level int
	Text  string
}

type page struct {
	URL         string
	ContentType string
	Body        string
	Text        string
	FromCache   bool
}

type mirrorManifest struct {
	Pages []string `json:"pages"`
}

func isPlainContent(contentType, url string) bool {
	contentType = strings.ToLower(contentType)
	if strings.Contains(contentType, "text/markdown") || strings.Contains(contentType, "text/plain") {
		return true
	}
	lower := strings.ToLower(url)
	for _, ext := range []string{".md", ".markdown", ".rst", ".txt", ".adoc"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func extractText(body, contentType, url string) string {
	if isPlainContent(contentType, url) {
		return body
	}
	return htmlToText(body)
}

func fetchPage(url, cacheDir string, refresh bool, timeout time.Duration) (page, error) {
	if !refresh {
		if entry, ok := docscache.Load(cacheDir, url); ok {
			text := entry.Text
			if text == "" {
				// Backfill de entradas gravadas por versões anteriores do cache.
				text = extractText(entry.Body, entry.ContentType, url)
				_ = docscache.Save(cacheDir, url, entry.ContentType, entry.Body, text)
			}
			return page{URL: url, ContentType: entry.ContentType, Body: entry.Body, Text: text, FromCache: true}, nil
		}
	}

	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return page{}, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return page{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return page{}, fmt.Errorf("HTTP %d ao buscar %s", resp.StatusCode, url)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return page{}, err
	}

	contentType := resp.Header.Get("Content-Type")
	body := string(raw)
	text := extractText(body, contentType, url)
	_ = docscache.Save(cacheDir, url, contentType, body, text)
	return page{URL: url, ContentType: contentType, Body: body, Text: text, FromCache: false}, nil
}

func htmlToText(input string) string {
	s := commentRe.ReplaceAllString(input, " ")
	for _, tag := range removableBlocks {
		re := regexp.MustCompile(`(?is)<` + tag + `\b[^>]*>.*?</` + tag + `>`)
		s = re.ReplaceAllString(s, " ")
	}
	s = blockBreakRe.ReplaceAllString(s, "\n")
	s = tagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = spacesRe.ReplaceAllString(s, " ")

	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func extractHeadings(input string) []heading {
	matches := headingRe.FindAllStringSubmatch(input, -1)
	headings := make([]heading, 0, len(matches))
	for _, m := range matches {
		level, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		text := strings.TrimSpace(html.UnescapeString(tagRe.ReplaceAllString(m[2], "")))
		if text != "" {
			headings = append(headings, heading{Level: level, Text: text})
		}
	}
	return headings
}

func renderOutline(headings []heading) string {
	var b strings.Builder
	for _, h := range headings {
		b.WriteString(strings.Repeat("  ", h.Level-1))
		b.WriteString("- ")
		b.WriteString(h.Text)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func applyFilters(content, grep string, maxLines int) string {
	lines := strings.Split(content, "\n")
	if grep != "" {
		needle := strings.ToLower(grep)
		var kept []string
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), needle) {
				kept = append(kept, line)
			}
		}
		lines = kept
	}
	if maxLines > 0 && len(lines) > maxLines {
		lines = append(lines[:maxLines], fmt.Sprintf("... (%d linhas ocultadas)", len(lines)-maxLines))
	}
	return strings.Join(lines, "\n")
}

func runMirror(manifestPath, cacheDir string, refresh bool, timeout time.Duration) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var manifest mirrorManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("manifest inválido (%s): %w", manifestPath, err)
	}
	if len(manifest.Pages) == 0 {
		return fmt.Errorf("manifest %s não lista páginas", manifestPath)
	}

	fmt.Printf("📦 Sincronizando %d página(s) para %s\n", len(manifest.Pages), cacheDir)
	fetched, failed := 0, 0
	for _, url := range manifest.Pages {
		pg, err := fetchPage(url, cacheDir, refresh, timeout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  %s: %v\n", url, err)
			failed++
			continue
		}
		status := "baixado"
		if pg.FromCache {
			status = "cache"
		}
		fmt.Printf("✅ %s (%s)\n", pg.URL, status)
		fetched++
	}
	fmt.Printf("\n✨ %d sincronizada(s), %d falha(s).\n", fetched, failed)
	if failed > 0 {
		return fmt.Errorf("%d página(s) falharam", failed)
	}
	return nil
}

func runSearch(cacheDir, term string, maxLines int) error {
	matches, err := docscache.Search(cacheDir, term, 20)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		fmt.Printf("Nenhum resultado para %q em %s\n", term, cacheDir)
		return nil
	}
	fmt.Printf("🔎 %d fonte(s) com %q:\n", len(matches), term)
	for _, m := range matches {
		fmt.Printf("\n📄 %s\n", m.URL)
		if m.Heading != "" && m.Heading != "Geral" {
			fmt.Printf("   [%s]\n", m.Heading)
		}
		for _, line := range m.Lines {
			fmt.Printf("   %s\n", line)
		}
	}
	return nil
}

func runList(cacheDir string) error {
	entries, err := docscache.List(cacheDir)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Printf("Cache vazio em %s\n", cacheDir)
		return nil
	}
	urls := make([]string, 0, len(entries))
	for _, e := range entries {
		urls = append(urls, e.URL)
	}
	sort.Strings(urls)
	fmt.Printf("📚 %d página(s) em cache (%s):\n", len(urls), cacheDir)
	for _, u := range urls {
		fmt.Println(" -", u)
	}
	return nil
}

func main() {
	outlineFlag := flag.Bool("outline", false, "Exibe apenas os títulos (h1-h6)")
	rawFlag := flag.Bool("raw", false, "Exibe o conteúdo bruto baixado, sem processar")
	refreshFlag := flag.Bool("refresh", false, "Ignora o cache e baixa novamente")
	grepFlag := flag.String("grep", "", "Mantém apenas linhas que contenham o termo")
	maxLines := flag.Int("max", 400, "Máximo de linhas na saída (0 = ilimitado)")
	cacheDirFlag := flag.String("cache-dir", "", "Diretório de cache (default ~/.cache/agent-sync/docs)")
	timeoutFlag := flag.Duration("timeout", 25*time.Second, "Timeout da requisição HTTP")
	mirrorFlag := flag.String("mirror", "", "Sincroniza um manifest JSON de páginas ({ \"pages\": [...] })")
	searchFlag := flag.String("search", "", "Busca offline um termo nas docs já cacheadas")
	listFlag := flag.Bool("list", false, "Lista as docs cacheadas")
	flag.Parse()

	cacheDir := *cacheDirFlag
	if cacheDir == "" {
		cacheDir = docscache.DefaultDir()
	}

	switch {
	case *listFlag:
		if err := runList(cacheDir); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}
		return
	case *searchFlag != "":
		if err := runSearch(cacheDir, *searchFlag, *maxLines); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}
		return
	case *mirrorFlag != "":
		if err := runMirror(*mirrorFlag, cacheDir, *refreshFlag, *timeoutFlag); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		fmt.Println("Uso: docs-fetch [flags] <url>")
		fmt.Println("  -outline        só títulos (útil para mapear a doc)")
		fmt.Println("  -grep <termo>   filtra linhas")
		fmt.Println("  -raw            conteúdo bruto")
		fmt.Println("  -refresh        ignora cache")
		fmt.Println("  -max <n>        limite de linhas (default 400)")
		fmt.Println("  -mirror <json>  sincroniza um manifest de páginas")
		fmt.Println("  -search <termo> busca offline no cache")
		fmt.Println("  -list           lista as docs cacheadas")
		os.Exit(1)
	}

	pg, err := fetchPage(args[0], cacheDir, *refreshFlag, *timeoutFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}

	status := "baixado"
	if pg.FromCache {
		status = "cache"
	}
	fmt.Fprintf(os.Stderr, "📄 %s (%s, %s)\n", pg.URL, strings.Split(pg.ContentType, ";")[0], status)

	var content string
	switch {
	case *rawFlag:
		content = pg.Body
	case *outlineFlag:
		content = renderOutline(extractHeadings(pg.Body))
	default:
		content = pg.Text
	}

	fmt.Println(applyFilters(content, *grepFlag, *maxLines))
}
