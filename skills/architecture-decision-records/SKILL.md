---
name: architecture-decision-records
description: "Use when documenting significant architectural choices (lib selection, schema, communication patterns). Criação e manutenção de Architecture Decision Records (ADRs) estruturados."
---

# Architecture Decision Records

Padrões enxutos para registrar contexto, opções e consequências de decisões técnicas estruturantes.

## Use this skill when

- Documentando decisões arquiteturais, escolha de libs, padrões de comunicação ou schemas.
- Registrando trade-offs técnicos significativos e regras de evolução de contratos.

## Do not use this skill when

- Mudança é detalhe interno de implementação, rotina de manutenção ou bug fix.
- Decisão já está coberta por regra global ou convenção existente do repositório.

## Convenções neste repositório (`agent-sync`)

1. **Localização e Nomes:**
   - Documentos em `docs/ADR-<topico>.md` (ex: `docs/ADR-001-schema-output.md` ou `docs/ADR-session-event-jsonl-append-only.md`).
   - Sincronização direta com `STATE.md` / `.agent-sync/session-state.json` através de IDs de decisão `D-N` e ações `A-N`.
2. **Ciclo de Vida:**
   - `Proposto` → `Implementado` → `Aceito` (após smoke/validação real) → `Superado` (linkando o substituto).
3. **Princípios de Decisão:**
   - Contratos fechados por padrão (`additionalProperties: false` em JSON Schema). Exceções devem ser justificadas na ADR.
   - Mudanças de arquitetura requerem plano de verificação determinístico (ex: cenários de smoke documentados).

## Template Canônico (MADR Enxuto)

```markdown
# ADR-NNN: [Título Conciso]

## Status
[Proposto | Implementado | Aceito | Superado por ADR-MMM]

## Contexto & Drivers
- O que motivou a decisão? Quais os requisitos e restrições técnicas?
- Problema real e limitações das abordagens atuais.

## Opções Consideradas
- **Opção 1 (Escolhida)**: Prós e contras.
- **Opção 2**: Por que foi descartada.

## Decisão
- O que foi decidido, escopo e tecnologias envolvidas.

## Consequências & Trade-offs
- **Positivas**: Ganhos imediatos e garantias.
- **Negativas / Riscos**: Débitos assumidos ou complexidade adicional.
- **Mitigação**: Como os riscos serão contornados.

## Critérios de Aceite / Smoke
- Condições práticas para mover o status de `Implementado` para `Aceito`.
```

## Anti-patterns

- ADR genérica sem impacto prático ou sem vínculo com código/tarefas (`A-N`).
- Omitir trade-offs ou consequências negativas da escolha.
- Editar o histórico de uma ADR já aceita sem registrar nova decisão ou marcar como superada.
