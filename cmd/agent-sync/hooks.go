package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const contextGuardHookName = "agent-sync-context-guard"
const docsCacheHookName = "agent-sync-docs-cache"
const memoryNudgeHookName = "agent-sync-memory-nudge"
const agentReactNudgeHookName = "agent-sync-agent-react-nudge"
const ctxCompactHookName = "agent-sync-ctx-compact"
const ctxHandoffHookName = "agent-sync-ctx-handoff"
const ctxNudgeHookName = "agent-sync-ctx-nudge"
const shellValidateHookName = "agent-sync-shell-validate"
const agentStopHookName = "agent-sync-agent-stop"
const preInvocationReminderHookName = "agent-sync-preinvocation-reminder"

// hookEntry é o formato comum a Claude Code e Gemini CLI para um item de hooks.<Evento>[].
type hookEntry struct {
	Matcher string    `json:"matcher"`
	Hooks   []hookCmd `json:"hooks"`
}

type hookCmd struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Name    string `json:"name,omitempty"`
	Timeout int    `json:"timeout,omitempty"`
}

// syncHooks injeta (ou atualiza) o hook de lembrete do context-guard no
// arquivo de configuração da CLI alvo, preservando o que já existir.
// Só age quando o target define HooksSettingsPath e HooksEvent.
func syncHooks(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksEvent == "" {
		return nil
	}
	scriptName := "context-guard-nudge.sh"
	if target.HooksFormat == "antigravity" {
		scriptName = "context-guard-nudge.antigravity.sh"
		return syncAntigravityHook(baseDir, target, contextGuardHookName, scriptName, "*")
	}
	return syncStandardHook(baseDir, target, contextGuardHookName, scriptName, "*")
}

// syncMemoryNudgeHook instala o hook que lembra periodicamente de checar/
// gravar memoria via memory-mcp (store_memory), ja que hoje isso depende
// so da disciplina do modelo seguindo o CLAUDE.md. Suportado onde já existe
// HooksSettingsPath/HooksEvent (Claude Code, Codex e Antigravity).
func syncMemoryNudgeHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksEvent == "" {
		return nil
	}
	if target.HooksFormat == "antigravity" {
		return syncAntigravityHook(baseDir, target, memoryNudgeHookName, "memory-nudge.antigravity.sh", "*")
	}
	return syncStandardHook(baseDir, target, memoryNudgeHookName, "memory-nudge.sh", "*")
}

// syncAgentReactNudgeHook instala o lembrete de validacao de hipoteses
// (skill agent-react): hipotese != fato; validar ou pedir passo ao usuario.
func syncAgentReactNudgeHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksEvent == "" {
		return nil
	}
	if target.HooksFormat == "antigravity" {
		return syncAntigravityHook(baseDir, target, agentReactNudgeHookName, "agent-react-nudge.antigravity.sh", "*")
	}
	return syncStandardHook(baseDir, target, agentReactNudgeHookName, "agent-react-nudge.sh", "*")
}

// syncCtxCompactHook registra chamadas de ferramenta para um resumo manual e hooks de nudge.
func syncCtxCompactHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksEvent == "" {
		return nil
	}
	command := "ctx-window hook " + target.AgentKind
	if target.HooksFormat == "cursor" {
		if err := syncCursorCommandAtEvent(target.HooksSettingsPath, "postToolUse", command); err != nil {
			return err
		}
		return syncCursorCommandAtEvent(target.HooksSettingsPath, "preCompact", command+" precompact")
	}
	if target.HooksFormat == "antigravity" {
		targetCopy := target
		targetCopy.HooksEvent = "PostToolUse"
		if err := syncAntigravityHookCommand(targetCopy, ctxCompactHookName, command, "*"); err != nil {
			return err
		}
		root, err := readJSONObject(target.HooksSettingsPath)
		if err == nil {
			if g, ok := root[ctxCompactHookName].(map[string]interface{}); ok {
				delete(g, "PreInvocation")
				_ = writeJSONObject(target.HooksSettingsPath, root)
			}
		}
		return syncAntigravityPreInvocation(target, ctxNudgeHookName, command+" preinvocation")
	}
	return syncStandardHookCommand(baseDir, target, ctxCompactHookName, command, "*")
}

func syncAntigravityFlatHook(target TargetCLI, hookName, event, command string) error {
	root, err := readJSONObject(target.HooksSettingsPath)
	if err != nil {
		return err
	}
	group, _ := root[hookName].(map[string]interface{})
	if group == nil {
		group = map[string]interface{}{}
	}
	delete(group, "SessionStart")
	group[event] = []hookCmd{{Type: "command", Command: command, Name: hookName, Timeout: 10}}
	root[hookName] = group
	return writeJSONObject(target.HooksSettingsPath, root)
}

func syncAntigravityPreInvocation(target TargetCLI, hookName, command string) error {
	return syncAntigravityFlatHook(target, hookName, "PreInvocation", command)
}

func syncAntigravityStop(target TargetCLI, hookName, command string) error {
	return syncAntigravityFlatHook(target, hookName, "Stop", command)
}

// wrapHookCommand envelopa scriptPath com wrap-hook.sh para captura de
// exit code/stderr/duration_ms via observe-error.sh. Retorna o comando
// absoluto pronto para gravar no settings.json do target.
func wrapHookCommand(baseDir, stage, hookName, scriptPath string) string {
	wrapper := filepath.Join(baseDir, "hooks", "wrap-hook.sh")
	return wrapper + " " + stage + " " + hookName + " " + scriptPath
}

// syncStopHook instala o hook para o evento oficial Stop no Antigravity, Claude Code e Codex.
func syncStopHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" {
		return nil
	}
	if target.HooksFormat == "antigravity" {
		scriptPath := filepath.Join(baseDir, "hooks", "agent-stop.antigravity.sh")
		if _, err := os.Stat(scriptPath); err != nil {
			return fmt.Errorf("script do hook stop não encontrado: %s", scriptPath)
		}
		wrapped := wrapHookCommand(baseDir, "Stop", "agent-stop.antigravity", scriptPath)
		return syncAntigravityStop(target, agentStopHookName, wrapped)
	}
	if target.AgentKind == "claude" || target.AgentKind == "codex" {
		return syncHookCommandAtEvent(baseDir, target, "agent-sync-false-success-guard", "false-success-guard hook", "*", "Stop")
	}
	return nil
}

// syncPreInvocationReminderHook registra no evento PreInvocation do Antigravity CLI o lembrete
// efêmero just-in-time ("Diretriz ativa: responda em PT-BR, sem rodeios e finalize com o resumo de 1-2 frases do que mudou e o que falta.").
func syncPreInvocationReminderHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" {
		return nil
	}
	if target.HooksFormat == "antigravity" {
		scriptPath := filepath.Join(baseDir, "hooks", "agent-preinvocation.antigravity.sh")
		if _, err := os.Stat(scriptPath); err != nil {
			return fmt.Errorf("script do hook preinvocation não encontrado: %s", scriptPath)
		}
		wrapped := wrapHookCommand(baseDir, "PreInvocation", "agent-preinvocation.antigravity", scriptPath)
		return syncAntigravityPreInvocation(target, preInvocationReminderHookName, wrapped)
	}
	return nil
}

func syncCtxHandoffHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" {
		return nil
	}
	command := "ctx-window handoff " + target.AgentKind
	switch target.HooksFormat {
	case "cursor":
		return syncCursorCommandAtEvent(target.HooksSettingsPath, "sessionStart", command)
	case "antigravity":
		return syncAntigravityPreInvocation(target, ctxHandoffHookName, command)
	default:
		return syncHookCommandAtEvent(baseDir, target, ctxHandoffHookName, command, ".*", "SessionStart")
	}
}

func syncCursorCommandAtEvent(path, event, command string) error {
	root, err := readJSONObject(path)
	if err != nil {
		return err
	}
	hooks, _ := root["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = map[string]interface{}{}
	}
	entries := decodeCursorHookEntries(hooks[event])
	kept := entries[:0:0]
	for _, entry := range entries {
		if entry.Command != command {
			kept = append(kept, entry)
		}
	}
	hooks[event] = encodeCursorHookEntries(append(kept, cursorHookEntry{Command: command}))
	root["hooks"] = hooks
	return writeJSONObject(path, root)
}

// syncShellValidateHook instala o hook PreToolUse que sinaliza comandos
// shell provavelmente inválidos (binário sem argumento posicional). É
// opt-in: o binário só emite aviso se AGENT_SYNC_PRETOOLUSE_VALIDATE=1
// estiver setado no ambiente. Por isso o hook é seguro de registrar —
// sem essa env, ele é no-op silencioso.
//
// Importante: este hook tem que ir em PreToolUse (verificar ANTES da
// execução), não no HooksEvent padrão da CLI (que é PostToolUse).
//
// Mapping de eventos por CLI (verificado no settings.json/schema oficial):
//   - Claude Code / Codex:  "PreToolUse" / matcher "Bash" (schema padrão)
//   - Cursor:               shell-validate não se aplica — Cursor já tem
//     `beforeShellExecution` com bash-guardian que faz
//     papel equivalente (instalado por syncBash- ou
//     outro helper específico do Cursor). Não escreve.
//   - Antigravity:          usa syncAntigravityHookCommand direto, formato
//     top-level já gravado em HooksEvent="PreInvocation".
func syncShellValidateHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" {
		return nil
	}
	if target.HooksFormat == "cursor" {
		// bash-guardian já cobre a checagem pré-execução em
		// beforeShellExecution; shell-validate seria redundante aqui.
		return nil
	}
	command := "shell-validate hook"
	if target.HooksFormat == "antigravity" {
		return syncAntigravityHookCommand(target, shellValidateHookName, command, "*")
	}
	// Claude, Codex (e qualquer outro que use schema padrão)
	return syncHookCommandAtEvent(baseDir, target, shellValidateHookName, command, "Bash", "PreToolUse")
}

// syncOpenCodeCtxCompactPlugin instala o plugin TS para tracking e para o
// callback de compactação nativa. A limitação #13574 afeta tool.execute.after.
func syncOpenCodeCtxCompactPlugin(baseDir string, target TargetCLI) error {
	if target.OpenCodePluginDir == "" {
		return nil
	}
	src := filepath.Join(baseDir, "hooks", "ctx-compact.opencode.ts")
	if _, err := os.Stat(src); err != nil {
		// plugin opcional — silencioso se nao existir
		return nil
	}
	return copyFile(src, filepath.Join(target.OpenCodePluginDir, "ctx-compact.ts"))
}

// syncDocsCacheHook instala o hook que cacheia passivamente docs consultadas
// via WebFetch/read_url_content e context7 (query-docs). Claude Code e Codex
// compartilham o mesmo script (schema de PostToolUse equivalente); Antigravity
// usa o seu próprio (lê o resultado do transcriptPath).
func syncDocsCacheHook(baseDir string, target TargetCLI) error {
	if target.HooksSettingsPath == "" || target.HooksEvent == "" {
		return nil
	}
	if target.HooksFormat == "antigravity" {
		return syncAntigravityHook(baseDir, target, docsCacheHookName, "docs-cache.antigravity.sh", "read_url_content|call_mcp_tool")
	}
	return syncStandardHook(baseDir, target, docsCacheHookName, "docs-cache.sh", "WebFetch|mcp__context7__.*")
}

// syncStandardHook cobre o formato compartilhado por Claude Code e Codex:
// {"hooks": {"<Evento>": [{"matcher", "hooks": [...]}]}}. Espera um script
// em disco (hooks/<scriptName>); para instalar um comando literal (ex.: um
// subcomando do binário ctx-window, sem arquivo de script), use
// syncStandardHookCommand diretamente.
func syncStandardHook(baseDir string, target TargetCLI, hookName, scriptName, matcher string) error {
	scriptPath := filepath.Join(baseDir, "hooks", scriptName)
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("script do hook não encontrado: %s", scriptPath)
	}
	return syncStandardHookCommand(baseDir, target, hookName, scriptPath, matcher)
}

// syncStandardHookCommand é a versão sem exigência de arquivo em disco:
// `command` é gravado literalmente no settings.json (pode ser um script ou
// um comando de binário já esperado no PATH, ex. "ctx-window hook claude").
// Sempre grava no evento padrão do target (HooksEvent).
func syncStandardHookCommand(baseDir string, target TargetCLI, hookName, command, matcher string) error {
	return syncHookCommandAtEvent(baseDir, target, hookName, command, matcher, target.HooksEvent)
}

// syncHookCommandAtEvent grava o hook em um evento específico, não
// necessariamente o HooksEvent padrão do target. Usado pelo shell-validate,
// que precisa ir em PreToolUse (verificar antes de executar) enquanto o
// event padrão das CLIs é PostToolUse.
func syncHookCommandAtEvent(baseDir string, target TargetCLI, hookName, command, matcher, event string) error {
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
	hooksRoot[event] = upsertHookEntry(entries, command, hookName, matcher)
	settings["hooks"] = hooksRoot

	return writeJSONObject(target.HooksSettingsPath, settings)
}

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

// adaptCodexPlugins percorre ~/.codex/plugins/*/hooks/hooks.json e adapta
// cada plugin que referenciar npx protect-mcp@0.7.4 evaluate/sign para
// passar pelo codex-protect-mcp-adapter.sh. Idempotente: plugins adaptados
// são pulados silenciosamente e o backup .bak só é criado na primeira vez.
//
// Se ~/.codex/plugins não existir (Codex não instalado), retorna nil.
//
// Se ~/.codex/config.toml contiver trusted_hash para algum plugin
// modificado, emite aviso em stderr mas prossegue com a adaptação
// (interpretação conservadora — Codex pode recusar o hook até o hash
// ser regenerado, mas isso é responsabilidade do usuário).
func adaptCodexPlugins(baseDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("encontrar HOME para plugins Codex: %w", err)
	}
	pluginsDir := filepath.Join(home, ".codex", "plugins")

	dirEntries, err := os.ReadDir(pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("listar %s: %w", pluginsDir, err)
	}

	adapterPath := filepath.Join(baseDir, "hooks", "codex-protect-mcp-adapter.sh")
	configTOML := filepath.Join(home, ".codex", "config.toml")
	trustedHashes := loadCodexTrustedHashes(configTOML)

	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() {
			continue
		}
		hooksPath := filepath.Join(pluginsDir, dirEntry.Name(), "hooks", "hooks.json")
		if _, statErr := os.Stat(hooksPath); statErr != nil {
			continue
		}

		data, readErr := os.ReadFile(hooksPath)
		if readErr != nil {
			return fmt.Errorf("ler %s: %w", hooksPath, readErr)
		}

		originalSnapshot := snapshotCodexPluginCommands(data)
		if originalSnapshot == nil {
			continue
		}
		if codexPluginAlreadyAdapted(originalSnapshot, adapterPath) {
			continue
		}
		if !codexPluginNeedsAdaptation(originalSnapshot) {
			continue
		}

		var root map[string]interface{}
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("JSON inválido em %s: %w", hooksPath, err)
		}

		hooksRoot, _ := root["hooks"].(map[string]interface{})
		if hooksRoot == nil {
			continue
		}

		for event, raw := range hooksRoot {
			eventEntries := decodeHookEntries(raw)
			adaptCodexProtectionHooks(eventEntries, adapterPath)
			hooksRoot[event] = eventEntries
		}
		root["hooks"] = hooksRoot

		encoded, err := json.MarshalIndent(root, "", "  ")
		if err != nil {
			return fmt.Errorf("serializar %s: %w", hooksPath, err)
		}
		encoded = append(encoded, '\n')

		bakPath := hooksPath + ".bak"
		if _, bakErr := os.Stat(bakPath); os.IsNotExist(bakErr) {
			if err := os.WriteFile(bakPath, data, 0o644); err != nil {
				return fmt.Errorf("criar backup %s: %w", bakPath, err)
			}
		}
		if err := os.WriteFile(hooksPath, encoded, 0o644); err != nil {
			return fmt.Errorf("escrever %s: %w", hooksPath, err)
		}

		if trustedHashes[dirEntry.Name()] {
			fmt.Fprintf(os.Stderr, "⚠️  Plugin %s: trusted_hash presente em config.toml; Codex pode recusar o hook até regenerar hash\n", dirEntry.Name())
		}
	}
	return nil
}

// snapshotCodexPluginCommands devolve cópias (por valor) das entradas de
// cada evento, ou nil se o JSON for inválido / não tiver chave "hooks".
// Usado para detectar "plugin já adaptado" sem mutar o original.
func snapshotCodexPluginCommands(data []byte) map[string][]hookEntry {
	var root struct {
		Hooks map[string][]hookEntry `json:"hooks"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}
	if root.Hooks == nil {
		return nil
	}
	out := make(map[string][]hookEntry, len(root.Hooks))
	for event, entries := range root.Hooks {
		copied := make([]hookEntry, len(entries))
		for i, e := range entries {
			copied[i] = e
			copied[i].Hooks = make([]hookCmd, len(e.Hooks))
			copy(copied[i].Hooks, e.Hooks)
		}
		out[event] = copied
	}
	return out
}

func codexPluginAlreadyAdapted(snapshot map[string][]hookEntry, adapterPath string) bool {
	for _, entries := range snapshot {
		for _, e := range entries {
			for _, h := range e.Hooks {
				if strings.Contains(h.Command, adapterPath) {
					return true
				}
			}
		}
	}
	return false
}

func codexPluginNeedsAdaptation(snapshot map[string][]hookEntry) bool {
	for _, entries := range snapshot {
		for _, e := range entries {
			for _, h := range e.Hooks {
				if strings.Contains(h.Command, "npx protect-mcp@0.7.4 evaluate") ||
					strings.Contains(h.Command, "npx protect-mcp@0.7.4 sign") {
					return true
				}
			}
		}
	}
	return false
}

// loadCodexTrustedHashes varre ~/.codex/config.toml procurando entradas
// trusted_hash da forma:
//
//	[hooks.state."<plugin-name>@<marketplace>:hooks/hooks.json:<event>:<i>:<j>"]
//	trusted_hash = "sha256:..."
//
// Retorna um map pluginName -> true quando o config.toml declara trusted_hash
// para aquele plugin. Conservador: parsing por regex porque não há
// dependência TOML no go.mod e o objetivo é apenas detectar presença.
func loadCodexTrustedHashes(path string) map[string]bool {
	out := map[string]bool{}
	raw, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	stateHeader := regexp.MustCompile(`(?m)^\[hooks\.state\."([^"]+)"\]\s*$`)
	hashLine := regexp.MustCompile(`(?m)^trusted_hash\s*=\s*"sha256:`)
	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		match := stateHeader.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		key := match[1]
		for j := i + 1; j < len(lines); j++ {
			trimmed := strings.TrimSpace(lines[j])
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "[") {
				break
			}
			if hashLine.MatchString(lines[j]) {
				plugin := extractCodexPluginFromStateKey(key)
				if plugin != "" {
					out[plugin] = true
				}
				break
			}
		}
	}
	return out
}

func extractCodexPluginFromStateKey(key string) string {
	at := strings.Index(key, "@")
	if at <= 0 {
		return ""
	}
	return key[:at]
}

// syncAntigravityHook cobre o formato próprio do Antigravity CLI, sem chave
// "hooks" de topo: {"<nome-do-hook>": {"<Evento>": [{"matcher", "hooks": [...]}]}}.
// Espera um script em disco (hooks/<scriptName>); para um comando literal
// (sem arquivo), use syncAntigravityHookCommand diretamente.
//
// O scriptPath é envelopado via wrap-hook.sh para captura de exit code /
// stderr / duration_ms; o nome lógico do wrapper é o scriptName sem o sufixo
// .sh (ex.: "context-guard-nudge.antigravity"). O stage usado é o
// target.HooksEvent já configurado pelo caller (PostToolUse ou PreToolUse).
func syncAntigravityHook(baseDir string, target TargetCLI, hookName, scriptName, matcher string) error {
	scriptPath := filepath.Join(baseDir, "hooks", scriptName)
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("script do hook não encontrado: %s", scriptPath)
	}
	wrapped := wrapHookCommand(baseDir, target.HooksEvent, strings.TrimSuffix(scriptName, ".sh"), scriptPath)
	return syncAntigravityHookCommand(target, hookName, wrapped, matcher)
}

// syncAntigravityHookCommand é a versão sem exigência de arquivo em disco.
func syncAntigravityHookCommand(target TargetCLI, hookName, command, matcher string) error {
	root, err := readJSONObject(target.HooksSettingsPath)
	if err != nil {
		return err
	}

	hookGroup, _ := root[hookName].(map[string]interface{})
	if hookGroup == nil {
		hookGroup = map[string]interface{}{}
	}

	entries := decodeHookEntries(hookGroup[target.HooksEvent])
	hookGroup[target.HooksEvent] = upsertHookEntry(entries, command, hookName, matcher)
	root[hookName] = hookGroup

	return writeJSONObject(target.HooksSettingsPath, root)
}

// upsertHookEntry remove uma entrada anterior do hook nomeado (se existir) e
// adiciona a versão atual, mantendo entradas de outras origens intactas.
func upsertHookEntry(entries []hookEntry, scriptPath, hookName, matcher string) []hookEntry {
	filtered := entries[:0:0]
	for _, e := range entries {
		if !hasNamedHook(e, hookName) {
			filtered = append(filtered, e)
		}
	}
	filtered = append(filtered, hookEntry{
		Matcher: matcher,
		Hooks: []hookCmd{
			{
				Type:    "command",
				Command: scriptPath,
				Name:    hookName,
				Timeout: 10,
			},
		},
	})
	return filtered
}

func decodeHookEntries(raw interface{}) []hookEntry {
	if raw == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var entries []hookEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		return nil
	}
	return entries
}

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

// syncOpenCodePlugin instala o plugin best-effort de lembrete do context-guard
// para o OpenCode (não é config declarativa: precisa de um plugin TS real).
func syncOpenCodePlugin(baseDir string, target TargetCLI) error {
	if target.OpenCodePluginDir == "" {
		return nil
	}
	src := filepath.Join(baseDir, "hooks", "context-guard-nudge.opencode.ts")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("plugin do hook não encontrado: %s", src)
	}
	return copyFile(src, filepath.Join(target.OpenCodePluginDir, "context-guard-nudge.ts"))
}

// syncOpenCodeMemoryNudgePlugin instala o plugin best-effort de lembrete de
// memory-mcp (store_memory) para o OpenCode.
func syncOpenCodeMemoryNudgePlugin(baseDir string, target TargetCLI) error {
	if target.OpenCodePluginDir == "" {
		return nil
	}
	src := filepath.Join(baseDir, "hooks", "memory-nudge.opencode.ts")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("plugin do hook não encontrado: %s", src)
	}
	return copyFile(src, filepath.Join(target.OpenCodePluginDir, "memory-nudge.ts"))
}

// syncOpenCodeAgentReactNudgePlugin instala o plugin best-effort de lembrete
// de validacao de hipoteses (agent-react) para o OpenCode.
func syncOpenCodeAgentReactNudgePlugin(baseDir string, target TargetCLI) error {
	if target.OpenCodePluginDir == "" {
		return nil
	}
	src := filepath.Join(baseDir, "hooks", "agent-react-nudge.opencode.ts")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("plugin do hook não encontrado: %s", src)
	}
	return copyFile(src, filepath.Join(target.OpenCodePluginDir, "agent-react-nudge.ts"))
}

// syncOpenCodeDocsCachePlugin instala o plugin best-effort que cacheia
// passivamente docs consultadas via webfetch/context7 no OpenCode.
func syncOpenCodeDocsCachePlugin(baseDir string, target TargetCLI) error {
	if target.OpenCodePluginDir == "" {
		return nil
	}
	src := filepath.Join(baseDir, "hooks", "docs-cache.opencode.ts")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("plugin do docs-cache não encontrado: %s", src)
	}
	return copyFile(src, filepath.Join(target.OpenCodePluginDir, "docs-cache.ts"))
}

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
