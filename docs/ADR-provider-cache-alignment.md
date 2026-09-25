# ADR: Alinhamento de Cache de Provedores e Compactação Segura (CacheAligner)

- **Status**: Aceito (implementado em 2026-09-23 via A-40 / D-55)
- **Data**: 2026-09-23
- **Decisor**: Matheus Dutra
- **Tags**: prompt-caching, kv-cache, ctx-window, token-optimization, headroom, performance

---

## Contexto

Técnicas de compactação de contexto e injeção de sumários (`ctx-window-nudge`, `precompact-snapshot`, `STATE.md`) são utilizadas no `agent-sync` para controlar a janela de contexto. Contudo:

1. **Risco de Invalidação de Prefix-Cache:** Os principais provedores de modelos de linguagem (Anthropic Claude Prompt Caching, OpenAI Prompt Caching, Google Gemini Context Caching) utilizam caches baseados no prefixo exato dos tokens enviados.
2. **Impacto Econômico e de Latência:** Modificar trechos iniciais do prompt ou reescrever continuamente blocos do início da conversa a cada turno **invalida 100% do cache de prefixo**, tornando as requisições até 10x mais caras e sensivelmente mais lentas do que se nenhum texto tivesse sido compactado.
3. **Padrão validado no `headroomlabs-ai/headroom`:** O conceito de `CacheAligner` monitora blocos estáveis vs. blocos voláteis, garantindo que o cabeçalho estático (system rules, definições de ferramentas e contexto base) permaneça inalterado para manter o hit de cache no provedor.

---

## Decisão

Adotar o princípio de **Cache Alignment** em todas as transformações de contexto e compactações no `agent-sync`:

### 1. Separação Rígida de Blocos Estáticos vs. Voláteis
- **Bloco Estático (Prefix Cacheable):**
  - Regras globais (`rules/global-rules.md`).
  - Definições canônicas de agentes e ferramentas disponíveis.
  - Snapshot de arquitetura imutável durante o turno.
  Este bloco deve ser idêntico byte a byte em todas as mensagens consecutivas.
- **Bloco Volátil (Append / Tail Only):**
  - Histórico de tool calls da sessão atual.
  - Atualizações de estado incremental e nudges.
  - Notas de fim de turno (`followup_message`).

### 2. Política de No-Op Rewrite em Injeções
- Nenhum hook deve reordenar ou reformatar blocos de contexto existentes se o conteúdo semântico for o mesmo.
- Injeções de sumário (`STATE.md` compactado) devem ser anexadas no final do contexto como checkpoint de transição, em vez de reescrever retroativamente o histórico já cacheado pelo provedor.

### 3. Matriz de Cobertura Cross-CLI (5xN)

| CLI | Estado | Mecanismo de Integração |
|---|---|---|
| **Claude Code** | ✅ Coberto | Respeito aos breakpoints de cache do Claude (`type: ephemeral`) sem modificar prefixes estáveis. |
| **OpenCode v2** | ✅ Coberto | Plugin `ctx.session.hook('context')` injeta notas apenas no final (`tail`) da pilha de contexto. |
| **Codex** | ✅ Coberto | Manutenção de cabeçalhos de sistema imutáveis no wrapper de execução. |
| **Antigravity** | ✅ Coberto | Preservação do cache do Gemini CLI / SDK através de prefixos determinísticos fixos. |
| **Cursor** | 🟡 Contornável | MDC rules e prompts locais mantidos com hash constante durante a sessão. |

---

## Consequências

**Positivas:**
- Aproveitamento máximo dos descontos de Prompt Caching (até 90% de desconto nos tokens de entrada em Anthropic e OpenAI).
- Menor tempo de primeira resposta (Time to First Token) devido ao cache hit.

**Negativas / Trade-offs:**
- Restrição no design de plugins e hooks: não é permitido "reescrever" o passado do chat; mudanças estruturais precisam ser tratadas como novos checkpoints.
