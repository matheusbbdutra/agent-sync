// Package secretscan aplica um filtro leve (regex, sem chamada de LLM) para
// evitar que segredos óbvios (chaves de API, tokens, chaves privadas) sejam
// persistidos em cache local ou em arquivos de estado. Inspirado no conceito
// de "Memory Defense" de sistemas de memória de agente, adaptado sem
// dependências externas nem custo de tokens.
package secretscan

import "regexp"

type pattern struct {
	name string
	re   *regexp.Regexp
}

var patterns = []pattern{
	{"aws-access-key", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{"generic-api-key", regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`)},
	{"github-token", regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`)},
	{"slack-token", regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`)},
	{"jwt", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)},
	{"private-key-block", regexp.MustCompile(`-----BEGIN[ A-Z]*PRIVATE KEY-----[\s\S]*?-----END[ A-Z]*PRIVATE KEY-----`)},
}

// Redact substitui ocorrências de padrões conhecidos de segredo por um
// marcador, e devolve os nomes dos padrões encontrados (vazio se nenhum).
func Redact(text string) (redacted string, found []string) {
	redacted = text
	seen := map[string]bool{}
	for _, p := range patterns {
		if p.re.MatchString(redacted) {
			redacted = p.re.ReplaceAllString(redacted, "[REDACTED:"+p.name+"]")
			if !seen[p.name] {
				seen[p.name] = true
				found = append(found, p.name)
			}
		}
	}
	return redacted, found
}
