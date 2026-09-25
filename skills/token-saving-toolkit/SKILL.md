---
name: token-saving-toolkit
description: "Ferramentas de alta performance (repo-map, ast-outline, trace-strip) para redução drástica de tokens e navegação estrutural."
---

# Token-Saving Toolkit & Data Guardians

Esta skill instrui o agente a utilizar utilitários nativos de alto desempenho instalados em `~/.local/bin` para filtrar contexto desnecessário e preservar tokens.

## Utilitários Disponíveis

### 1. `repo-map`
Mapa estrutural e relacional do repositório com cache incremental determinístico (<20ms):
- **Quando usar:** Ao iniciar tarefas, investigar arquitetura ou descobrir dependências sem despejar arquivos inteiros no contexto.
- **Suporte multi-linguagem nativo:** Go (`.go`), TypeScript/JavaScript (`.ts`, `.tsx`, `.js`, `.jsx`), PHP (`.php`) e Python (`.py`).
- **Comandos principais:**
  - `repo-map --focus <caminho/arquivo>`: Subgrafo textual em torno do arquivo (símbolos declarados, imports e chamadas/importers).
  - `repo-map --summary`: Top hubs estruturais do projeto (símbolos mais referenciados no repositório).
  - `repo-map --update`: Re-indexa silenciosamente apenas os arquivos modificados (delta).

### 2. `ast-outline`
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

1. **Orientação:** Use `repo-map --focus <arquivo>` ou `repo-map --summary` para entender a vizinhança e dependências sem ler múltiplos arquivos.
2. **Mapeamento:** Use `ast-outline <arquivo>` para descobrir em que linhas a função desejada está.
3. **Foco:** Use `view_file` especificando `StartLine` e `EndLine` no trecho identificado.
4. **Debug:** Passe a saída de erros pelo `trace-strip` para persistir no contexto apenas a causa raiz e o stack trace útil.
5. **Revisão:** Use `git-diff-summary` para validar o impacto das mudanças de código sem poluir o histórico.
