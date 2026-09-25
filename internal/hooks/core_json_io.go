package hooks

// core_json_io.go: helpers cross-feature para leitura/escrita de objetos JSON
// usados no wirar de hooks. Prefixo 'core_' em vez de subpasta porque Go
// trata cada pasta como package separado - manter mesmo package main exige
// um arquivo por diretorio.
//
// Migrado de hooks.go em 2026-09-21 (refator por feature - Fase 1.2 do
// detalhamento). Sem mudanca de comportamento: mesmas assinaturas, mesmos
// helpers, mesmo tratamento de dry-run.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// readJSONObject le um arquivo JSON como map[string]interface{}. Se o arquivo
// nao existir ou estiver vazio, retorna mapa vazio (caminho feliz comum em
// wirar de hooks onde o settings.json ainda nao existe).
func readJSONObject(path string) (map[string]interface{}, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]interface{}{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return map[string]interface{}{}, nil
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("%s: JSON inválido, corrija manualmente antes de sincronizar: %w", path, err)
	}
	return obj, nil
}

// writeJSONObject grava um objeto JSON no caminho com formatacao 2-space e
// trailing newline. Respeita shouldDryRun() (no-op + log em dry-run).
func writeJSONObject(path string, obj map[string]interface{}) error {
	if shouldDryRun() {
		fmt.Printf("[dry-run] write json %s\n", path)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return os.WriteFile(path, encoded, 0o644)
}
