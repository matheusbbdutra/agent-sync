---
name: token-saving-toolkit
description: "Ferramentas de alta performance (ast-outline, trace-strip) para redução drástica de tokens."
---

# Token-Saving Toolkit & Data Guardians

Esta skill instrui o agente a utilizar utilitários nativos de alto desempenho instalados em `~/.local/bin` para filtrar contexto desnecessário e preservar tokens.

## Utilitários Disponíveis

### 1. `ast-outline`
Gera o esqueleto do arquivo (classes, métodos, interfaces, funções com números de linha) em vez de ler o arquivo inteiro.
- **Quando usar:** Sempre antes de ler arquivos com mais de 100 linhas.
- **Comando:** `ast-outline <caminho_do_arquivo>`
- **Suporte nativo:** Go (via AST nativo), Python, TypeScript, JavaScript, PHP e fallback genérico.
- **Economia:** Reduz de ~2.500 tokens para ~150 tokens por inspeção.

### 2. `trace-strip`
Filtra stack traces e logs extensos, ocultando frames internos de frameworks, runtime e vendors (`node_modules`, `vendor/`, `site-packages`, Spring/Jakarta, etc.), mantendo apenas o ponto exato da falha na aplicação.
- **Quando usar:** Ao analisar falhas, logs de testes ou exceções em terminal.
- **Comando:** `trace-strip [arquivo_ou_pipe]`
- **Exemplo:** `cat error.log | trace-strip` ou `trace-strip error.log`

### 3. `db-guardian`
Guardião de consultas a banco de dados e APIs:
- Bloqueia mutações destrutivas (`INSERT`, `UPDATE`, `DELETE`, `DROP`, `ALTER`, etc.) a menos que expressamente autorizadas.
- Adiciona limites defensivos (`LIMIT 20`) automaticamente se a query não tiver paginação.
- Alerta contra `SELECT *`.
### 4. `git-diff-summary`
Resume diffs unificados do Git, extraindo arquivos modificados, adicionados ou deletados com status `[M]`/`[A]`/`[D]`, cabeçalho da função alterada e contagem de linhas adicionadas/removidas (`+`/`-`), evitando despejar diffs brutos gigantes no contexto.
- **Quando usar:** Ao inspecionar mudanças antes de commits ou em revisões de código.
- **Comando:** `git-diff-summary [arquivo_ou_pipe]` (ou sem argumentos para inspecionar o `git diff` atual).
- **Exemplo:** `git-diff-summary` ou `git diff main | git-diff-summary`

## Fluxo Recomendado de Resolução com Baixo Consumo de Tokens

1. **Mapeamento:** Use `ast-outline <arquivo>` para descobrir em que linhas a função desejada está.
2. **Foco:** Use `view_file` especificando `StartLine` e `EndLine` no trecho identificado.
3. **Debug:** Passe a saída de erros pelo `trace-strip` para persistir no contexto apenas a causa raiz e o stack trace útil.
4. **Revisão:** Use `git-diff-summary` para validar o impacto das mudanças de código sem poluir o histórico.
