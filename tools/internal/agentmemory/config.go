package agentmemory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config contém a configuração privada da sincronização remota.
type Config struct {
	Projects map[string]string `json:"projects"`
	Turso    struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	} `json:"turso"`
	Summarizer string `json:"summarizer,omitempty"`
	CTXK       int    `json:"ctx_k,omitempty"`
	CTXBudget  int    `json:"ctx_budget,omitempty"`
}

func ConfigPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("localizar diretório de configuração: %w", err)
	}
	return filepath.Join(root, "agent-sync", "config.json"), nil
}

// EnsureConfig garante que o arquivo de configuração existe com todos os campos
// obrigatórios. Se o arquivo já existir, injeta campos faltantes sem sobrescrever
// valores já preenchidos (merge seguro).
func EnsureConfig() (string, error) {
	path, err := ConfigPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("criar diretório de configuração: %w", err)
	}

	// Tenta criar atomicamente; se já existe, faz merge dos campos faltantes.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		// Arquivo novo: grava esqueleto completo e fecha.
		_, writeErr := file.WriteString("{\n  \"turso\": {\n    \"url\": \"\",\n    \"token\": \"\"\n  },\n  \"projects\": {}\n}\n")
		closeErr := file.Close()
		if writeErr != nil {
			return "", fmt.Errorf("escrever configuração: %w", writeErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("fechar configuração: %w", closeErr)
		}
		return path, nil
	}
	if !os.IsExist(err) {
		return "", fmt.Errorf("criar configuração: %w", err)
	}

	// Arquivo existe: lê e injeta campos faltantes.
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("ler configuração existente: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		// JSON corrompido — não toca no arquivo.
		return path, nil
	}
	changed := false
	if _, ok := m["turso"]; !ok {
		m["turso"] = map[string]any{"url": "", "token": ""}
		changed = true
	}
	if _, ok := m["projects"]; !ok {
		m["projects"] = map[string]any{}
		changed = true
	}
	if !changed {
		return path, nil
	}
	merged, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", fmt.Errorf("serializar configuração: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(merged, '\n'), 0o600); err != nil {
		return "", fmt.Errorf("escrever configuração temporária: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("atualizar configuração: %w", err)
	}
	return path, nil
}

func LoadRemoteConfig() (string, string, error) {
	path, err := EnsureConfig()
	if err != nil {
		return "", "", err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", "", fmt.Errorf("inspecionar configuração: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("configuração deve ser um arquivo regular: %s", path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return "", "", fmt.Errorf("permissões inseguras em %s; use chmod 600", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("ler configuração: %w", err)
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return "", "", fmt.Errorf("JSON inválido em %s: %w", path, err)
	}
	address := strings.TrimSpace(config.Turso.URL)
	token := strings.TrimSpace(config.Turso.Token)
	// Token pode vir de env var (AGENT_SYNC_TURSO_TOKEN) com prioridade sobre JSON.
	// URL permanece no JSON. User prefere secrets em env var (zshrc) por causa de
	// chmod 600 fragil. Origem: ses_f2a16545bffeeCcxIFwZ04VVDa, 2026-09-25.
	if envToken := strings.TrimSpace(os.Getenv("AGENT_SYNC_TURSO_TOKEN")); envToken != "" {
		token = envToken
	}
	if address == "" || token == "" {
		return "", "", fmt.Errorf("preencha turso.url em %s e AGENT_SYNC_TURSO_TOKEN (env var) ou turso.token (JSON)", path)
	}
	return address, token, nil
}
