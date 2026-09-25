---
name: postmortem-writing
description: "Use when writing incident postmortems with root cause, timeline, and corrective actions. Criação de postmortems estruturados e blameless com causa raiz comprovada e ações corretivas."
---

# Postmortem Writing

Guia enxuto para análise pós-incidente, focando em causa raiz comprovada, mitigação e prevenção de reincidência.

## Use this skill when

- Documentando falhas de produção, quebras de contrato ou incidentes graves.
- Investigando bugs complexos com múltiplos fatores contribuintes para evitar reincidência.

## Do not use this skill when

- Incidente em andamento (estabilize e mitigue primeiro).
- Bug simples ou erro de digitação resolvido sem impacto sistêmico.

## Mecânica e Princípios

1. **Cultura Blameless**: Foque nas falhas de sistema, gaps de teste e falta de guards — nunca em indivíduos.
2. **Causa Raiz Comprovada**: Diferencie sintoma de causa raiz. Use a técnica dos "5 Porquês" baseando-se em evidências e logs, nunca suposições.
3. **Ações Corretivas com Dono**: Toda ação deve ter prioridade (P0/P1), responsável e prazo, preferencialmente amarrada a testes automatizados para impedir regressão.

## Template Canônico de Postmortem

```markdown
# Postmortem: [Título do Incidente]

**Data**: YYYY-MM-DD | **Severidade**: [SEV1 | SEV2 | SEV3] | **Duração**: [X min]
**Autores**: [@autor] | **Status**: [Rascunho | Revisado | Concluído]

## Resumo Executivo
- Breve sumário do que quebrou, impacto no sistema/usuários e como foi estabilizado.

## Linha do Tempo (UTC)
- `HH:MM` - Evento desencadeador (deploy, migração, falha externa).
- `HH:MM` - Primeiro alerta / detecção.
- `HH:MM` - Ação de mitigação iniciada.
- `HH:MM` - Sistema estabilizado.

## Análise de Causa Raiz
- **Causa Imediata**: O que falhou diretamente no código/infra.
- **5 Porquês**:
  1. Por que falhou? -> ...
  2. Por que isso ocorreu? -> ...
  3. Por que não foi detectado antes? -> ...
- **Fatores Contribuintes**: Gaps de teste, monitoramento ausente ou premissas incorretas.

## O que Funcionou vs O que Falhou
- **Funcionou**: Alertas rápidos, rollback ágil, logs úteis.
- **Falhou**: Falta de validação prévia, timeout inadequado, falso-positivo.

## Ações Preventivas (Action Items)

| Prioridade | Ação | Responsável | Prazo | Prevenção / Detecção |
|------------|------|-------------|-------|----------------------|
| P0 | Adicionar teste automatizado de regressão | @user | YYYY-MM-DD | Prevenção |
| P1 | Ajustar threshold de monitoramento/guard | @user | YYYY-MM-DD | Detecção |
```

## Anti-patterns

- Atribuir culpa a "erro humano" em vez de ausência de testes e salvaguardas.
- Parar no sintoma (ex: "memória acabou") sem entender o gatilho ("por que houve vazamento").
- Postmortem sem ações práticas de prevenção com prazos definidos.
