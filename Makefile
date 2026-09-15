.PHONY: help setup build install sync status vendor mcp mirror test clean

.DEFAULT_GOAL := help

help: ## Lista os comandos disponíveis
	@echo "Uso: make <alvo>"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

setup: ## Garante o Go (>= 1.24) via mise ou tarball oficial
	bash scripts/setup-go.sh

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

install: build ## Compila e instala os binários/scripts em ~/.local/bin
	mkdir -p ~/.local/bin
	@for f in bin/*; do \
		name="$$(basename "$$f")"; \
		cp "$$f" ~/.local/bin/"$$name".tmp && chmod +x ~/.local/bin/"$$name".tmp && mv -f ~/.local/bin/"$$name".tmp ~/.local/bin/"$$name"; \
	done
	install -m 0755 scripts/delegate-run.sh ~/.local/bin/delegate-run
	@echo "Binários instalados em ~/.local/bin com sucesso!"

sync: install ## Instala e roda agent-sync -apply (sincroniza as 5 CLIs)
	~/.local/bin/agent-sync -apply

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

clean: ## Remove os binários compilados em bin/
	rm -rf bin
