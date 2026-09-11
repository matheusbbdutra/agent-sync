.PHONY: setup build install sync status vendor mcp mirror test clean

setup:
	bash scripts/setup-go.sh

build:
	go build -o bin/agent-sync ./cmd/agent-sync
	cd tools && go build -o ../bin/ast-outline ./cmd/ast-outline
	cd tools && go build -o ../bin/trace-strip ./cmd/trace-strip
	cd tools && go build -o ../bin/db-guardian ./cmd/db-guardian
	cd tools && go build -o ../bin/docs-fetch ./cmd/docs-fetch
	cd tools && go build -o ../bin/docs-mcp ./cmd/docs-mcp

install: build
	mkdir -p ~/.local/bin
	cp bin/* ~/.local/bin/
	@echo "Binários instalados em ~/.local/bin com sucesso!"

sync: install
	~/.local/bin/agent-sync -apply

status:
	~/.local/bin/agent-sync -status

vendor:
	go run ./cmd/agent-sync -vendor

mcp:
	bash scripts/setup-mcp.sh

mirror:
	cd tools && go run ./cmd/docs-fetch -mirror ../mirror/sources.json

test:
	go test ./...
	cd tools && go test ./...

clean:
	rm -rf bin
