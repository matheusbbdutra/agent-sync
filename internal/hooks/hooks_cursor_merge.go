package hooks

import (
	"path/filepath"
	"encoding/json"
	"strings"
)

// hooks_cursor_merge.go: helpers de merge/codificacao de hooks.json do Cursor.
//
// mergeCursorHooksJSON faz merge idempotente preservando hooks nao
// gerenciados. decodeCursorHookEntries/encodeCursorHookEntries sao o
// equivalente Cursor de decodeHookEntries/encodeHookEntries. Migrado
// de cursor.go em 2026-09-21 (Fase 7).


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
		var cmd string
		if h.WrapStage != "" {
			cmd = "./hooks/wrap-hook.sh " + h.WrapStage + " " + h.Script + " " + h.Script
		} else {
			cmd = "./hooks/" + h.Script
		}
		list := decodeCursorHookEntries(hooksRoot[h.Event])
		list = append(list, cursorHookEntry{Command: cmd, Matcher: h.Matcher, LoopLimit: h.LoopLimit})
		hooksRoot[h.Event] = encodeCursorHookEntries(list)
	}

	root["hooks"] = hooksRoot
	return writeJSONObject(path, root)
}

type cursorHookEntry struct {
	Command   string
	Matcher   string
	LoopLimit int
	Raw       map[string]interface{}
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
		var loopLimit int
		switch v := m["loop_limit"].(type) {
		case float64:
			loopLimit = int(v)
		case int:
			loopLimit = v
		}
		out = append(out, cursorHookEntry{Command: cmd, Matcher: matcher, LoopLimit: loopLimit, Raw: m})
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
		if e.LoopLimit > 0 {
			m["loop_limit"] = e.LoopLimit
		}
		out = append(out, m)
	}
	return out
}

func isCursorManagedCommand(command string, managed map[string]cursorHookDef) bool {
	trimmed := strings.TrimSpace(command)
	base := filepath.Base(trimmed)
	if _, ok := managed[base]; ok {
		return true
	}
	for _, h := range managed {
		if h.WrapStage != "" && trimmed == "./hooks/wrap-hook.sh "+h.WrapStage+" "+h.Script+" "+h.Script {
			return true
		}
	}
	return false
}