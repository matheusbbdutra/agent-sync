package hooks

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// loadBashGuardianPatterns lê hooks/bash-guardian-patterns.txt (um padrão glob
// por linha; linhas em branco ou iniciadas com # são ignoradas).
func loadBashGuardianPatterns(baseDir string) ([]string, error) {
	path, err := pathutil.HookScriptPath(baseDir, "bash-guardian-patterns.txt")
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var patterns []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns, scanner.Err()
}

// syncBashGuardianClaude adiciona os padrões de risco à lista nativa
// permissions.ask do Claude Code, preservando o restante do settings.json.
// Claude Code não trata a ordem dentro do array como sequencial/prioritária
// (é checagem de pertencimento), então o merge genérico é seguro aqui.
func syncBashGuardianClaude(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" {
		return nil
	}
	patterns, err := loadBashGuardianPatterns(baseDir)
	if err != nil {
		return err
	}

	settings, err := readJSONObject(target.HooksSettingsPath)
	if err != nil {
		return err
	}

	permissions, _ := settings["permissions"].(map[string]interface{})
	if permissions == nil {
		permissions = map[string]interface{}{}
	}

	existing := decodeStringSlice(permissions["ask"])
	// remove entradas anteriores do bash-guardian (marcadas com o comentário
	// implícito de padrão Bash(...)) antes de reinserir a versão atual.
	kept := existing[:0:0]
	for _, e := range existing {
		if !isBashGuardianEntry(e, patterns) {
			kept = append(kept, e)
		}
	}
	for _, p := range patterns {
		kept = append(kept, fmt.Sprintf("Bash(%s)", p))
	}
	permissions["ask"] = kept
	settings["permissions"] = permissions

	return writeJSONObject(target.HooksSettingsPath, settings)
}

func isBashGuardianEntry(entry string, patterns []string) bool {
	for _, p := range patterns {
		if entry == fmt.Sprintf("Bash(%s)", p) {
			return true
		}
	}
	return false
}

func decodeStringSlice(raw interface{}) []string {
	if raw == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var out []string
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}

const bashGuardianHookName = "agent-sync-bash-guardian"

// syncBashGuardianAntigravity instala o hook PreToolUse que pede confirmação
// (decision: ask) quando um comando bate um padrão de risco conhecido.
func syncBashGuardianAntigravity(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksFormat != "antigravity" {
		return nil
	}
	scriptPath, err := pathutil.HookScriptPath(baseDir, "bash-guardian.antigravity.sh")
	if err != nil {
		return err
	}
	wrapped := wrapHookCommand(baseDir, "PreToolUse", bashGuardianHookName, scriptPath)

	targetCopy := target
	targetCopy.HooksEvent = "PreToolUse"
	if err := syncAntigravityHookCommand(targetCopy, bashGuardianHookName, wrapped, "run_command"); err != nil {
		return err
	}

	// Limpa entrada órfã em PreInvocation se existir (de versões legadas).
	root, err := readJSONObject(target.HooksSettingsPath)
	if err == nil {
		if g, ok := root[bashGuardianHookName].(map[string]interface{}); ok {
			if _, exists := g["PreInvocation"]; exists {
				delete(g, "PreInvocation")
				return writeJSONObject(target.HooksSettingsPath, root)
			}
		}
	}
	return nil
}


// --- OpenCode: merge que preserva a ordem de permission.bash -------------

const openCodeConfigFile = "opencode.json"

// syncBashGuardianOpenCode mescla os padrões de risco em permission.bash no
// opencode.json, marcados como "ask". Como o OpenCode resolve a regra pela
// ÚLTIMA que casar (ordem de definição importa), esta função preserva a ordem
// original das chaves já existentes e só reordena as que pertencem ao
// agent-sync, sempre reinserindo-as ao final.
func syncBashGuardianOpenCode(baseDir string, target TargetCLI) error {
	if target.OpenCodePluginDir == "" {
		return nil
	}
	patterns, err := loadBashGuardianPatterns(baseDir)
	if err != nil {
		return err
	}

	configPath := filepath.Join(filepath.Dir(target.OpenCodePluginDir), openCodeConfigFile)

	raw, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		raw = []byte(`{"$schema":"https://opencode.ai/config.json"}`)
	} else if err != nil {
		return err
	}

	var root map[string]json.RawMessage
	if len(raw) == 0 {
		root = map[string]json.RawMessage{}
	} else if err := json.Unmarshal(raw, &root); err != nil {
		return fmt.Errorf("%s: JSON inválido, corrija manualmente antes de sincronizar: %w", configPath, err)
	}

	permission := map[string]json.RawMessage{}
	if raw, ok := root["permission"]; ok {
		if err := json.Unmarshal(raw, &permission); err != nil {
			return fmt.Errorf("%s: campo permission inválido: %w", configPath, err)
		}
	}

	var bashPairs []kvPair
	if raw, ok := permission["bash"]; ok {
		bashPairs, err = decodeOrderedStringMap(raw)
		if err != nil {
			return fmt.Errorf("%s: permission.bash inválido: %w", configPath, err)
		}
	}

	bashPairs = upsertBashGuardianPairs(bashPairs, patterns)

	bashRaw, err := encodeOrderedStringMap(bashPairs)
	if err != nil {
		return err
	}
	permission["bash"] = bashRaw

	permissionRaw, err := json.Marshal(permission)
	if err != nil {
		return err
	}
	root["permission"] = permissionRaw

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(configPath, out, 0o644)
}

type kvPair struct {
	Key   string
	Value string
}

func upsertBashGuardianPairs(existing []kvPair, patterns []string) []kvPair {
	isOurs := func(key string) bool {
		for _, p := range patterns {
			if key == p {
				return true
			}
		}
		return false
	}
	kept := existing[:0:0]
	for _, kv := range existing {
		if !isOurs(kv.Key) {
			kept = append(kept, kv)
		}
	}
	for _, p := range patterns {
		kept = append(kept, kvPair{Key: p, Value: "ask"})
	}
	return kept
}

// decodeOrderedStringMap lê um objeto JSON {string: string} preservando a
// ordem original das chaves (encoding/json em map[string]... não preserva).
func decodeOrderedStringMap(raw json.RawMessage) ([]kvPair, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("esperava objeto JSON")
	}

	var pairs []kvPair
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("chave não-string em permission.bash")
		}
		var value string
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		pairs = append(pairs, kvPair{Key: key, Value: value})
	}
	return pairs, nil
}

// encodeOrderedStringMap serializa os pares preservando a ordem fornecida.
func encodeOrderedStringMap(pairs []kvPair) (json.RawMessage, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, kv := range pairs {
		if i > 0 {
			buf.WriteByte(',')
		}
		keyJSON, err := json.Marshal(kv.Key)
		if err != nil {
			return nil, err
		}
		valJSON, err := json.Marshal(kv.Value)
		if err != nil {
			return nil, err
		}
		buf.Write(keyJSON)
		buf.WriteByte(':')
		buf.Write(valJSON)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
