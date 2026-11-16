package promptlint

import (
	"strings"
	"testing"
)

// longProse devolve um prompt acima de longPromptChars sem nenhum marcador de
// fim: serve de base para o check no-terminator.
func longProse() string {
	return strings.Repeat("O time de suporte responde o cliente pelo canal oficial e registra o protocolo em ata. ", 12)
}

func countByCheck(findings []Finding, check string) int {
	n := 0
	for _, f := range findings {
		if f.Check == check {
			n++
		}
	}
	return n
}

// --- check 1: negation -----------------------------------------------------

func TestCheckNegation(t *testing.T) {
	cases := []struct {
		name   string
		prompt string
		want   int
	}{
		{
			name:   "proibicao sem alternativa positiva dispara",
			prompt: "Voce e o revisor de contrato.\nNunca invente numero de clausula.\n",
			want:   1,
		},
		{
			name:   "alternativa positiva na mesma frase nao dispara",
			prompt: "Nao use girias; escreva em registro formal.\n",
			want:   0,
		},
		{
			name:   "alternativa positiva na linha seguinte nao dispara",
			prompt: "Never guess an API name.\nUse apenas o que a fonte mostra.\n",
			want:   0,
		},
		{
			name:   "palavra derivada nao dispara",
			prompt: "Avoidable delays are tracked by the scheduler.\n",
			want:   0,
		},
		{
			name:   "oracao subordinada descritiva nao dispara",
			prompt: "Preencha com null o campo que nao aparecer no texto.\n",
			want:   0,
		},
		{
			name:   "proibicao coordenada dispara",
			prompt: "Cite o protocolo e nao prometa prazo que o contrato nao preve.\n",
			want:   1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkNegation(splitLines(tc.prompt))
			if len(got) != tc.want {
				t.Fatalf("achados = %d, quer %d: %+v", len(got), tc.want, got)
			}
			for _, f := range got {
				if f.Check != CheckNegation || f.Severity != SeverityWarning {
					t.Errorf("finding fora do contrato: %+v", f)
				}
				if f.Fix == "" || f.Message == "" {
					t.Errorf("finding sem mensagem ou sugestao: %+v", f)
				}
			}
		})
	}
}

// --- check 2: no-terminator ------------------------------------------------

func TestCheckNoTerminator(t *testing.T) {
	cases := []struct {
		name   string
		prompt string
		want   int
	}{
		{
			name:   "prompt curto sem marcador nao dispara",
			prompt: "Resuma o texto em tres frases.",
			want:   0,
		},
		{
			name:   "prompt longo com secao final nao dispara",
			prompt: longProse() + "\n# Further Reading\n",
			want:   0,
		},
		{
			name:   "prompt longo sem marcador dispara",
			prompt: longProse(),
			want:   1,
		},
		{
			name:   "prompt longo terminando em cerca de fechamento nao dispara",
			prompt: longProse() + "\n```\n",
			want:   0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkNoTerminator(tc.prompt)
			if len(got) != tc.want {
				t.Fatalf("achados = %d, quer %d: %+v", len(got), tc.want, got)
			}
			if tc.want > 0 && got[0].Line == 0 {
				t.Errorf("finding de ausencia precisa apontar linha: %+v", got[0])
			}
		})
	}
}

// --- check 3: undeclared-output-format ------------------------------------

func TestCheckUndeclaredOutputFormat(t *testing.T) {
	cases := []struct {
		name   string
		prompt string
		want   int
	}{
		{
			name:   "pedido de saida sem formato dispara",
			prompt: "Resuma o ticket e responda com o parecer para o cliente.\n",
			want:   1,
		},
		{
			name:   "formato declarado nao dispara",
			prompt: "Responda em JSON com os campos id e nome.\n",
			want:   0,
		},
		{
			name:   "formato uma linha nao dispara",
			prompt: "Extraia os nomes do texto e devolva uma linha por nome.\n",
			want:   0,
		},
		{
			name:   "prompt sem verbo de saida nao dispara",
			prompt: "O sistema processa arquivos de nota fiscal durante a madrugada.\n",
			want:   0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkUndeclaredOutputFormat(splitLines(tc.prompt))
			if len(got) != tc.want {
				t.Fatalf("achados = %d, quer %d: %+v", len(got), tc.want, got)
			}
			for _, f := range got {
				if f.Severity != SeverityError {
					t.Errorf("severidade = %q, quer %q", f.Severity, SeverityError)
				}
			}
		})
	}
}

// --- check 4: non-disjoint-classes ---------------------------------------

func TestCheckNonDisjointClasses(t *testing.T) {
	cases := []struct {
		name        string
		prompt      string
		wantOverlap int
		wantChoice  int
	}{
		{
			name:        "rotulo contido em outro dispara",
			prompt:      "Classifique o ticket.\n- bug\n- bug critico\nEscolha exatamente uma opcao.\n",
			wantOverlap: 1,
			wantChoice:  0,
		},
		{
			name:        "sem instrucao de escolha unica dispara",
			prompt:      "Classifique o ticket.\n- bug\n- duvida\n",
			wantOverlap: 0,
			wantChoice:  1,
		},
		{
			name:        "opcoes disjuntas com escolha unica nao disparam",
			prompt:      "Rotule a mensagem.\n- bug critico\n- duvida comercial\nescolha exatamente uma categoria.\n",
			wantOverlap: 0,
			wantChoice:  0,
		},
		{
			name:        "prompt sem classificacao nem lista nao dispara",
			prompt:      "Escreva um resumo do texto em tres frases.\n",
			wantOverlap: 0,
			wantChoice:  0,
		},
		{
			name:        "lista de instrucoes numeradas nao vira opcao",
			prompt:      "1. Extraia o valor total do texto.\n2. Preencha com null o campo ausente.\n",
			wantOverlap: 0,
			wantChoice:  0,
		},
		{
			name:        "bloco de regras com verbo nao vira opcao",
			prompt:      "Classifique o ticket.\n\n- Nao use girias no parecer.\n- Evite adjetivo no parecer.\n\n- bug\n- bug critico\nescolha exatamente uma opcao.\n",
			wantOverlap: 1,
			wantChoice:  0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkNonDisjointClasses(splitLines(tc.prompt))
			overlap := 0
			choice := 0
			for _, f := range got {
				if strings.Contains(f.Message, "nao sao disjuntas") {
					overlap++
					continue
				}
				if strings.Contains(f.Message, "escolha unica") {
					choice++
					continue
				}
				t.Errorf("mensagem fora do contrato: %+v", f)
			}
			if overlap != tc.wantOverlap || choice != tc.wantChoice {
				t.Fatalf("overlap=%d (quer %d) escolha=%d (quer %d): %+v",
					overlap, tc.wantOverlap, choice, tc.wantChoice, got)
			}
		})
	}
}

// --- check 5: placeholder-leak -------------------------------------------

func TestCheckPlaceholderLeak(t *testing.T) {
	cases := []struct {
		name   string
		prompt string
		want   int
	}{
		{
			name:   "chave dupla nao renderizada dispara",
			prompt: "Ola {{nome_do_cliente}}, siga as instrucoes abaixo.\n",
			want:   1,
		},
		{
			name:   "placeholder entre angulares dispara",
			prompt: "Preencha <NOME_DO_CLIENTE> antes de publicar.\n",
			want:   1,
		},
		{
			name:   "marcador de pendencia dispara",
			prompt: "Revise a instrucao TODO antes de publicar.\n",
			want:   1,
		},
		{
			name:   "tag de markup minuscula nao dispara",
			prompt: "Devolva o resultado dentro de <output>texto</output>.\n",
			want:   0,
		},
		{
			name:   "palavra todo em minuscula nao dispara",
			prompt: "Todo o texto deve estar em registro formal.\n",
			want:   0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkPlaceholderLeak(splitLines(tc.prompt))
			if len(got) != tc.want {
				t.Fatalf("achados = %d, quer %d: %+v", len(got), tc.want, got)
			}
		})
	}
}

// --- Check / Summarize / FilterBySeverity --------------------------------

func TestCheckAggregaEOrdenaPorLinha(t *testing.T) {
	prompt := "Nunca invente nome de funcao.\nContexto: {{cliente}}\n"
	got := Check(prompt)
	if len(got) != 2 {
		t.Fatalf("achados = %d, quer 2: %+v", len(got), got)
	}
	if got[0].Line != 1 || got[0].Check != CheckNegation {
		t.Errorf("primeiro achado = %+v, quer negation na linha 1", got[0])
	}
	if got[1].Line != 2 || got[1].Check != CheckPlaceholderLeak {
		t.Errorf("segundo achado = %+v, quer placeholder-leak na linha 2", got[1])
	}
}

func TestSummarizeContaAsCincoChaves(t *testing.T) {
	summary := Summarize(Check("Nunca invente nome de funcao.\n"))
	if summary.Total != 1 || summary.Warnings != 1 || summary.Errors != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	for _, code := range CheckCodes() {
		if _, ok := summary.ByCheck[code]; !ok {
			t.Errorf("by_check sem a chave %q: %+v", code, summary.ByCheck)
		}
	}
	if len(summary.ByCheck) != len(CheckCodes()) {
		t.Errorf("by_check tem %d chaves, quer %d", len(summary.ByCheck), len(CheckCodes()))
	}
}

func TestFilterBySeverity(t *testing.T) {
	findings := Check("Nunca invente nome de funcao.\nContexto: {{cliente}}\n")

	all, err := FilterBySeverity(findings, SeverityWarning)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("warning manteve %d achados, quer 2", len(all))
	}

	onlyErrors, err := FilterBySeverity(findings, SeverityError)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(onlyErrors) != 1 || onlyErrors[0].Check != CheckPlaceholderLeak {
		t.Fatalf("error manteve %+v, quer so placeholder-leak", onlyErrors)
	}

	if _, err := FilterBySeverity(findings, "info"); err == nil {
		t.Fatal("severidade invalida deveria devolver erro")
	}
}

func TestCheckPromptVazioNaoDispara(t *testing.T) {
	if got := Check(""); len(got) != 0 {
		t.Fatalf("prompt vazio devolveu %+v", got)
	}
	if got := countByCheck(Check("   \n\n"), CheckNegation); got != 0 {
		t.Fatalf("prompt em branco disparou negation")
	}
}
