# ADR: Pipeline de Observação Automática e Persistência de Memória Cross-Session (claude-mem)

- **Status**: Aceito (promovido por D-64 em 2026-09-23; matriz 3/5 wirada — Cursor 🟡/Antigravity 🟡 aceitos como gap documentado)
- **Data**: 2026-09-23
- **Decisor**: Matheus Dutra
- **Tags**: memory-mcp, cross-session, hooks, postToolUse, stop, automated-learning

---

## Contexto

O `agent-sync` já disponibiliza o [`memory-mcp`](file:///home/matheusdutra/Projects/agent-sync/tools/cmd/memory-mcp) e hooks de nudge (`memory-nudge.ts`, `agent-react-nudge.stop.cursor.sh`), além de suporte a Turso Cloud. Entretanto:

1. **A persistência atual é passiva:** Depende de o agente decidir proativamente invocar `store_memory` ou seguir o fluxo do `STATE.md`.
2. **Perda de aprendizados entre sessões:** Decisões de arquitetura tomadas em um turno ou bugs de bibliotecas resolvidos não ficam automaticamente disponíveis quando outra CLI (ex.: Codex ou Antigravity) é aberta.
3. **Padrão validado no `thedotmack/claude-mem`:** O projeto demonstra alta eficiência ao vincular hooks automáticos em momentos-chave:
   - `tool.execute.after`: extração de observações e fatos técnicos.
   - `PreToolUse:Read`: busca proativa de memórias ligadas ao arquivo a ser lido.
   - `Stop / SessionEnd`: compressão e consolidação das observações da sessão para o banco local.

---

## Decisão

Implementar um **Pipeline de Observação Contínua e Injeção de Memória Automática** no `agent-sync`:

### 1. Observação Não-Bloqueante pós-Ferramenta (`postToolUse`)
- Após comandos de bash relevantes (ex.: testes falhando/passando, builds, migrações) ou edições estruturais, o hook registra um log efêmero de evento no buffer da sessão (`.agent-sync/session_buffer.jsonl`).
- Execução em background (`BG=1`), sem adicionar latência perceptível ao turno do agente.

### 2. Consolidação Automática no Fim de Turno (`Stop`)
- No hook de encerramento (`Stop` / `compaction`), disparar rotina de síntese que:
  1. Agrupa os eventos da sessão atual.
  2. Extrai fatos novos verificáveis (erros corrigidos, padrões de rota/banco descobertos).
  3. Grava no `memory-mcp` (SQLite com suporte a FTS5) sob a tag apropriada (`repo-knowledge`, `decision`, `fix`).

### 3. Recuperação de Memória Pré-Leitura (`PreToolUse:Read`)
- Ao ler um arquivo de configuração central, arquivo de rota ou entidade, o hook faz query rápida via SQLite FTS5 por notas existentes sobre o caminho (`path`).
- Injeta no contexto do agente se houver histórico relevante (ex.: "Atenção: migration X deste modelo requer PostgreSQL 16+").

### 4. Matriz de Cobertura Cross-CLI (5xN)

| CLI | Estado | Mecanismo de Integração |
|---|---|---|
| **Claude Code** | ✅ Coberto | Hooks `PostToolUse` (buffer), `PreToolUse:Read` (consulta memória) e `Stop` (consolidação). |
| **OpenCode v2** | ✅ Coberto | Plugins `ctx.tool.hook('execute.after')` e `ctx.session.hook('compaction')` gravando no SQLite do memory-mcp. |
| **Codex** | ✅ Coberto | Hooks de script bash em `postToolUse` e `stop` registrados via `~/.codex/hooks.json`. |
| **Antigravity** | 🟡 Contornável | Hook em `PreInvocation` / `PostToolUse` com injeção via prompt context. |
| **Cursor** | 🟡 Contornável | Hook `stop` já wirado (`agent-stop.cursor.sh`), buffer de observação gravado em background. |

---

## Consequências

**Positivas:**
- Memória verdadeiramente contínua entre sessões e entre ferramentas (o que é corrigido no Claude Code fica visível no Cursor e Antigravity).
- Agente deixa de reincidir em erros de runtime já diagnosticados anteriormente no mesmo projeto.

**Negativas / Trade-offs:**
- Necessidade de política de expiração ou desduplicação de memórias para evitar inchaço do SQLite local.
- Cuidado rigoroso para não gravar credenciais ou dados do `.env` na memória automática.
