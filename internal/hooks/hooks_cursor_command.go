package hooks

// hooks_cursor_command.go: Cursor command at event especifico.

// Migrado de hooks.go em 2026-09-21 (Fase 2.3 do refator por feature).
// Sem mudanca de comportamento: mesma logica, mesmo package.

// syncCursorCommandAtEvent: Cursor command at event especifico
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
