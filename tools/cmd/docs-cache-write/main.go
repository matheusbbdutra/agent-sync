// docs-cache-write persiste no cache local do docs-fetch (usado também pelo
// docs-mcp) o conteúdo de uma doc já consultada por outra ferramenta
// (WebFetch, context7, read_url_content etc.), sem refazer a requisição de
// rede. Lê um único objeto JSON via stdin: {"url", "contentType", "text"}.
// "url" pode ser uma URL real ou uma chave sintética (ex: context7://<lib>/<query>).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/matheusdutra/token-tools/internal/docscache"
)

type input struct {
	URL         string `json:"url"`
	ContentType string `json:"contentType"`
	Text        string `json:"text"`
}

func main() {
	cacheDirFlag := flag.String("cache-dir", "", "Diretório de cache (default ~/.cache/agent-sync/docs)")
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
	if err := json.Unmarshal(raw, &in); err != nil {
		fmt.Fprintf(os.Stderr, "docs-cache-write: JSON inválido: %v\n", err)
		os.Exit(1)
	}
	if in.URL == "" || in.Text == "" {
		// Nada de útil pra cachear (ex: contexto vazio, resultado só de erro).
		// Sai silenciosamente com sucesso — não é uma falha do hook.
		return
	}
	if in.ContentType == "" {
		in.ContentType = "text/plain"
	}

	if err := docscache.Save(cacheDir, in.URL, in.ContentType, in.Text, in.Text); err != nil {
		fmt.Fprintf(os.Stderr, "docs-cache-write: erro salvando cache: %v\n", err)
		os.Exit(1)
	}
}
