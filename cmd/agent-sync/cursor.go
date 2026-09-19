package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const cursorRulesFrontmatter = `---
description: Agent-sync global rules (Clean Code, OWASP, anti-hallucination, context-guard)
alwaysApply: true
---

`

// cursorHookDef descreve uma entrada em ~/.cursor/hooks.json gerenciada pelo agent-sync.
type cursorHookDef struct {
	Event   string
	Script  string // nome do arquivo em hooks/ (repo) e em ~/.cursor/hooks/
	Matcher string // opcional
}

func cursorManagedHooks() []cursorHookDef {
	return []cursorHookDef{
		{Event: "postToolUse", Script: "context-guard-nudge.cursor.sh"},
		{Event: "postToolUse", Script: "memory-nudge.cursor.sh"},
		{Event: "postToolUse", Script: "agent-react-nudge.cursor.sh"},
		{Event: "postToolUse", Script: "docs-cache.cursor.sh", Matcher: "WebFetch"},
		{Event: "afterMCPExecution", Script: "docs-cache-mcp.cursor.sh", Matcher: "query-docs"},
		{Event: "beforeShellExecution", Script: "bash-guardian.cursor.sh"},
		{Event: "stop", Script: "agent-stop.cursor.sh"},
	}
}

// syncCursorRules grava as regras globais como .mdc com alwaysApply no ~/.cursor/rules.
func syncCursorRules(src, dst string) error {
	if shouldDryRun() {
		fmt.Printf("[dry-run] write mdc %s\n", dst)
		return nil
	}
	body, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, []byte(cursorRulesFrontmatter+string(body)), 0o644)
}

// syncCursorAll instala scripts em ~/.cursor/hooks/ e faz merge idempotente em hooks.json.
// Também copia o patterns.txt usado pelo bash-guardian e o helper Python do docs-cache.
func syncCursorAll(baseDir string, target TargetCLI) error {
	if target.HooksFormat != "cursor" || target.HooksSettingsPath == "" {
		return nil
	}
	if shouldDryRun() {
		fmt.Printf("[dry-run] cursor hooks/scripts em %s\n", target.HooksSettingsPath)
		return nil
	}

	hooksDir := filepath.Join(filepath.Dir(target.HooksSettingsPath), "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return err
	}

	extras := []string{
		"bash-guardian-patterns.txt",
		"docs-cache.cursor.py",
	}
	for _, name := range extras {
		src := filepath.Join(baseDir, "hooks", name)
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("arquivo auxiliar Cursor não encontrado: %s", src)
		}
		if err := copyFile(src, filepath.Join(hooksDir, name)); err != nil {
			return err
		}
	}

	for _, h := range cursorManagedHooks() {
		src := filepath.Join(baseDir, "hooks", h.Script)
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("script do hook Cursor não encontrado: %s", src)
		}
		dst := filepath.Join(hooksDir, h.Script)
		if err := copyFile(src, dst); err != nil {
			return err
		}
		if err := os.Chmod(dst, 0o755); err != nil {
			return err
		}
	}

	return mergeCursorHooksJSON(target.HooksSettingsPath)
}

func mergeCursorHooksJSON(path string) error {
	root, err := readJSONObject(path)
	if err != nil {
		return err
	}
	if _, ok := root["version"]; !ok {
		root["version"] = 1
	}

	hooksRoot, _ := root["hooks"].(map[string]interface{})
	if hooksRoot == nil {
		hooksRoot = map[string]interface{}{}
	}

	managed := map[string]cursorHookDef{}
	for _, h := range cursorManagedHooks() {
		managed[h.Script] = h
	}

	// Remove entradas antigas do agent-sync (identificadas pelo nome do script).
	for event, raw := range hooksRoot {
		entries := decodeCursorHookEntries(raw)
		kept := entries[:0:0]
		for _, e := range entries {
			if !isCursorManagedCommand(e.Command, managed) {
				kept = append(kept, e)
			}
		}
		if len(kept) == 0 {
			delete(hooksRoot, event)
		} else {
			hooksRoot[event] = encodeCursorHookEntries(kept)
		}
	}

	// Reinsere a versão atual (comandos relativos a ~/.cursor/).
	for _, h := range cursorManagedHooks() {
		cmd := "./hooks/" + h.Script
		list := decodeCursorHookEntries(hooksRoot[h.Event])
		list = append(list, cursorHookEntry{Command: cmd, Matcher: h.Matcher})
		hooksRoot[h.Event] = encodeCursorHookEntries(list)
	}

	root["hooks"] = hooksRoot
	return writeJSONObject(path, root)
}

type cursorHookEntry struct {
	Command string
	Matcher string
	Raw     map[string]interface{}
}

func decodeCursorHookEntries(raw interface{}) []cursorHookEntry {
	if raw == nil {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		b, err := json.Marshal(raw)
		if err != nil {
			return nil
		}
		if err := json.Unmarshal(b, &arr); err != nil {
			return nil
		}
	}
	var out []cursorHookEntry
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		cmd, _ := m["command"].(string)
		matcher, _ := m["matcher"].(string)
		out = append(out, cursorHookEntry{Command: cmd, Matcher: matcher, Raw: m})
	}
	return out
}

func encodeCursorHookEntries(entries []cursorHookEntry) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(entries))
	for _, e := range entries {
		if e.Raw != nil {
			out = append(out, e.Raw)
			continue
		}
		m := map[string]interface{}{"command": e.Command}
		if e.Matcher != "" {
			m["matcher"] = e.Matcher
		}
		out = append(out, m)
	}
	return out
}

func isCursorManagedCommand(command string, managed map[string]cursorHookDef) bool {
	base := filepath.Base(strings.TrimSpace(command))
	_, ok := managed[base]
	return ok
}
