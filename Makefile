.PHONY: build install sync test clean

build:
	go build -o bin/agent-sync ./cmd/agent-sync
	cd tools && go build -o ../bin/ast-outline ./cmd/ast-outline
	cd tools && go build -o ../bin/trace-strip ./cmd/trace-strip
	cd tools && go build -o ../bin/db-guardian ./cmd/db-guardian

install: build
	mkdir -p ~/.local/bin
	cp bin/* ~/.local/bin/
	@echo "Binários instalados em ~/.local/bin com sucesso!"

sync: install
	~/.local/bin/agent-sync -apply

status:
	~/.local/bin/agent-sync -status
