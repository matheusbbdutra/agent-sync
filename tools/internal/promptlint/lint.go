// Package promptlint faz lint mecanico de prompt: detecta defeitos
// estruturais que o proprio texto revela, sem chamar modelo e sem julgar
// qualidade semantica. Cada check tem codigo, severidade, mensagem e sugestao
// de correcao proprios; a saida e deterministica para o mesmo texto.
package promptlint

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Severidades reconhecidas em Finding.Severity, em ordem de precedencia
// (error > warning). Proibicao sem alternativa positiva (p.102) e prompt longo
// sem marcador de fim (p.161-162) sao defeitos de forma e ficam em warning; os
// demais quebram o contrato com o codigo que consome a saida e ficam em error.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// Codigos de check. Viram chave de Finding.Check e da contagem por check.
const (
	CheckNegation               = "negation"
	CheckNoTerminator           = "no-terminator"
	CheckUndeclaredOutputFormat = "undeclared-output-format"
	CheckNonDisjointClasses     = "non-disjoint-classes"
	CheckPlaceholderLeak        = "placeholder-leak"
)

// Finding e 1 defeito estrutural encontrado no prompt. Line e 1-based; para
// defeito de ausencia (no-terminator) aponta a ultima linha, onde o marcador
// deveria estar.
type Finding struct {
	Check    string `json:"check"`
	Severity string `json:"severity"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
	Fix      string `json:"fix"`
}

// CheckCodes devolve os checks na ordem usada na contagem final da CLI.
func CheckCodes() []string {
	return []string{
		CheckNegation,
		CheckNoTerminator,
		CheckUndeclaredOutputFormat,
		CheckNonDisjointClasses,
		CheckPlaceholderLeak,
	}
}

// Check roda os 5 checks sobre o prompt e devolve os achados ordenados por
// linha (desempate pelo codigo do check).
func Check(prompt string) []Finding {
	lines := splitLines(prompt)
	var findings []Finding
	findings = append(findings, checkNegation(lines)...)
	findings = append(findings, checkNoTerminator(prompt)...)
	findings = append(findings, checkUndeclaredOutputFormat(lines)...)
	findings = append(findings, checkNonDisjointClasses(lines)...)
	findings = append(findings, checkPlaceholderLeak(lines)...)

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Check < findings[j].Check
	})
	return findings
}

// splitLines normaliza CRLF e devolve as linhas, mantendo o indice 1-based.
func splitLines(prompt string) []string {
	return strings.Split(strings.ReplaceAll(prompt, "\r\n", "\n"), "\n")
}

// --- check 1: negation -----------------------------------------------------

// prohibitionRE casa instrucao escrita como proibicao (EN e PT-BR com e sem
// acento). "-- p.102: ask for positives instead of negatives and dos instead of
// don'ts."
var prohibitionRE = regexp.MustCompile(`(?i)\b(don'?t|do not|never|avoid|n[aã]o|nunca|evite)\b`)

// positiveCueRE marca a alternativa positiva que deve acompanhar a proibicao na
// mesma frase. So conta quando aparece DEPOIS da proibicao: em "Nao use girias"
// o "use" pertence a propria proibicao, nao a uma alternativa.
var positiveCueRE = regexp.MustCompile(`(?i)\b(em vez|ao inv[eé]s|instead|prefira|troque|substitua|use|utilize|escreva|retorne|responda|gere|produza|inclua|cite|formate)\b`)

// imperativeStartRE reconhece a linha seguinte que ja da o caminho positivo,
// forma aceita pela fonte ("bolster your command with a reason", p.102).
var imperativeStartRE = regexp.MustCompile(`(?i)^\s*(?:[-*+]\s*|\d+[.)]\s*)?(use|utilize|escreva|retorne|responda|gere|produza|inclua|cite|formate|liste|classifique|extraia|prefira|escolha|mantenha|write|return|respond|generate|produce|include|cite|format|list|classify|extract|prefer|choose|keep)\b`)

// checkNegation sinaliza proibicao que ficou sem alternativa positiva. Fonte:
// p.102 (Ask for positives instead of negatives and dos instead of don'ts).
func checkNegation(lines []string) []Finding {
	var findings []Finding
	for i, line := range lines {
		loc := prohibitionRE.FindStringIndex(line)
		if loc == nil {
			continue
		}
		// Oracao subordinada descritiva ("o campo que nao aparecer", "se nao
		// houver") nao e instrucao: o marcador precisa abrir clausula.
		if !startsClause(line[:loc[0]]) {
			continue
		}
		if positiveCueRE.MatchString(line[loc[1]:]) {
			continue
		}
		if nextLineIsImperative(lines, i+1) {
			continue
		}
		findings = append(findings, Finding{
			Check:    CheckNegation,
			Severity: SeverityWarning,
			Line:     i + 1,
			Message:  fmt.Sprintf("proibicao %q sem alternativa positiva: o modelo recebe o que evitar, nao o que fazer", strings.TrimSpace(line[loc[0]:loc[1]])),
			Fix:      `Reescreva como afirmacao do que fazer: "Responda em JSON" no lugar de "Nao responda em texto" (p.102).`,
		})
	}
	return findings
}

// startsClause informa se o prefixo antes da proibicao termina em inicio de
// clausula: comeco da linha, marcador de lista, pontuacao de fim de frase ou
// coordenacao ("e nao", "mas nao", "nem").
func startsClause(prefix string) bool {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		return true
	}
	last, _ := utf8.DecodeLastRuneInString(trimmed)
	if strings.ContainsRune(".!?:;*>-", last) {
		return true
	}
	for _, coord := range []string{"e", "mas", "ou", "nem"} {
		if strings.HasSuffix(trimmed, " "+coord) || trimmed == coord {
			return true
		}
	}
	return false
}

// nextLineIsImperative informa se a proxima linha nao vazia comeca com verbo no
// imperativo — indice testavel de que a alternativa positiva ja foi dada.
func nextLineIsImperative(lines []string, from int) bool {
	for i := from; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		return imperativeStartRE.MatchString(lines[i])
	}
	return false
}

// --- check 2: no-terminator ------------------------------------------------

// longPromptChars e o limiar a partir do qual a falta de marcador de fim passa a
// contar. Prompt curto cabe na janela de atencao sem secao sentinela; a partir
// daqui a fonte manda fechar inicio e fim reconheciveis (p.161-162).
const longPromptChars = 800

// terminatorRE casa marcador de fim/parada declarado no texto. O lint nao
// infere ordem: qualquer ocorrencia do marcador satisfaz o check.
var terminatorRE = regexp.MustCompile(`(?i)(\bfurther reading\b|\bappendix\b|\bap[eê]ndice\b|\brefer[eê]ncias\b|\bfim do prompt\b|\bpare ap[oó]s\b|\bpare depois\b|\bresponda apenas\b|\bresponda somente\b|\banswer only\b|\breply only\b|\bstop\b)`)

// closingDelimiterRE casa o fechamento de bloco que tambem serve de terminador
// testavel por substring (p.161-162).
var closingDelimiterRE = regexp.MustCompile("^\\s*(```\\S*|~~~\\S*|---|\\*\\*\\*|</[A-Za-z][\\w-]*>)\\s*$")

// checkNoTerminator sinaliza prompt longo sem marcador de fim. Fonte:
// p.161-162 (o fim da resposta precisa ser testavel por substring).
func checkNoTerminator(prompt string) []Finding {
	if len(prompt) <= longPromptChars {
		return nil
	}
	lines := splitLines(prompt)
	if terminatorRE.MatchString(prompt) {
		return nil
	}
	lastLine := 0
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		lastLine = i + 1
		if closingDelimiterRE.MatchString(lines[i]) {
			return nil
		}
		break
	}
	return []Finding{{
		Check:    CheckNoTerminator,
		Severity: SeverityWarning,
		Line:     lastLine,
		Message:  fmt.Sprintf("prompt com %d chars sem marcador de fim: o modelo nao tem ponto de parada reconhecivel", len(prompt)),
		Fix:      `Feche com secao sentinela ("# Further Reading", "# Apendice"), instrucao de parada ("Pare apos a secao Conclusao") ou cerca de codigo de fechamento (p.161-162).`,
	}}
}

// --- check 3: undeclared-output-format ------------------------------------

// outputVerbRE casa o pedido explicito de saida (PT-BR e EN).
var outputVerbRE = regexp.MustCompile(`(?i)\b(retorne|retornar|gere|gerar|produza|produzir|responda|responder|classifique|classificar|extraia|extrair|return|generate|produce|respond|classify|extract)\b`)

// formatMarkerRE casa a declaracao concreta de formato, que e o que torna a
// saida parseavel. Fonte: p.143 (structured document).
var formatMarkerRE = regexp.MustCompile(`(?i)(\bjson\b|\bxml\b|\byaml\b|\btoml\b|\bcsv\b|\bmarkdown\b|\btabela\b|\btable\b|\bschema\b|\buma linha\b|\bapenas o\b|\bapenas a\b|\bone line\b|\bonly the\b)`)

// checkUndeclaredOutputFormat sinaliza prompt que pede saida sem declarar
// formato. Emite 1 achado por prompt, na primeira linha com verbo de saida.
// Fonte: p.143.
func checkUndeclaredOutputFormat(lines []string) []Finding {
	for _, line := range lines {
		if formatMarkerRE.MatchString(line) {
			return nil
		}
	}
	for i, line := range lines {
		loc := outputVerbRE.FindStringIndex(line)
		if loc == nil {
			continue
		}
		return []Finding{{
			Check:    CheckUndeclaredOutputFormat,
			Severity: SeverityError,
			Line:     i + 1,
			Message:  fmt.Sprintf("pede saida (%q) sem declarar formato parseavel", strings.TrimSpace(line[loc[0]:loc[1]])),
			Fix:      `Declare o formato na propria instrucao: "Responda com um objeto JSON {campo: tipo}" ou "uma linha por registro" (p.143).`,
		}}
	}
	return nil
}

// --- check 4: non-disjoint-classes ---------------------------------------

// classificationCueRE casa o vocabulario de classificacao.
var classificationCueRE = regexp.MustCompile(`(?i)\b(classifique|classificar|classifica[cç][aã]o|categoria|categorias|rotule|r[oó]tulo|label|classify|category|categories)\b`)

// singleChoiceRE casa a instrucao que obriga escolher 1 opcao. Fonte: p.165.
var singleChoiceRE = regexp.MustCompile(`(?i)(exatamente uma|apenas uma|uma [uú]nica|somente uma|exactly one|only one)`)

// listItemRE casa item de lista com marcador ou numeracao.
var listItemRE = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+(.+?)\s*$`)

// optionLabelMaxRunes limita o que conta como rotulo: item de lista mais longo
// que isso e prosa ou instrucao, nao opcao de classificacao.
const optionLabelMaxRunes = 24

// option e 1 rotulo candidato extraido de item de lista.
type option struct {
	label string
	line  int
}

// optionLabel reduz o item de lista a um rotulo curto: corta a explicacao apos
// ':' ou travessao e recusa item longo (prosa). Devolve "" quando o item nao
// parece rotulo.
func optionLabel(content string) string {
	for _, sep := range []string{" — ", " - ", ": ", ":"} {
		if idx := strings.Index(content, sep); idx > 0 {
			content = content[:idx]
			break
		}
	}
	content = strings.TrimSpace(strings.Trim(content, "`"))
	content = strings.TrimSuffix(content, ".")
	content = strings.Trim(content, "`")
	if content == "" || utf8.RuneCountInString(content) > optionLabelMaxRunes {
		return ""
	}
	return content
}

// extractOptions coleta os rotulos de lista de opcoes: bloco de 2+ itens
// consecutivos cujo texto, cortado na explicacao, cabe em optionLabelMaxRunes e
// nao comeca com verbo no imperativo — senao e lista de instrucoes numeradas,
// nao lista de opcoes.
func extractOptions(lines []string) []option {
	var opts []option
	var block []option
	flush := func() {
		if len(block) >= 2 {
			opts = append(opts, block...)
		}
		block = nil
	}
	for i, line := range lines {
		m := listItemRE.FindStringSubmatch(line)
		if m == nil {
			flush()
			continue
		}
		label := optionLabel(m[1])
		if label == "" || imperativeStartRE.MatchString(label) {
			flush()
			continue
		}
		block = append(block, option{label: label, line: i + 1})
	}
	flush()
	return opts
}

// checkNonDisjointClasses sinaliza opcoes de classificacao que nao sao
// disjuntas entre si (uma e substring de outra) ou prompt que nao exige escolha
// unica. Fonte: p.165 (garanta primeiro token unico por opcao e proiba resposta
// com varias alternativas).
func checkNonDisjointClasses(lines []string) []Finding {
	opts := extractOptions(lines)

	cueLine := 0
	for i, line := range lines {
		if classificationCueRE.MatchString(line) {
			cueLine = i + 1
			break
		}
	}
	if cueLine == 0 && len(opts) < 2 {
		return nil
	}
	if cueLine == 0 {
		cueLine = opts[0].line
	}

	var findings []Finding
	emitted := map[string]bool{}
	for i := range opts {
		for j := range opts {
			if i == j {
				continue
			}
			inner := strings.ToLower(opts[i].label)
			outer := strings.ToLower(opts[j].label)
			if inner == outer || !strings.Contains(outer, inner) {
				continue
			}
			key := fmt.Sprintf("%d>%d", opts[i].line, opts[j].line)
			if emitted[key] {
				continue
			}
			emitted[key] = true
			findings = append(findings, Finding{
				Check:    CheckNonDisjointClasses,
				Severity: SeverityError,
				Line:     opts[j].line,
				Message:  fmt.Sprintf("rotulo %q contem %q: as opcoes nao sao disjuntas", opts[j].label, opts[i].label),
				Fix:      `Troque por rotulos mutuamente exclusivos ("bug critico" e "bug de UI", nao "bug" e "bug critico") — p.165.`,
			})
		}
	}

	if len(opts) >= 2 && !singleChoiceRE.MatchString(strings.Join(lines, "\n")) {
		findings = append(findings, Finding{
			Check:    CheckNonDisjointClasses,
			Severity: SeverityError,
			Line:     cueLine,
			Message:  fmt.Sprintf("%d opcoes de classificacao sem instrucao de escolha unica", len(opts)),
			Fix:      `Exija 1 rotulo por resposta ("escolha exatamente uma opcao") e proiba varias alternativas — p.165.`,
		})
	}

	return findings
}

// --- check 5: placeholder-leak -------------------------------------------

var (
	// doubleBraceRE casa placeholder de template nao renderizado ({{nome}}).
	doubleBraceRE = regexp.MustCompile(`\{\{[^{}\n]{1,80}\}\}`)
	// angleRE casa <...> candidato a placeholder; isAnglePlaceholder decide.
	angleRE = regexp.MustCompile(`<([A-Za-z][A-Za-z0-9 _\-]{0,40})>`)
	// todoRE casa marcador de pendencia. Case-sensitive de proposito: "todo" em
	// PT-BR/ES e palavra comum e viraria falso positivo.
	todoRE = regexp.MustCompile(`\b(TODO|FIXME|TBD|XXX)\b`)
)

// isAnglePlaceholder separa placeholder (<NOME>, <your name>, <CAMPO_X>) de tag
// de markup (<output>, <br>): tag XML usual e minuscula e sem espaco.
func isAnglePlaceholder(content string) bool {
	if strings.ContainsAny(content, " ") {
		return true
	}
	if content != strings.ToUpper(content) || utf8.RuneCountInString(content) < 2 {
		return false
	}
	return strings.IndexFunc(content, func(r rune) bool { return r >= 'A' && r <= 'Z' }) >= 0
}

// checkPlaceholderLeak sinaliza placeholder que iria literal para o modelo:
// o modelo o le como conteudo, nao como lacuna a preencher. Emite 1 achado por
// linha, com o primeiro marcador encontrado.
func checkPlaceholderLeak(lines []string) []Finding {
	var findings []Finding
	for i, line := range lines {
		marker := ""
		if m := doubleBraceRE.FindString(line); m != "" {
			marker = m
		} else if m := angleRE.FindStringSubmatch(line); m != nil && isAnglePlaceholder(m[1]) {
			marker = m[0]
		} else if m := todoRE.FindString(line); m != "" {
			marker = m
		}
		if marker == "" {
			continue
		}
		findings = append(findings, Finding{
			Check:    CheckPlaceholderLeak,
			Severity: SeverityError,
			Line:     i + 1,
			Message:  fmt.Sprintf("placeholder nao preenchido %q iria literal para o modelo", marker),
			Fix:      `Preencha o valor na renderizacao do template ou remova o placeholder antes de enviar o prompt.`,
		})
	}
	return findings
}

// --- agregacao ------------------------------------------------------------

// Summary agrega os achados por check e por severidade. ByCheck sempre traz as
// 5 chaves, inclusive com 0, para a contagem final da CLI ser estavel.
type Summary struct {
	Total    int            `json:"total"`
	Errors   int            `json:"errors"`
	Warnings int            `json:"warnings"`
	ByCheck  map[string]int `json:"by_check"`
}

// Summarize conta os achados por check e por severidade.
func Summarize(findings []Finding) Summary {
	s := Summary{ByCheck: map[string]int{}}
	for _, code := range CheckCodes() {
		s.ByCheck[code] = 0
	}
	for _, f := range findings {
		s.Total++
		s.ByCheck[f.Check]++
		switch f.Severity {
		case SeverityError:
			s.Errors++
		case SeverityWarning:
			s.Warnings++
		}
	}
	return s
}

// SeverityRank devolve a precedencia de uma severidade (error=2, warning=1).
// ok=false para valor fora do contrato.
func SeverityRank(severity string) (int, bool) {
	switch severity {
	case SeverityError:
		return 2, true
	case SeverityWarning:
		return 1, true
	default:
		return 0, false
	}
}

// FilterBySeverity mantem os achados com precedencia >= min. Devolve erro quando
// min nao e error nem warning, para a CLI reprovar com exit de uso.
func FilterBySeverity(findings []Finding, min string) ([]Finding, error) {
	minRank, ok := SeverityRank(min)
	if !ok {
		return nil, fmt.Errorf("severidade invalida %q (use error ou warning)", min)
	}
	kept := make([]Finding, 0, len(findings))
	for _, f := range findings {
		rank, known := SeverityRank(f.Severity)
		if !known || rank < minRank {
			continue
		}
		kept = append(kept, f)
	}
	return kept, nil
}
