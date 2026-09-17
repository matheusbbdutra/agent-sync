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

// EnsureConfig cria um modelo privado sem substituir a configuração existente.
func EnsureConfig() (string, error) {
	path, err := ConfigPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("criar diretório de configuração: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		return path, nil
	}
	if err != nil {
		return "", fmt.Errorf("criar configuração: %w", err)
	}
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
	if address == "" || token == "" {
		return "", "", fmt.Errorf("preencha turso.url e turso.token em %s", path)
	}
	return address, token, nil
}
