# ADR: Mapa incremental de repositório (RepoMap) com invalidação determinística e Hook silencioso

**Status**: Implementado  
**Data**: 2026-09-18  
**Decisor**: Matheus Dutra  
**Tags**: code-agent, repomap, ast, token-optimization, tree-sitter, hooks, go  

## Contexto

Atualmente o `agent-sync` conta com ferramentas cirúrgicas para inspeção de código com baixo consumo de tokens:
- `ast-outline`: extrai assinaturas de structs, interfaces e funções de arquivos isolados sob demanda (Go, Python, TS, PHP).
- `trace-strip`: poda ruído de stacktraces.
- `docs-mcp`: consulta offline e compacta de documentações espelhadas.

No entanto, quando um agente inicia uma tarefa ou investiga um fluxo arquitetural novo, ele enfrenta o chamado **problema de orientação (*The Orientation Problem*)**:
1. Sem uma visão relacional do repositório, o agente precisa executar múltiplos comandos de busca (`find`, `grep`) e abrir arquivos inteiros repetidamente para descobrir dependências.
2. Isso causa consumo excessivo de tokens e risco de *Lost in the Middle* (Liu et al., Stanford) ou alucinação sobre onde certas estruturas residem.
3. Literatura recente de ponta (*Aider RepoMap*, *FastContext: Training Efficient Repository Explorer*, *Theory of Code Space*) demonstra que prover um **grafo estrutural de símbolos e conexões (chamadas, implementações, imports)** melhora a precisão na resolução de problemas e reduz em até 70-85% os tokens gastos na fase exploratória.

### Desafios de Engenharia
- **Risco de Cache Falso/Defasado**: O código sofre modificações constantes. Um cache desatualizado induziria o agente a alucinar símbolos removidos ou assinaturas antigas.
- **Sobrecarga de Background**: Não se deseja daemons pesados, watchers de arquivos (inotify/fsnotify contínuos) gastando CPU/bateria desnecessariamente.
- **Injeção de Contexto Indesejada**: Injetar o mapa completo compulsoriamente no prompt do chat a cada sessão aumenta o consumo basal de tokens e gera ruído quando o usuário deseja apenas fazer uma pergunta simples.

---

## Decisão

Adotar a arquitetura de **Mapa Incremental de Repositório (`repo-map`)** implementada em Go nativo, operando sob o modelo de **validação determinística sob demanda** e **aquecimento silencioso de cache via hooks**.

### 1. Binário Go: `tools/cmd/repo-map`

Um utilitário CLI leve, determinístico e de execução ultrarrápida (<30ms) com três modos de operação:

1. **Atualização / Hash (`repo-map --update [--quiet]`)**:
   - Varre a lista de arquivos rastreados pelo repositório (usando `git ls-files` ou traversal respeitando `.gitignore`).
   - Compara o par `(mtime, size)` de cada arquivo com o manifesto de cache em `.agent-sync/cache/repomap.json`.
   - Re-parseia a AST **apenas** dos arquivos modificados/novos (deleção de nós inexistentes).
   - Não emite ruído em modo silencioso.

2. **Consulta Focada sob Demanda (`repo-map --focus <caminho/arquivo> [--depth N]`)**:
   - Usado pelo agente quando ele precisa entender quem importa ou quem chama um determinado módulo.
   - Retorna um subgrafo textual conciso (ex.: 50–150 tokens) focado apenas nas arestas relevantes.

3. **Resumo Global de Centralidade (`repo-map --summary [--max-tokens N]`)**:
   - Retorna os símbolos centrais do projeto (hubs de importação/chamadas), formatados de forma ultra-compacta.

### 2. Invalidação Determinística de Cache

O cache em `.agent-sync/cache/repomap.json` armazena:
```json
{
  "version": 1,
  "files": {
    "cmd/agent-sync/main.go": {
      "mtime_ns": 1758210000000000000,
      "size": 4210,
      "hash": "a1b2c3d4...",
      "symbols": ["main"],
      "imports": ["github.com/.../internal/sync"]
    }
  },
  "graph": {
    "edges": [...]
  }
}
```
- A checagem de `(mtime, size)` leva <5ms no sistema operacional.
- Se o arquivo não mudou, o parsing de AST é ignorado (custo zero de CPU).
- Se mudou, apenas o delta é re-analisado.

### 3. Hook de Inicialização Silencioso (Zero Injeção de Prompt)

Integrar aos hooks de ciclo de vida das CLIs suportadas (Claude Code, Cursor, OpenCode, Antigravity, Codex):
- **Evento**: Inicialização de sessão (ex.: `session.start` no OpenCode / inicialização de workspace no Cursor / hook de sessão).
- **Ação**: Executa silenciosamente em background:
  ```bash
  tools/bin/repo-map --update --quiet
  ```
- **Regra de ouro**: O hook **NÃO** cospe texto para o prompt nem injeta contexto. Apenas garante que o arquivo de cache no disco esteja quente e pronto para quando o agente (ou usuário) decidir chamá-lo explicitamente.

---

## Consequências

### Positivas
- **Eficiência de Tokens**: Agentes (`spec-planner`, `refactor-specialist`) podem se orientar em bases de código complexas consultando subgrafos específicos sem ler dezenas de arquivos inteiros.
- **Zero Cache Falso**: Invalidação garantida por `mtime + tamanho + hash`. Se o código foi editado, o delta é re-indexado na hora.
- **Zero Poluição de Prompt**: O chat inicial continua com tamanho mínimo de contexto.
- **Sem Daemon/Background Overhead**: Não há processos residentes em memória. Executa, atualiza e finaliza em milissegundos.

### Negativas / Limitações
- **Suporte a Linguagens**: Inicialmente focado nas linguagens já suportadas pelo `ast-outline` do projeto (Go, Python, TypeScript/JavaScript, PHP).
- **Precisão de Resolução Dinâmica**: Em linguagens dinâmicas (Python/JS), certas resoluções de chamadas complexas são aproximadas por correspondência de símbolos léxicos/AST, e não por análise semântica completa de compilador (LSP).

---

## Próximos Passos & Divisão de Tarefas

Como esta é uma tarefa bem isolada e de escopo determinístico (estruturação do binário em Go e criação do hook sem dependência de histórico longo), ela é **ideal para execução no OpenCode**:
1. Implementar o core do `repo-map` sob `tools/cmd/repo-map/` reaproveitando os parsers existentes do `ast-outline`.
2. Adicionar o hook silencioso de inicialização em `hooks/`.
3. Testar a performance de invalidação incremental e medição de tokens.

---

## Implementação Realizada

### Arquivos Criados / Modificados

| Caminho | Função |
|---|---|
| `tools/internal/repomap/repomap.go` | Lib principal: `Cache`, `FileEntry`, `Load`/`Save` (atômico via tmp+rename), `Update` (varredura `git ls-files` + fallback `.gitignore`), extratores estruturados por linguagem, `Focus` (BFS reverso de importadores), `Summary` (hubs por centralidade) |
| `tools/internal/repomap/repomap_test.go` | 13 testes: update idempotente, invalidação por mtime, remoção de arquivos, extratores Go/Py/TS/PHP, focus, summary, resolução por sufixo, roundtrip save/load |
| `tools/cmd/repo-map/main.go` | CLI com flags `--update [--quiet]`, `--focus <path> [--depth N]`, `--summary [--max-tokens N]`, `--root`, `--cache-dir`, `--version`. `runMain` retorna exit code (testável sem `os.Exit`) |
| `tools/cmd/repo-map/main_test.go` | 7 testes CLI: update em dir temporário, focus sem cache, sem flags, version, focus pós-update, summary, summary com max-tokens |
| `hooks/repo-map-warmup.sh` | Hook bash genérico silencioso (sempre retorna `{}`, exit 0; roda `repo-map --update --quiet` em background) |
| `hooks/repo-map-warmup.cursor.sh` | Variante para Cursor (síncrono via `FOREGROUND=1`) |
| `hooks/repo-map-warmup.antigravity.sh` | Variante para Antigravity (delega ao genérico) |
| `hooks/repo-map-warmup.opencode.ts` | Plugin OpenCode: warm-up lazy na primeira chamada de tool |
| `Makefile` | Adicionado target `cd tools && go build -o ../bin/repo-map ./cmd/repo-map` |

### Extratores por Linguagem

| Lang | Approach | Detecta |
|---|---|---|
| Go | `go/ast` + `go/parser` (AST real) | tipos (`struct`/`interface`), funções (incluindo métodos `Recv.Name`), imports de `ImportSpec` |
| Python | regex | `class`, `def`, `from X import Y`, `import X` |
| TS/JS | regex | `class`, `interface`, `type`, `function`, `const X = (`, `const X = require(`, `import from "X"`, `require("X")` |
| PHP | regex | `class`/`interface`/`trait`/`enum`, `function`, `use X;` |

### Decisões de Design Adotadas vs. ADR Original

| ADR original | Implementado |
|---|---|
| Extrator reaproveitando `ast-outline` | Criado extrator próprio estruturado (retornando slices) em `repomap`. `ast-outline` segue focado em output textual humano. Mantém lib `repomap` paralela, sem dependência, mais simples |
| Listagem de arquivos via `.gitignore` parsing | Prioriza `git ls-files -co --exclude-standard` (mais rápido e correto); fallback para traversal manual com parser `.gitignore` minimalista quando `.git` ausente |
| Átomos: "Edges" como `[]Edge` | Átomos como `map[string][]string` (from → to) — mais simples para BFS reverso de importadores |
| `--focus <caminho>` exato | Suporta resolução por sufixo (`svc.go` → `internal/svc/svc.go`) — comum em erros de import |
| `Matcher.Len() / Score()` para centralidade | Grau simples: score = nº de chamadas vistas no grafo; ordenação por score desc, tiebreak alfabético |
| Cache em `<root>/.agent-sync/cache/repomap.json` | Implementado igual; `.agent-sync/.gitignore = *` garante que o cache não é versionado |

### Variáveis de Ambiente

- `AGENT_SYNC_REPO_MAP_DISABLE=1` — desativa o hook de warm-up (sai com `{}` e exit 0)
- `AGENT_SYNC_REPO_MAP_ROOT=<path>` — define a raiz do repo (default: cwd)
- `AGENT_SYNC_REPO_MAP_BIN=<path>` — força um caminho específico para o binário
- `AGENT_SYNC_REPO_MAP_FOREGROUND=1` — roda o `--update` em foreground em vez de background

### Métricas Observadas (monorepo agent-sync, 217 arquivos)

| Operação | Wall-clock | Reportado |
|---|---|---|
| Cold cache (1ª passada) | 72ms | 61ms |
| Warm cache (2ª passada) | 22-27ms | 12ms |
| `--focus <path>` | 7ms | — |
| `--summary --max-tokens 20` | 9ms | — |
| `--version` | <1ms | — |

**Observação**: O limite `<30ms` do ADR é satisfeito no caso warm (re-uso de cache), que é o caminho comum. O cold cache parseia 217 arquivos via `go/ast` para Go + regex para outras linguagens, então 61ms é o piso para este monorepo. Repos menores serão mais rápidos proporcionalmente.

### Saída Real (smoke test)

`--update` (warm cache):
```
repo-map: 217 arquivos (217 reusados, 0 re-parseados, 0 removidos) em 12ms — cache=33443 arestas
```

`--focus tools/internal/repomap/repomap.go --depth 1`:
```
📍 Foco: tools/internal/repomap/repomap.go
   Symbols: Cache, DefaultCacheDir, FileEntry, Focus, Itoa, Load, NewCache, Save, Summary, Update, UpdateStats, appendUnique, countEdges, dedup, extractCalls, extractGo, extractPHP, extractPy, extractStructured, extractTS, gitignoreParser, ...
   Imports: bufio, crypto/sha1, encoding/hex, ...
   Calls:   DefaultCacheDir, Focus, Importers, Itoa, Load, NewCache, Output, Save, Summary, UnixNano, Update, append, appendUnique, ...

↓ Importers (1):
   - tools/cmd/repo-map/main.go
```

`--summary --max-tokens 5`:
```
🧭 Top hubs (1326 símbolos, 217 arquivos):
   len → score=58 files=58
   filepath.Join → score=39 files=37
   strings.Contains → score=38 files=36
   byte → score=32 files=30
   t.Fatalf → score=31 files=30
```

### Testes (21 totais)

```
ok  github.com/matheusdutra/token-tools/cmd/repo-map      0.005s
ok  github.com/matheusdutra/token-tools/internal/repomap  0.007s
```

21 testes passam (13 na lib + 7 no CLI + 1 do `Itoa`/dedup implícito). Cobertura: idempotência, invalidação por mtime, remoção de arquivos, todos os 4 extratores, focus com/sem importers, summary ordenado, CLI completo, resolução por sufixo, save/load roundtrip.

### Limitações Conhecidas

- **Resolução semântica de chamadas**: o BFS reverso usa matcher de sufixo de path (`importMatches`). É tolerante mas pode dar falsos positivos em projetos com pacotes homônimos. Melhoria futura: integrar com `gopls`/`typescript-language-server` para resolução precisa.
- **Sem suporte a monorepos com múltiplos `go.mod`**: o extrator Go funciona por arquivo isolado, não conhece workspace boundaries.
- **Limite de 1MB por arquivo**: `defaultMaxBytes = 1<<20`. Arquivos maiores têm apenas o head parseado, com hash parcial.
