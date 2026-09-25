.PHONY: help setup setup-go setup-opencode build install apply sync status vendor mcp mirror test lint fmt clean

.DEFAULT_GOAL := help

help: ## Lista os comandos disponíveis
	@echo "Uso: make <alvo>"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

setup: setup-go setup-opencode ## Instala todos os pre-requisitos de runtime (Go + OpenCode v2)

setup-go: ## Garante o Go (>= 1.24) via mise ou tarball oficial
	bash scripts/setup-go.sh

setup-opencode: ## Garante Node + @opencode/plugin para os plugins OpenCode v2 wirados
	bash scripts/setup-opencode.sh

build: ## Compila agent-sync e as ferramentas em bin/
	go build -o bin/agent-sync ./cmd/agent-sync
	cd tools && go build -o ../bin/ast-outline ./cmd/ast-outline
	cd tools && go build -o ../bin/trace-strip ./cmd/trace-strip
	cd tools && go build -o ../bin/db-guardian ./cmd/db-guardian
	cd tools && go build -o ../bin/docs-fetch ./cmd/docs-fetch
	cd tools && go build -o ../bin/docs-mcp ./cmd/docs-mcp
	cd tools && go build -o ../bin/docs-cache-write ./cmd/docs-cache-write
	cd tools && go build -o ../bin/memory-mcp ./cmd/memory-mcp
	cd tools && go build -o ../bin/git-diff-summary ./cmd/git-diff-summary
	cd tools && go build -o ../bin/mr-review-local ./cmd/mr-review-local
	cd tools && go build -o ../bin/mr-collect-cli ./cmd/mr-collect-cli
	cd tools && go build -o ../bin/memory-sync ./cmd/memory-sync
	cd tools && go build -o ../bin/ctx-window ./cmd/ctx-window
	cd tools && go build -o ../bin/false-success-guard ./cmd/false-success-guard
	cd tools && go build -o ../bin/shell-validate ./cmd/shell-validate
	cd tools && go build -o ../bin/repo-map ./cmd/repo-map

install: build ## Compila e instala os binários/scripts em ~/.local/bin
	mkdir -p ~/.local/bin
	@for f in bin/*; do \
		name="$$(basename "$$f")"; \
		cp "$$f" ~/.local/bin/"$$name".tmp && chmod +x ~/.local/bin/"$$name".tmp && mv -f ~/.local/bin/"$$name".tmp ~/.local/bin/"$$name"; \
	done
	install -m 0755 scripts/delegate-run.sh ~/.local/bin/delegate-run
	install -m 0755 scripts/agent-sync-session.sh ~/.local/bin/agent-sync-session
	@echo "Binários instalados em ~/.local/bin com sucesso!"

apply: install ## Compila, instala e aplica agent-sync nas 5 CLIs (hooks + skills + regras)
	# shell-validate é opt-in via env (AGENT_SYNC_PRETOOLUSE_VALIDATE=1).
	# `agent-sync -apply` persiste a env em ~/.zshrc (ou ~/.bashrc) por
	# conta própria, então não precisa setar manualmente aqui.
	~/.local/bin/agent-sync -apply
	# Garante o esqueleto de ~/.config/agent-sync/config.json (turso.url/token
	# vazios) sem sobrescrever um já existente; EnsureConfig() é idempotente.
	# Preencher o token é manual — não expor segredo em log/commit.
	~/.local/bin/memory-sync -init

sync: apply ## Alias silencioso de apply (mantido para retrocompatibilidade)

status: ## Mostra o status de sincronização de cada CLI
	~/.local/bin/agent-sync -status

vendor: ## Reimporta as skills curadas do catálogo (skills/manifest.json)
	go run ./cmd/agent-sync -vendor

mcp: ## Configura os MCPs (context7, docs, sentry) nos CLIs instalados
	bash scripts/setup-mcp.sh

mirror: ## Baixa/atualiza o cache offline de docs (mirror/sources.json)
	cd tools && go run ./cmd/docs-fetch -mirror ../mirror/sources.json

test: ## Roda os testes Go (raiz + tools)
	go test ./...
	cd tools && go test ./...

lint: ## go vet + gofmt -l em ambos os módulos (nao mutativo)
	go vet ./...
	cd tools && go vet ./...
	@gofmt -l . | tee /tmp/agent-sync-gofmt-root.out && [ ! -s /tmp/agent-sync-gofmt-root.out ]
	cd tools && gofmt -l . | tee /tmp/agent-sync-gofmt-tools.out && [ ! -s /tmp/agent-sync-gofmt-tools.out ]

fmt: ## Corrige formatação com gofmt -w em ambos os módulos
	gofmt -w .
	cd tools && gofmt -w .

clean: ## Remove os binários compilados em bin/
	rm -rf bin
