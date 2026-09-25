# Playbook K — Reabertura da pendência de nudge de tokens (Cursor + Antigravity)

**Quando reabrir?** Quando `ctx-window track-docs` retornar **exit 1** (positive match) OU quando revisão manual das URLs abaixo mostrar que tokens aparecem em payload de hook.

**Quando NÃO reabrir?** Por heurística local (parsear transcripts, inferir tokens) — esta regra foi gravada em [`memory-mcp`/`gap-closure-policy`] e está refletida em [Issue #2](https://github.com/matheusbbdutra/agent-sync/issues/2).

## 1. Confirmação manual (sempre, antes de implementar)

Antes de escrever qualquer código, confirme a mudança oficial:

1. Rode `ctx-window track-docs`. Se exit 1, vá para o passo 2.
2. Abra o snapshot Markdown apontado no stdout (em `~/.cache/agent-sync/docs-snapshots/`).
3. Para Cursor: confirme o feature request oficial [forum.cursor.com/t/cursor-hooks-token-usage-support/147216](https://forum.cursor.com/t/cursor-hooks-token-usage-support/147216) — procure por posts de **deanrie** (employee Cursor) marcando como entregue/shipped/implemented.
4. Para Antigravity CLI: confirme a doc [antigravity.google/docs/hooks?tab=cli](https://antigravity.google/docs/hooks?tab=cli) listando explicitamente tokens no schema de `PostToolUse` ou similar.
5. Documente em commit o link exato da fonte (não invente — copie da página).

Se nenhuma confirmação oficial existir mas o track-docs matchou, é **falso positivo** — reporte como issue e refine a regex em `track-docs/main.go`.

## 2. Implementação (espelho de `codex_usage.go`)

Esta seção assume Cursor (Antigravity segue o mesmo padrão).

### 2.1 Criar `tools/cmd/ctx-window/cursor_usage.go`

Template (substituir `<CLI>` por `cursor` ou `antigravity`):

```go
package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type <CLI>Usage struct {
	TokensInput  int
	TokensOutput int
	Total        int
}

// latest<CLI>UsageOnce reads the most recent hook payload written by <CLI>'s
// session storage and returns it. Returns (nil, false) if not available.
// CRITICAL: only consume the latest, never aggregate across sessions — the
// nudge is per-session, not per-account.
func latest<CLI>UsageOnce() (*<CLI>Usage, bool) {
	dir := <CLI>SessionDir()
	matches, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil || len(matches) == 0 {
		return nil, false
	}
	sort.Strings(matches)
	latest := matches[len(matches)-1]
	return parse<CLI>UsageFromFile(latest)
}

func <CLI>SessionDir() string {
	home, _ := os.UserHomeDir()
	// Cursor: ~/.cursor/projects/.../agent-transcripts/<uuid>/<uuid>.jsonl
	// Antigravity CLI: ~/.gemini/antigravity-cli/brain/<conversationId>/.system_generated/logs/transcript.jsonl
	return filepath.Join(home, "<CLI-specific-path>")
}

func parse<CLI>UsageFromFile(path string) (*<CLI>Usage, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	// Walk backwards through JSONL until we find a record with usage
	// metadata. (See step 3 for schema-specific parsing.)
	// ...
}

func check<CLI>Nudge(sessionID string) string {
	u, ok := latest<CLI>UsageOnce()
	if !ok {
		return ""
	}
	threshold := nudgeTokensThreshold()
	if u.Total < threshold {
		return ""
	}
	return "<CLI> context tokens approaching threshold; run `ctx-window summarize`."
}

func write<CLI>Nudge(w io.Writer, sessionID string) {
	if msg := check<CLI>Nudge(sessionID); msg != "" {
		_, _ = io.WriteString(w, msg+"\n")
	}
}

// _ ensures imports are used even when adapting the template
var _ = json.Marshal
var _ = strings.TrimSpace
```

### 2.2 Criar `tools/cmd/ctx-window/<cli>_usage_test.go`

Mínimo 2 testes:

```go
func Test<CLI>NudgeUsesLatestUsageOnce(t *testing.T) {
	// Write 3 fake session files with descending timestamps.
	// Set total on the LATEST above threshold; earlier ones below.
	// Call check<CLI>Nudge → assert non-empty.
}

func Test<CLI>NudgeBelowThreshold(t *testing.T) {
	// Write 1 fake session file with total below threshold.
	// Call check<CLI>Nudge → assert empty.
}
```

### 2.3 Integrar em `tools/cmd/ctx-window/main.go`

Adicionar caso no switch:

```go
case "on-tool-call":
    return runOnToolCall(rest, stdout, stderr)
case "on-tool-call-llm":
    return runOnToolCallLLM(rest, stdout, stderr)
// novo:
case "<cli>":
    return run<CLI>Hook(rest, os.Stdin, stdout, stderr)
```

E em `runOnToolCall`, adicionar bloco similar ao de opencode:

```go
if cli := strings.TrimSpace(*cliFlag); cli == "<cli>" {
    if nudge, _ := check<CLI>Nudge(session); nudge != "" {
        fmt.Fprintf(stdout, "[AVISO agent-sync] %s\n", nudge)
        return nil
    }
}
```

### 2.4 Registrar o hook em `cmd/agent-sync/hooks.go`

Já existe `syncCtxCompactHook` — adicionar a CLI nova ao switch. Validar que `~/.cursor/hooks.json` ou `~/.gemini/config/hooks.json` referencia o comando corretamente.

## 3. Smoke

Adicionar entrada em `docs/SMOKE-TEST-J.md`:

```
### <cli> (`<cli> -p ...`)
- Arquivos: `email_validator.py`, `test_email_validator.py`
- Pytest: 3/3 PASS
- Nudge: <verificar que aparece quando total > threshold via hook>
```

Smoke script análogo ao dos outros 4 CLIs em `/tmp/agent-sync-smoke/<cli>/`.

## 4. Fechamento (passos finais)

1. `go test ./...` deve passar.
2. `make build && make install` deve produzir binário atualizado.
3. Commit:
   ```
   feat(agent-sync): implementa nudge de tokens para <CLI>
   
   <CLI> hook payload agora expõe {tokens_input, tokens_output}.
   Implementação análoga a codex_usage.go.
   
   - tools/cmd/ctx-window/<cli>_usage.go (novo)
   - tools/cmd/ctx-window/<cli>_usage_test.go (novo)
   - tools/cmd/ctx-window/main.go (integração)
   - cmd/agent-sync/hooks.go (syncCtxCompactHook)
   - docs/SMOKE-TEST-J.md (smoke entry)
   
   Triggered by: <link do post/issue/PR que confirmou tokens no payload>
   Closes #2
   ```
4. Atualizar `docs/ADR-context-window-strategy.md:105-106` (Cursor/Antigravity): trocar 🔭 por ✅ + linkar o commit.
5. Fechar [issue #2](https://github.com/matheusbbdutra/agent-sync/issues/2) com comentário `Resolved by <commit-sha>`.
6. Atualizar `STATE.md` K de `tracking-only` para `fechado <data>`.

## 5. Critério de "pronto"

- [ ] `ctx-window track-docs` retorna exit 0 novamente (mudança absorvida).
- [ ] `go test ./...` 100% PASS.
- [ ] `make build` sem warnings.
- [ ] Smoke `<cli>` mostra nudge quando sessão estoura threshold.
- [ ] Issue #2 fechada.
- [ ] ADR + STATE.md refletem status novo.

## 6. Anti-objetivos (regras para NÃO fazer)

- ❌ Implementar heurística que parseia transcript JSONL para inferir tokens — caiu nessa armadilha uma vez (issue #2), e o próprio time Cursor documentou publicamente que o workaround não é confiável.
- ❌ Auto-reabrir a issue #2 quando track-docs der match — sempre revisão humana antes.
- ❌ Copiar o caminho do transcript.jsonl achado em Antigravity CLI sem validar — o schema mudou pelo menos uma vez (jun 2026) e pode mudar de novo.
- ❌ Adicionar token tracking em hook que ainda não documenta o campo — bloqueia a si mesmo se a CLI remove o campo sem aviso.
