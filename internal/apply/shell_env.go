package apply

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// apply_shell_env.go: persistencia de env vars no shell rc (~/.zshrc ou
// ~/.bashrc) usada por `agent-sync -apply` para marcar wirar do
// shell-validate (opt-in via AGENT_SYNC_PRETOOLUSE_VALIDATE=1).
//
// Migrado de main.go em 2026-09-21 (Fase 6). replaceBlock eh usado para
// insercao idempotente do marker no rc file.

const shellEnvMarker = "# agent-sync: shell-validate hook (gerenciado por `agent-sync -apply`)"

func persistShellEnv() error {
	if shouldDryRun() {
		fmt.Println("[dry-run] persistir AGENT_SYNC_PRETOOLUSE_VALIDATE=1 no shell rc (noop em dry-run)")
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("encontrar HOME: %w", err)
	}
	rcPath := filepath.Join(home, ".zshrc")
	if _, err := os.Stat(rcPath); err != nil {
		rcPath = filepath.Join(home, ".bashrc")
	}

	existing, err := os.ReadFile(rcPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("ler %s: %w", rcPath, err)
	}

	block := shellEnvMarker + "\nexport AGENT_SYNC_PRETOOLUSE_VALIDATE=1\n"
	var out []byte
	if bytes.Contains(existing, []byte(shellEnvMarker)) {
		// Substitui o bloco existente (marcador + 1 linha) por uma versão nova.
		out = replaceBlock(existing, shellEnvMarker, block)
		fmt.Printf("✅ env atualizada em %s\n", rcPath)
	} else {
		// Anexa novo bloco com separador para legibilidade.
		sep := []byte("\n")
		if len(existing) > 0 && existing[len(existing)-1] != '\n' {
			sep = []byte("\n\n")
		}
		out = append(existing, sep...)
		out = append(out, []byte(block)...)
		fmt.Printf("✅ env persistida em %s (próxima sessão já ativa)\n", rcPath)
	}

	info, err := os.Stat(rcPath)
	mode := os.FileMode(0o644)
	if err == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(rcPath, out, mode)
}

func replaceBlock(data []byte, markerStart string, newBlock string) []byte {
	idx := bytes.Index(data, []byte(markerStart))
	if idx < 0 {
		return data
	}
	end := idx + len(markerStart)
	// Procura fim do bloco: próxima linha em branco dupla ou fim do arquivo.
	rest := data[end:]
	endOffset := len(data)
	for i := 0; i < len(rest); i++ {
		if i+1 < len(rest) && rest[i] == '\n' && rest[i+1] == '\n' {
			endOffset = end + i + 1
			break
		}
	}
	out := make([]byte, 0, len(data))
	out = append(out, data[:idx]...)
	out = append(out, []byte(newBlock)...)
	out = append(out, data[endOffset:]...)
	return out
}