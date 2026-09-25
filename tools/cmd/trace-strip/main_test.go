package main

import (
	"strings"
	"testing"

	"github.com/matheusdutra/token-tools/internal/tracestrip"
)

func TestFilterLinesRemovesVendorNoise(t *testing.T) {
	lines := []string{
		"Error: boom",
		"    at handler (/app/src/handler.js:10:5)",
		"    at Object.<anonymous> (/app/node_modules/express/lib/router.js:1:1)",
		"    at module.exports (/app/vendor/foo/bar.go:2)",
		"    at org.springframework.web.filter.Filter.doFilter(Filter.java:1)",
		"    at process.internal.process (internal/process/task.js:1:1)",
		"",
		"    at main (/app/src/main.js:1:1)",
	}
	kept, omitted := tracestrip.FilterLines(lines, 25)

	if omitted != 4 {
		t.Fatalf("esperava 4 linhas omitidas, got %d", omitted)
	}
	joined := strings.Join(kept, "\n")
	if strings.Contains(joined, "node_modules") || strings.Contains(joined, "springframework") {
		t.Fatalf("ruído não filtrado: %q", joined)
	}
	if !strings.Contains(joined, "handler") || !strings.Contains(joined, "main") {
		t.Fatalf("linhas relevantes faltando: %q", joined)
	}
}

func TestFilterLinesRespectsMax(t *testing.T) {
	lines := []string{"a", "b", "c", "d", "e"}
	kept, omitted := tracestrip.FilterLines(lines, 2)
	if len(kept) != 2 {
		t.Fatalf("esperava 2 linhas, got %d", len(kept))
	}
	if omitted != 0 {
		t.Fatalf("esperava 0 omitidas, got %d", omitted)
	}
}
