package main

import (
	"strings"
	"testing"
)

func TestHTMLToTextStripsNoise(t *testing.T) {
	input := `<html><head><title>X</title><style>.a{color:red}</style></head>
<body>
<script>alert('x')</script>
<h1>Instalação</h1>
<p>Rode o <code>composer</code> para instalar &amp; atualizar.</p>
<noscript>off</noscript>
<div>Segunda linha</div>
</body></html>`

	got := htmlToText(input)
	for _, unwanted := range []string{"alert", "color:red", "<title>", "off"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("ruído %q não removido:\n%s", unwanted, got)
		}
	}
	for _, want := range []string{"Instalação", "composer", "instalar & atualizar", "Segunda linha"} {
		if !strings.Contains(got, want) {
			t.Errorf("faltando %q em:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("linhas em branco não colapsadas:\n%q", got)
	}
}

func TestExtractHeadingsOrderAndLevels(t *testing.T) {
	input := `<h1>Intro</h1><p>x</p><h2>Uso <em>básico</em></h2><h3>Detalhe</h3><h2>Fim</h2>`
	headings := extractHeadings(input)

	if len(headings) != 4 {
		t.Fatalf("esperava 4 títulos, got %d: %+v", len(headings), headings)
	}
	wantLevels := []int{1, 2, 3, 2}
	wantTexts := []string{"Intro", "Uso básico", "Detalhe", "Fim"}
	for i, h := range headings {
		if h.Level != wantLevels[i] || h.Text != wantTexts[i] {
			t.Errorf("título %d = %+v, want level=%d text=%q", i, h, wantLevels[i], wantTexts[i])
		}
	}
}

func TestRenderOutline(t *testing.T) {
	got := renderOutline([]heading{{1, "A"}, {2, "B"}})
	if got != "- A\n  - B" {
		t.Fatalf("outline inesperado: %q", got)
	}
}

func TestIsPlainContent(t *testing.T) {
	if !isPlainContent("text/markdown; charset=utf-8", "https://x/y") {
		t.Error("markdown deveria ser plain")
	}
	if !isPlainContent("", "https://x/y.rst") {
		t.Error(".rst deveria ser plain")
	}
	if isPlainContent("text/html; charset=utf-8", "https://x/y") {
		t.Error("html não deveria ser plain")
	}
}

func TestApplyFiltersGrepAndMax(t *testing.T) {
	content := "alpha\nbeta\ngamma\nbeta dois"
	if got := applyFilters(content, "beta", 0); strings.Contains(got, "alpha") || !strings.Contains(got, "beta dois") {
		t.Fatalf("grep incorreto: %q", got)
	}
	if got := applyFilters("a\nb\nc\nd", "", 2); !strings.Contains(got, "linhas ocultadas") || !strings.HasPrefix(got, "a\nb") {
		t.Fatalf("max incorreto: %q", got)
	}
}
