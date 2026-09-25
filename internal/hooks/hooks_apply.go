package hooks

// hooks_apply.go: infraestrutura compartilhada para wirar de hooks cross-CLI.
// Camada de aplicacao: tipos JSON comuns, helpers de encode/decode/upsert,
// syncHookCommandAtEvent (gravacao em settings.json), adaptadores Codex
// (protect-mcp), e adaptacao de plugins OpenCode v1/v2.
//
// Migrado de hooks.go em 2026-09-21 (Fase 2 do refator por feature). Sem
// mudanca de comportamento: mesmos tipos, mesmos helpers, mesmas assinaturas.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// hookEntry é a estrutura JSON de {"matcher", "hooks": [...]} que cada CLI
// grava por evento (ex.: {"matcher": "*", "hooks": [{"type":"command",
// "command":"...", "name":"..."}]}).
type hookEntry struct {
	Matcher string    `json:"matcher"`
	Hooks   []hookCmd `json:"hooks"`
}

// hookCmd é o conteúdo do array "hooks" interno de hookEntry. Campos opcionais
// (`name`, `timeout`, `if`) são ignorados por algumas CLIs mas aceitos pelo
// formato JSON compartilhado entre Claude Code e Codex.
type hookCmd struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Name    string `json:"name,omitempty"`
	Timeout int    `json:"timeout,omitempty"`
	If      string `json:"if,omitempty"`
}

// syncStandardHook cobre o formato compartilhado por Claude Code e Codex:
// {"hooks": {"<Evento>": [{"matcher", "hooks": [...]}]}}. Espera um script
// em disco (hooks/<scriptName>); para instalar um comando literal (ex.: um
// subcomando do binário ctx-window, sem arquivo de script), use
// syncStandardHookCommand diretamente.
func syncStandardHook(baseDir string, target TargetCLI, hookName, scriptName, matcher string) error {
	return syncStandardHookFiltered(baseDir, target, hookName, scriptName, matcher, nil)
}

// syncStandardHookFiltered adiciona o campo `if` em cada hookCmd para que
// Claude Code (e Codex) filtrem sub-comandos antes do spawn. Em outros
// targets, o campo `if` é silenciosamente aceito mas ignorado.
func syncStandardHookFiltered(baseDir string, target TargetCLI, hookName, scriptName, matcher string, ifFilters []string) error {
	scriptPath := filepath.Join(baseDir, "hooks", scriptName)
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("script do hook não encontrado: %s", scriptPath)
	}
	return syncStandardHookCommandFiltered(baseDir, target, hookName, scriptPath, matcher, ifFilters)
}

// syncStandardHookCommand é a versão sem exigência de arquivo em disco:
// `command` é gravado literalmente no settings.json (pode ser um script ou
// um comando de binário já esperado no PATH, ex. "ctx-window hook claude").
// Sempre grava no evento padrão do target (HooksEvent).
func syncStandardHookCommand(baseDir string, target TargetCLI, hookName, command, matcher string) error {
	return syncStandardHookCommandFiltered(baseDir, target, hookName, command, matcher, nil)
}

// syncStandardHookCommandFiltered adiciona o campo `if` (um por entry,
// quando ifFilters não-vazio) ao hookCmd para que o Claude Code (e Codex,
// mesmo formato) filtre sub-comandos antes do spawn. Em outros targets, o
// campo é silenciosamente aceito mas ignorado pela CLI.
func syncStandardHookCommandFiltered(baseDir string, target TargetCLI, hookName, command, matcher string, ifFilters []string) error {
	return syncHookCommandAtEvent(baseDir, target, hookName, command, matcher, target.HooksEvent, ifFilters)
}

// syncHookCommandAtEvent grava o hook em um evento específico, não
// necessariamente o HooksEvent padrão do target. Usado pelo shell-validate,
// que precisa ir em PreToolUse (verificar antes de executar) enquanto o
// event padrão das CLIs é PostToolUse. ifFilters opcional injeta o campo
// `if` em uma entry por filtro (válido em tool events no Claude Code;
// ignorado em outros).
func syncHookCommandAtEvent(baseDir string, target TargetCLI, hookName, command, matcher, event string, ifFilters []string) error {
	if target.HooksSettingsPath == "" {
		return nil
	}
	settings, err := readJSONObject(target.HooksSettingsPath)
	if err != nil {
		return err
	}

	hooksRoot, _ := settings["hooks"].(map[string]interface{})
	if hooksRoot == nil {
		hooksRoot = map[string]interface{}{}
	}

	entries := decodeHookEntries(hooksRoot[event])
	if target.AgentKind == "codex" {
		adapterPath := filepath.Join(baseDir, "hooks", "codex-protect-mcp-adapter.sh")
		entries = adaptCodexProtectionHooks(entries, adapterPath)
		if event != "PreToolUse" {
			hooksRoot["PreToolUse"] = adaptCodexProtectionHooks(decodeHookEntries(hooksRoot["PreToolUse"]), adapterPath)
		}
	}
	hooksRoot[event] = upsertHookEntry(entries, command, hookName, matcher, ifFilters)

	// Limpa wirar orfaos do mesmo hook em outros eventos (migracao
	// PostToolUse -> PreToolUse em 2026-09-23). Preserva wirar de
	// outros hooks e nao toca em eventos que nao sejam o canônico
	// (SessionStart/Stop etc.).
	for otherEvent := range hooksRoot {
		if otherEvent == event {
			continue
		}
		if otherEvent == "SessionStart" || otherEvent == "Stop" {
			continue
		}
		otherEntries := decodeHookEntries(hooksRoot[otherEvent])
		filtered := otherEntries[:0]
		changed := false
		for _, e := range otherEntries {
			keepEntry := true
			innerFiltered := e.Hooks[:0]
			for _, h := range e.Hooks {
				if h.Name == hookName {
					changed = true
					continue
				}
				innerFiltered = append(innerFiltered, h)
			}
			if changed {
				e.Hooks = innerFiltered
				if len(e.Hooks) == 0 {
					keepEntry = false
				}
			}
			if keepEntry {
				filtered = append(filtered, e)
			}
		}
		if changed {
			if len(filtered) == 0 {
				delete(hooksRoot, otherEvent)
			} else {
				hooksRoot[otherEvent] = filtered
			}
		}
	}

	settings["hooks"] = hooksRoot

	return writeJSONObject(target.HooksSettingsPath, settings)
}

// adaptCodexProtectionHooks substitui invocações de npx protect-mcp@0.7.4
// pelo adapter local (hooks/codex-protect-mcp-adapter.sh) para evitar
// downloads remotos em runtime. Aplica-se por hook entry.
func adaptCodexProtectionHooks(entries []hookEntry, adapterPath string) []hookEntry {
	for i := range entries {
		for j := range entries[i].Hooks {
			command := entries[i].Hooks[j].Command
			if strings.Contains(command, "npx protect-mcp@0.7.4 evaluate") {
				entries[i].Hooks[j].Command = strings.Replace(command, "npx protect-mcp@0.7.4 evaluate", adapterPath+" evaluate", 1)
			}
			if strings.Contains(command, "npx protect-mcp@0.7.4 sign") {
				entries[i].Hooks[j].Command = strings.Replace(command, "npx protect-mcp@0.7.4 sign", adapterPath+" sign", 1)
			}
		}
	}
	return entries
}

// upsertHookEntry adiciona ou atualiza uma hookEntry pelo nome. Entradas
// existentes com o mesmo hookName são removidas antes de inserir a nova,
// garantindo idempotência.
func upsertHookEntry(entries []hookEntry, scriptPath, hookName, matcher string, ifFilters []string) []hookEntry {
	filtered := entries[:0:0]
	for _, e := range entries {
		if !hasNamedHook(e, hookName) {
			filtered = append(filtered, e)
		}
	}
	if len(ifFilters) == 0 {
		filtered = append(filtered, hookEntry{
			Matcher: matcher,
			Hooks: []hookCmd{
				{Type: "command", Command: scriptPath, Name: hookName, Timeout: 10},
			},
		})
		return filtered
	}
	for _, ifFilter := range ifFilters {
		filtered = append(filtered, hookEntry{
			Matcher: matcher,
			Hooks: []hookCmd{
				{Type: "command", Command: scriptPath, Name: hookName, Timeout: 10, If: ifFilter},
			},
		})
	}
	return filtered
}

// decodeHookEntries converte uma interface{} (vinda de JSON unmarshal) em
// []hookEntry. Tolerante a formatos parciais (retorna [] se incompreensível).
func decodeHookEntries(raw interface{}) []hookEntry {
	if raw == nil {
		return nil
	}
	list, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]hookEntry, 0, len(list))
	for _, item := range list {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		matcher, _ := entry["matcher"].(string)
		hooksRaw, _ := entry["hooks"].([]interface{})
		hooks := make([]hookCmd, 0, len(hooksRaw))
		for _, hr := range hooksRaw {
			hc, ok := hr.(map[string]interface{})
			if !ok {
				continue
			}
			cmd := hookCmd{}
			cmd.Type, _ = hc["type"].(string)
			cmd.Command, _ = hc["command"].(string)
			cmd.Name, _ = hc["name"].(string)
			if t, ok := hc["timeout"].(float64); ok {
				cmd.Timeout = int(t)
			}
			cmd.If, _ = hc["if"].(string)
			hooks = append(hooks, cmd)
		}
		out = append(out, hookEntry{Matcher: matcher, Hooks: hooks})
	}
	return out
}

// encodeHookEntries converte []hookEntry de volta para o formato JSON
// (usado em Cursor que tem merge próprio, ver hooks_cursor_apply.go).
func encodeHookEntries(entries []hookEntry) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(entries))
	for _, e := range entries {
		hooksRaw := make([]map[string]interface{}, 0, len(e.Hooks))
		for _, h := range e.Hooks {
			hc := map[string]interface{}{
				"type":    h.Type,
				"command": h.Command,
			}
			if h.Name != "" {
				hc["name"] = h.Name
			}
			if h.Timeout != 0 {
				hc["timeout"] = h.Timeout
			}
			if h.If != "" {
				hc["if"] = h.If
			}
			hooksRaw = append(hooksRaw, hc)
		}
		out = append(out, map[string]interface{}{
			"matcher": e.Matcher,
			"hooks":   hooksRaw,
		})
	}
	return out
}

// hasNamedHook verifica se uma hookEntry contém um hook com o nome dado.
// Migrado de bashguardian.go em 2026-09-21 (Fase 8 refator: bash guardian
// passou a chamar hooks_apply.go para compartilhar helper).
func hasNamedHook(e hookEntry, name string) bool {
	for _, h := range e.Hooks {
		if h.Name == name {
			return true
		}
	}
	return false
}
