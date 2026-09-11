---
name: object-calisthenics
description: As 9 regras de Object Calisthenics de Jeff Bay para forçar design orientado a objetos limpo (um nível de indentação, sem else, sem getters/setters, coleções de primeira classe, nomes sem abreviação, classes pequenas). Use ao refatorar métodos longos, condicionais aninhadas, classes com dezenas de campos ou modelos anêmicos.
---

# Object Calisthenics

Conjunto de 9 restrições para exercitar bom design orientado a objetos. São um **exercício**, não um dogma: aplique-as para revelar problemas de design e negociadamente relaxe onde não fizer sentido (o pragmatismo vence a regra).

## Use this skill when

- Refatorar métodos longos, com muitos níveis de indentação ou `else` em cascata.
- Combater modelo anêmico (classes só com getters/setters e lógica nos serviços).
- Reduzir acoplamento e melhorar testabilidade.
- Revisar código novo antes de enviar para revisão.

## Do not use this skill when

- O trecho é uma função pura e trivial, sem estado de domínio.
- A restrição aumentaria a complexidade sem ganho real (ex.: wrappers sem comportamento).
- O time não tem testes que garantam o refactor.

## Instructions

1. Escolha a regra que ataca a dor atual (ex.: começar por "sem `else`" e "um nível de indentação").
2. Aplique uma regra por vez, mantendo os testes verdes (refactor em fatias pequenas).
3. Extraia métodos/objetos com nomes de domínio intencionais.
4. Relaxe conscientemente quando a regra conflitar com clareza ou performance.

## As 9 regras

### 1. Apenas um nível de indentação por método

- **Por quê:** força extração de métodos com responsabilidade única.
- **Antes:**
  ```python
  def calcular(pedido):
      total = 0
      for item in pedido.itens:
          if item.ativo:
              total += item.preco
      return total
  ```
- **Depois:** extraia `total_de_ativos(itens)` e o laço fica em um método próprio.
- **Como aplicar:** guard clauses para validação; extraia loop/condição para um método nomeado.

### 2. Não use `else`

- **Por quê:** reduz ramos e melhora o fluxo linear.
- **Antes:** `if x: return a else: return b`
- **Depois:** `if x: return a` seguido de `return b`.
- **Padrões:** early return, polimorfismo/Strategy, valor default.

### 3. Envolva primitivos e strings

- **Por quê:** dá tipo, validação e comportamento ao valor (Value Object).
- **Exemplo:** `Dinheiro`, `Email`, `CPF`, `Periodo` em vez de `float`/`str`.
- **Ganho:** invariantes centralizadas e assinatura autoexplicativa.

### 4. Um ponto por linha (Lei de Demeter)

- **Por quê:** evita acoplamento a estruturas internas de outros objetos.
- **Antes:** `pedido.cliente.endereco.cidade.uf`
- **Depois:** `pedido.uf_de_entrega()` (o dono da informação responde).
- **Regra prática:** só chame métodos do próprio objeto, de parâmetros ou de objetos criados localmente.

### 5. Não abreviações

- **Por quê:** nomes são documentação; abreviações geram ambiguidade.
- **Evite:** `usr`, `calc`, `qtd`, `tp`.
- **Prefira:** `usuario`, `calcular`, `quantidade`, `tipo`.

### 6. Mantenha entidades pequenas

- **Alvo:** regras ≤ ~50 linhas, arquivos ≤ ~100 linhas, pacotes ≤ ~10 arquivos.
- **Uso:** sinal de alerta para dividir; não é meta cega.

### 7. No máximo duas variáveis de instância por classe

- **Por quê:** classes com muitos campos costumam ter múltiplas responsabilidades.
- **Como aplicar:** agrupe campos coesos em Value Objects (ex.: `Endereco` em vez de rua/nº/cidade/cep).
- **Cuidado:** entidades de persistência às vezes justificam exceções.

### 8. Use coleções de primeira classe

- **Por quê:** encapsula operações sobre coleções e evita laços repetidos.
- **Antes:** `List<Item>` espalhado com `for` somando, filtrando, contando.
- **Depois:** classe `Itens` com `total()`, `ativos()`, `quantidade()`.

### 9. Não use getters/setters (Tell, Don't Ask)

- **Por quê:** expor estado convida a lógica externa e a modelo anêmico.
- **Antes:** `conta.set_saldo(conta.get_saldo() - valor)`
- **Depois:** `conta.debitar(valor)` (comportamento no dono do dado).
- **Exceção:** DTOs de fronteira, serialização e frameworks.

## Checklist de aplicação

- [ ] Cada método tem no máximo um nível de indentação?
- [ ] Não há `else` desnecessário?
- [ ] Primitivos de domínio estão em Value Objects?
- [ ] Cada linha faz no máximo um "ponto"?
- [ ] Nomes sem abreviações e revelando intenção?
- [ ] Classes e arquivos dentro do tamanho-alvo?
- [ ] Comportamento está no objeto dono do dado (sem getters/setters)?

Regras são ferramentas de diagnóstico: se aplicá-las piorar a clareza, **documente a exceção** e siga em frente.
