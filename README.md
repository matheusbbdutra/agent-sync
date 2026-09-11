# Agent-Sync 🚀

Repositório unificado para versionar, manter e sincronizar **Regras Globais**, **Skills** e **Ferramentas de Baixo Consumo de Tokens** em múltiplos ecossistemas de agentes de IA:
- **Claude Code** (`~/.claude`)
- **Codex / OpenAI** (`~/.codex`)
- **Google Antigravity / Gemini** (`~/.gemini`)
- **OpenCode** (`~/.config/opencode`)

---

## 📦 Estrutura do Repositório

```text
agent-sync/
├── rules/
│   └── global-rules.md     # Regras globais (Clean Code, OWASP, Anti-Alucinação, Data Guardians)
├── skills/
│   ├── token-saving-toolkit/ # Instruções para leitura concisa via AST e poda de logs
│   └── mcp-advisor/          # Avaliação de uso de MCPs
├── tools/                  # Binários utilitários de alta velocidade em Go
│   ├── cmd/ast-outline/    # Extrai classes/métodos em vez de ler arquivos inteiros (Go, Python, TS, PHP)
│   ├── cmd/trace-strip/    # Remove ruídos de frameworks em logs de erro
│   └── cmd/db-guardian/    # Proxy seguro de SQL (Read-Only por default, bloqueia mutações, injeta LIMIT)
├── cmd/agent-sync/         # Orquestrador de sincronização CLI
├── Makefile                # Comandos de automação
└── README.md
```

---

## ⚡ Como Usar em Qualquer Máquina

### 1. Clonar o repositório
```bash
git clone <seu-repo-url> ~/Documentos/agent-sync
cd ~/Documentos/agent-sync
```

### 2. Compilar e Instalar tudo
```bash
make install
```
Isso compilará os binários em Go (`agent-sync`, `ast-outline`, `trace-strip`, `db-guardian`) e os colocará em `~/.local/bin/`.

### 3. Sincronizar com todas as CLIs
```bash
make sync
# ou diretamente:
agent-sync -apply
```

### 4. Verificar Status das CLIs
```bash
agent-sync -status
```

---

## 🛡️ Ferramentas Inclusas
- **`ast-outline <arquivo>`**: Gera a estrutura de classes e funções com linhas correspondentes, economizando até 90% dos tokens de contexto.
- **`trace-strip <arquivo_ou_pipe>`**: Oculta frames irrelevantes de stack traces de bibliotecas externas.
- **`db-guardian -list`** / **`-profile <banco> -query "<sql>"`**: Consulta bancos protegendo dados sensíveis e prevenindo mutações acidentais.
- **`psr-check <arquivo.php> [--fix]`**: Validação e correção direta de regras PSR-12 e PER-CS 2.0.
