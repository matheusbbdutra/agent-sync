# ADR: Harness Trace Guard — Verificação Causal e Evidência de Transição de Estado em Fim de Turno

**Status**: Aceito (Fases 1-2; Trilha A Cursor `stop` validada 2026-09-19)  
**Data**: 2026-09-18  
**Validação runtime**: 2026-09-19 — smoke `docs/SMOKE-TEST-R.md`, evidência `docs/smoke-evidence/trilha-a-cursor-stop.md` (C1 bloqueia / C2 passa; fix `Shell`→`RanTestCommand`)  

**Decisor**: Matheus Dutra  
**Fundamentação Teórica**: arXiv:2606.06324v2 (*HarnessFix: Diagnosing and Repairing Harness Flaws via HTIR*)  
**Tags**: ai-agent, harness, verification, lifecycle, false-success-guard, observability  

---

## 1. Contexto & Motivação

Atualmente, o `agent-sync` possui o binário `false-success-guard` (`tools/cmd/false-success-guard/`) registrado no hook de fim de turno (`Stop` no Claude Code e Antigravity `agent-stop`).

### Limitações do Modelo Atual:
1. **Detecção Puramente Léxica**: O classificador atual baseia-se em expressões regulares (`successAssertionPattern` vs `evidencePattern`). Se o modelo cita superficialmente um comando (ex: ``fiz os ajustes e rodei `go test` ``), o guarda assume como evidência anexada, mesmo que:
   - Nenhum comando tenha sido executado de fato no turno;
   - O comando tenha falhado (exit code != 0);
   - Nenhuma mutação de arquivo ou estado tenha ocorrido no ambiente.
2. **Custo Econômico do "Ghost Work"**: Quando o agente declara falsamente a conclusão de uma tarefa de codificação, o usuário gasta tempo conferindo, encontra o erro e cola o log de volta. Esse ciclo de retrabalho injeta de 4.000 a 15.000 tokens adicionais na janela de contexto, acelerando o efeito *Lost in the Middle* e degradando a acurácia do modelo nos turnos seguintes.
3. **Evidência do HarnessFix (arXiv:2606.06324v2)**: Mais de 80% das falhas de agentes em benchmarks práticos (SWE-Bench, AppWorld, Terminal-Bench) concentram-se nas camadas de **Lifecycle**, **Tooling** e **Verification**. Agentes finalizam tarefas prematuramente porque o harness aceita status `success` sem verificar se a transição esperada no sistema de arquivos ou no estado externo realmente ocorreu.

---

## 2. Decisão Arquitetural

Evoluir o `false-success-guard` de um classificador puramente léxico para um **Harness Trace Guard** baseado no princípio de **State-Effect Alignment** do HarnessFix:

### 2.1 Inspeção Baseada em TraceSteps
O hook de fim de turno não lerá apenas a prosa do último texto do assistente, mas inspecionará o histórico do turno atual do `transcript_path`:
1. **Verificação de Mutação (*Artifact/State Effect*)**: Se o agente alegar conclusão de código/bugfix, o transcript do turno atual deve conter ao menos 1 invocação de ferramenta de escrita (`write_to_file`, `replace_file_content`, `patch` ou comando shell com mutação).
2. **Verificação de Execução & Saída de Ferramenta (*Tool Exit Code*)**: Se um comando de verificação foi chamado, o status de execução deve ser limpo (`exit code 0`), sem conter marcadores `[TOOL_STATUS: FAILED]` ou erros não tratados.
3. **Decisão Combinada**:
   - **Caso 1 (Léxico + Evidência de Rastreamento)**: Prosa confiante + arquivo alterado + teste verde $\rightarrow$ **Passa silencioso (`{}`)**.
   - **Caso 2 (Falha Declarada Honestamente)**: Agente admite bloqueio ou dúvida $\rightarrow$ **Passa silencioso (`{}`)**.
   - **Caso 3 (Alegação de Conclusão sem Mutação/Verificação)**: Prosa alega sucesso mas o TraceStep não tem efeito observável $\rightarrow$ **Dispara Nudge de Violação de Ciclo de Vida**.

### 2.2 Formato do Feedback Injetado (Advisory Nudge)
Mantendo o princípio de nunca abortar destrutivamente o processo, o hook retorna contexto adicional para que o próprio modelo se autocorrija no próximo ciclo de inferência:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "Stop",
    "additionalContext": "[agent-sync:harness-guard] Alegação de conclusão detectada, mas não há registro de mutação de arquivo ou comando de validação bem-sucedido no turno atual. Verifique o resultado na prática antes de finalizar."
  }
}
```

### 2.3 Contrato de Janelamento por Turno
O **turno atual** é definido como a janela entre a última entrada `role=user` que contém um bloco `content.type=text` e o `Stop` final do hook.

**Mecanismo**: `inspectTranscript` (`tools/cmd/false-success-guard/hook.go`) mantém um acumulador `ExecutionEvidence` enquanto percorre o JSONL do transcript. Ao encontrar um bloco `type=text` dentro de uma entrada `role=user`, o acumulador é zerado (`ev = ExecutionEvidence{HasTraceData: false}`). Blocos `tool_result` isolados não disparam reset — eles apenas alimentam o acumulador do turno atual.

**Edge case conhecido**: uma entrada `role=user` com `content=[text, tool_result]` misturados dispara o reset pelo bloco `text`. O `tool_result` que aparece na mesma entrada é descartado do turno atual — porque o loop processa todos os blocos antes de checar `hasUserText`, e o reset zera o estado após o loop. Esse comportamento é desejado: tool_results entregues junto com um prompt textual do usuário pertencem ao turno anterior, não ao novo turno.

**Justificativa**: optou-se por reset-por-texto em vez de janela deslizante de N tool calls pelo menor custo cognitivo de explicar e porque satisfaz diretamente o caso de uso do bug original (erros antigos contaminando o veredito do turno atual). N tool calls exigiria um parâmetro mágico e ainda falharia em cenários onde o usuário reage com texto entre execuções.

---

## 3. Plano de Execução em Fases

### Fase 1 — Extrator de TraceSteps no Módulo Go (`tools/cmd/false-success-guard/`)
- [x] Adicionar parser de blocos de `tool_use` e `tool_result` no JSONL de transcripts (`Claude Code`, `Antigravity`, `Cursor`).
- [x] Criar struct de evidência (`tools/cmd/false-success-guard/detector.go`):
  ```go
  type ExecutionEvidence struct {
      HasTraceData   bool
      HasMutation    bool // Tool calls like write/edit/patch/file creation
      RanTestCommand bool // Tool calls executing tests or verification commands
      HasToolError   bool // Unhandled errors, non-zero exit codes or [TOOL_STATUS: FAILED]
  }
  ```
- [x] Integrar no `detector.go`: `ClassifyWithTrace(text string, ev ExecutionEvidence) Verdict`.
- [x] Testes unitários com casos reais extraídos de transcripts com erros silenciosos.

### Fase 2 — Cobertura Unificada nas CLIs via Hooks Existentes
- [x] **Claude Code**: Atualizar handler de `Stop` (`tools/cmd/false-success-guard/hook.go`).
- [x] **Google Antigravity**: Conectar no hook `hooks/agent-stop.antigravity.sh` passando o `transcript.jsonl` da sessão.
- [x] **Cursor**: Integrar ao hook `stop` em `hooks/agent-stop.cursor.sh`.
  Validado em runtime 2026-09-19 (`docs/smoke-evidence/trilha-a-cursor-stop.md`):
  `RanTestCommand` reconhece tool name `Shell` (Cursor) além de `bash`/`test`/`command`.

### Fase 3 — Validação Prática & Benchmark Interno
- [x] Rodar `go test ./...` no módulo `tools` (todos os pacotes, incluindo `false-success-guard` e `cmd/agent-sync`).
- [ ] Medir consumo de tokens em tarefas com e sem o hook ativo (avaliar se o nudge imediato previne ciclos longos de retrabalho).

---

## 4. Consequências & Trade-offs

### Positivas
- **Eliminação de "Ghost Successes"**: O agente deixa de fingir que corrigiu códigos ou rodou testes sem tê-lo feito.
- **Redução de Falsos Positivos**: Menos alertas indevidos quando o agente realmente executou os testes e alterou arquivos.
- **Economia Líquida de Contexto**: Gasta ~40 tokens em um nudge pontual para evitar desperdício de milhares de tokens em retrabalho investigativo.

### Negativas / Riscos Mitigados
- **Leitura Adicional de I/O**: Ler os últimos blocos do JSONL do transcript adiciona ~2 a 5ms no hook em Go (desprezível frente ao tempo de rede/LLM).
- **Risco de Loop de Nudge**: Se o modelo continuar insistindo no texto sem rodar ferramentas, o limite de loops da CLI (`loop_limit` no Cursor / contador interno) previne travamento.
