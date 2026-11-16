---
name: object-calisthenics
description: "Use when applying the 9 Object Calisthenics rules by Jeff Bay to guide cohesive code without nesting."
---

# Object Calisthenics

A set of 9 constraints to exercise good object-oriented design. They are an **exercise**, not a dogma: apply them to reveal design problems and negotiate relaxing them where they make no sense (pragmatism beats the rule).

## Use this skill when

- Refactoring long methods, with many levels of indentation or cascading `else`.
- Fighting the anemic model (classes with only getters/setters and logic in the services).
- Reducing coupling and improving testability.
- Reviewing new code before sending it to review.

## Do not use this skill when

- The snippet is a pure and trivial function, with no domain state.
- The constraint would increase complexity with no real gain (e.g.: wrappers with no behavior).
- The team has no tests that guarantee the refactor.

## Instructions

1. Choose the rule that attacks the current pain (e.g.: start with "no `else`" and "one level of indentation").
2. Apply one rule at a time, keeping the tests green (refactor in small slices).
3. Extract methods/objects with intentional domain names.
4. Consciously relax when the rule conflicts with clarity or performance.

## The 9 rules

### 1. Only one level of indentation per method

- **Why:** it forces extraction of methods with a single responsibility.
- **Before:**
  ```python
  def calcular(pedido):
      total = 0
      for item in pedido.itens:
          if item.ativo:
              total += item.preco
      return total
  ```
- **After:** extract `total_de_ativos(itens)` and the loop stays in a method of its own.
- **How to apply:** guard clauses for validation; extract the loop/condition into a named method.

### 2. Do not use `else`

- **Why:** it reduces branches and improves the linear flow.
- **Before:** `if x: return a else: return b`
- **After:** `if x: return a` followed by `return b`.
- **Patterns:** early return, polymorphism/Strategy, default value.

### 3. Wrap primitives and strings

- **Why:** it gives type, validation and behavior to the value (Value Object).
- **Example:** `Dinheiro`, `Email`, `CPF`, `Periodo` instead of `float`/`str`.
- **Gain:** centralized invariants and a self-explanatory signature.

### 4. One dot per line (Law of Demeter)

- **Why:** it avoids coupling to the internal structures of other objects.
- **Before:** `pedido.cliente.endereco.cidade.uf`
- **After:** `pedido.uf_de_entrega()` (the owner of the information answers).
- **Rule of thumb:** only call methods of the object itself, of parameters or of locally created objects.

### 5. No abbreviations

- **Why:** names are documentation; abbreviations create ambiguity.
- **Avoid:** `usr`, `calc`, `qtd`, `tp`.
- **Prefer:** `usuario`, `calcular`, `quantidade`, `tipo`.

### 6. Keep entities small

- **Target:** rules ≤ ~50 lines, files ≤ ~100 lines, packages ≤ ~10 files.
- **Use:** a warning sign to split; it is not a blind goal.

### 7. At most two instance variables per class

- **Why:** classes with many fields usually have multiple responsibilities.
- **How to apply:** group cohesive fields into Value Objects (e.g.: `Endereco` instead of street/nº/city/zip).
- **Careful:** persistence entities sometimes justify exceptions.

### 8. Use first-class collections

- **Why:** it encapsulates operations over collections and avoids repeated loops.
- **Before:** `List<Item>` spread around with `for` summing, filtering, counting.
- **After:** class `Itens` with `total()`, `ativos()`, `quantidade()`.

### 9. Do not use getters/setters (Tell, Don't Ask)

- **Why:** exposing state invites external logic and the anemic model.
- **Before:** `conta.set_saldo(conta.get_saldo() - valor)`
- **After:** `conta.debitar(valor)` (behavior in the owner of the data).
- **Exception:** boundary DTOs, serialization and frameworks.

## Application checklist

- [ ] Does each method have at most one level of indentation?
- [ ] Are there no unnecessary `else`?
- [ ] Are domain primitives in Value Objects?
- [ ] Does each line make at most one "dot"?
- [ ] Names without abbreviations and revealing intent?
- [ ] Classes and files within the target size?
- [ ] Is behavior in the object that owns the data (no getters/setters)?

Rules are diagnostic tools: if applying them worsens clarity, **document the exception** and move on.

## References

- [code-refactoring-refactor-clean](reference/code-refactoring-refactor-clean.md) — Refactor plan, code smells and incremental slices.
- [refactoring-catalog](reference/refactoring-catalog.md) — catalog of named refactorings, `smell → move` map, Go-specific moves, when **not** to refactor and the safety protocol.
