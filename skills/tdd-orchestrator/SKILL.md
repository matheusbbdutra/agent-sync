---
name: tdd-orchestrator
description: "Use when implementing features or fixing bugs via Red-Green-Refactor TDD cycles. Ciclo TDD (Red-Green-Refactor) com cobertura de casos de borda e design guiado por testes. discipline, multi-agent workflow coordination, and comprehensive test-driven development practices. Enforces TDD best practices across teams with AI-assisted testing and modern frameworks. Use PROACTIVELY for TDD implementation and governance."
---

## Use this skill when

- Vai escrever/atualizar testes em qualquer módulo deste repo.
- Implementa lógica que ainda não tem cobertura (red-green-refactor).
- Vai refatorar código que já tem testes (regressão safety net).

## Do not use this skill when

- Tarefa é puramente documentação/markdown.
- Já existe cobertura e a mudança é trivial (typo, log, copy).
- Problema é harness/orquestração (use `agent-react` ou `debugging-strategies`).

## Mecânica neste repo

1. **Teste primeiro.** Não escreva implementação sem teste falhando correspondente.
2. **Localização.** Tests no mesmo pacote que o código: `pkg.go` → `pkg_test.go`. Cobertura por comando: `tools/cmd/<cmd>/`.
3. **Comandos.** `go test ./...` para tudo; `go test -run TestX ./pkg/` para alvo; `go test -race ./...` antes de PR com concorrência.
4. **Cobertura.** Sem threshold numérico enforced hoje. Foque em casos de borda e paths de erro — não em %.
5. **Mocks.** Prefira interfaces e `testify/mock` ou mocks manuais. `gomock` raramente necessário.
6. **Tabela-driven.** Quando ≥3 casos de teste similares, use `[]struct{...}{...}` + `t.Run`.

## Anti-patterns (rejeitar antes de PR)

- Teste que faz setup mas nunca assert.
- Teste que depende de ordem de execução.
- Sleep/time.Sleep para sincronizar (use channels ou polling).
- Cobertura alta via testes triviais (smoke de getters).
- Teste que ignora erro (`_, _ = fn()`).
