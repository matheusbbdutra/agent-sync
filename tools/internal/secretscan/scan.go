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
	repl string // vazio = marcador padrão "[REDACTED:<name>]"
}

var patterns = []pattern{
	{"aws-access-key", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), ""},
	{"generic-api-key", regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`), ""},
	{"github-token", regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`), ""},
	{"gitlab-pat", regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{20,}\b`), ""},
	{"slack-token", regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`), ""},
	{"jwt", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`), ""},
	{"private-key-block", regexp.MustCompile(`-----BEGIN[ A-Z]*PRIVATE KEY-----[\s\S]*?-----END[ A-Z]*PRIVATE KEY-----`), ""},
	{"google-api-key", regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{30,}\b`), ""},
	{"stripe-key", regexp.MustCompile(`\b[sr]k_(?:live|test)_[0-9A-Za-z]{16,}\b`), ""},
	{"npm-token", regexp.MustCompile(`\bnpm_[A-Za-z0-9]{30,}\b`), ""},
	{"pypi-token", regexp.MustCompile(`\bpypi-[A-Za-z0-9_-]{40,}\b`), ""},
	{"huggingface-token", regexp.MustCompile(`\bhf_[A-Za-z0-9]{30,}\b`), ""},
	// Headers HTTP com credenciais inline (curl -H "Authorization: ...").
	{"auth-header", regexp.MustCompile(`(?i)\b(authorization\s*:\s*(?:bearer|basic|token)\s+)[A-Za-z0-9._~+/=-]{8,}`), "${1}[REDACTED:auth-header]"},
	// Atribuições estilo env/TOML com nomes que denunciam credencial
	// (MINIMAX_API_KEY=..., OPENAI_API_KEY = "..."). O valor precisa ter
	// 12+ chars de charset de chave para reduzir falso positivo em prosa.
	{"env-secret", regexp.MustCompile(`(?i)\b([A-Z][A-Z0-9_]*(?:API[_-]?KEY|ACCESS[_-]?TOKEN|AUTH[_-]?TOKEN|SECRET[_-]?KEY|CLIENT[_-]?SECRET|PASSWORD|PASSWD)[A-Z0-9_]*\s*=\s*)["']?[A-Za-z0-9._~+/=@-]{12,}`), "${1}[REDACTED:env-secret]"},
	// Atribuições YAML/TOML/JSON em minúsculas (api_key: ..., token = "...").
	{"yaml-secret", regexp.MustCompile(`(?i)\b((?:api[_-]?key|access[_-]?token|auth[_-]?token|secret[_-]?key|client[_-]?secret|token|password|passwd)\s*[:=]\s*)["']?[A-Za-z0-9._~+/=@-]{16,}`), "${1}[REDACTED:yaml-secret]"},
}

// Redact substitui ocorrências de padrões conhecidos de segredo por um
// marcador, e devolve os nomes dos padrões encontrados (vazio se nenhum).
func Redact(text string) (redacted string, found []string) {
	redacted = text
	seen := map[string]bool{}
	for _, p := range patterns {
		if p.re.MatchString(redacted) {
			repl := p.repl
			if repl == "" {
				repl = "[REDACTED:" + p.name + "]"
			}
			redacted = p.re.ReplaceAllString(redacted, repl)
			if !seen[p.name] {
				seen[p.name] = true
				found = append(found, p.name)
			}
		}
	}
	return redacted, found
}
