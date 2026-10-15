// docs-cache-write persiste no cache local do docs-fetch (usado também pelo
// docs-mcp) o conteúdo de uma doc já consultada por outra ferramenta
// (WebFetch, context7, read_url_content etc.), sem refazer a requisição de
// rede.
//
// Dois modos (regra §4 Linguagens & Runtime: Python-zero, Go é fonte da verdade):
//   - direto (default): lê um único objeto JSON via stdin: {"url", "contentType", "text"}.
//   - hook (--from-hook): lê o payload cru do hook PostToolUse via stdin e extrai
//     {url,text} internamente (antes feito por hooks/docs-cache.py, removido).
//     --cli=auto|claude|codex|antigravity (auto detecta via presença de "toolCall").
//
// "url" pode ser uma URL real ou uma chave sintética (ex: context7://<lib>/<query>).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/matheusdutra/token-tools/internal/docscache"
	"github.com/matheusdutra/token-tools/internal/secretscan"
)

type input struct {
	URL         string `json:"url"`
	ContentType string `json:"contentType"`
	Text        string `json:"text"`
}

func main() {
	cacheDirFlag := flag.String("cache-dir", "", "Diretório de cache (default ~/.cache/agent-sync/docs)")
	fromHookFlag := flag.Bool("from-hook", false, "Lê payload cru do hook PostToolUse via stdin e extrai {url,text} (antes via Python)")
	cliFlag := flag.String("cli", "auto", "CLI do payload no modo --from-hook: auto|claude|codex|antigravity")
	flag.Parse()

	cacheDir := *cacheDirFlag
	if cacheDir == "" {
		cacheDir = docscache.DefaultDir()
	}

	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docs-cache-write: erro lendo stdin: %v\n", err)
		os.Exit(1)
	}

	var in input
	if *fromHookFlag {
		url, text := extractFromHook(raw, *cliFlag)
		if url == "" || text == "" {
			return
		}
		in = input{URL: url, ContentType: "text/plain", Text: text}
	} else {
		if err := json.Unmarshal(raw, &in); err != nil {
			fmt.Fprintf(os.Stderr, "docs-cache-write: JSON inválido: %v\n", err)
			os.Exit(1)
		}
	}
	if in.URL == "" || in.Text == "" {
		// Nada de útil pra cachear (ex: contexto vazio, resultado só de erro).
		// Sai silenciosamente com sucesso — não é uma falha do hook.
		return
	}
	if in.ContentType == "" {
		in.ContentType = "text/plain"
	}

	text, found := secretscan.Redact(in.Text)
	if len(found) > 0 {
		fmt.Fprintf(os.Stderr, "docs-cache-write: padrões de segredo redigidos antes de cachear (%s): %v\n", in.URL, found)
	}

	if err := docscache.Save(cacheDir, in.URL, in.ContentType, text, text); err != nil {
		fmt.Fprintf(os.Stderr, "docs-cache-write: erro salvando cache: %v\n", err)
		os.Exit(1)
	}
}
