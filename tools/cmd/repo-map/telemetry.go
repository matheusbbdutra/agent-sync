package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// GraphTelemetryEntry modela cada linha do log append-only JSONL de telemetria.
type GraphTelemetryEntry struct {
	TS                string `json:"ts"`
	SessionID         string `json:"session_id"`
	CLI               string `json:"cli"`
	ToolName          string `json:"tool_name"`
	ArgsPathOrSymbol  string `json:"args_path_or_symbol"`
	DurationMs        int64  `json:"duration_ms"`
	OutputBytes       int    `json:"output_bytes"`
	CacheHit          bool   `json:"cache_hit"`
}

var telemetryMu sync.Mutex

// resolveTelemetryFile determina onde gravar os eventos de telemetria.
// Retorna "" se a telemetria estiver desligada.
func resolveTelemetryFile(cfgTelemetryFile, cacheDir string) string {
	if cfgTelemetryFile != "" {
		return cfgTelemetryFile
	}
	if envPath := strings.TrimSpace(os.Getenv("AGENT_SYNC_GRAPH_TELEMETRY_FILE")); envPath != "" {
		return envPath
	}
	if optIn := strings.TrimSpace(os.Getenv("AGENT_SYNC_GRAPH_TELEMETRY")); optIn == "1" || strings.EqualFold(optIn, "true") {
		if ucd, err := os.UserCacheDir(); err == nil && ucd != "" {
			return filepath.Join(ucd, "agent-sync", "graph_telemetry.jsonl")
		}
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, ".cache", "agent-sync", "graph_telemetry.jsonl")
		}
		return filepath.Join(cacheDir, "graph_telemetry.jsonl")
	}
	return ""
}

// detectCLI tenta inferir qual CLI está invocando o MCP server.
func detectCLI() string {
	if v := strings.TrimSpace(os.Getenv("AGENT_SYNC_AGENT_KIND")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("AGENT_SYNC_CLI")); v != "" {
		return v
	}
	// Heurísticas por variáveis comuns de ambiente
	if os.Getenv("CLAUDE_CODE_ENTRYPOINT") != "" || os.Getenv("CLAUDE_PROJECT_DIR") != "" {
		return "claude"
	}
	if os.Getenv("CURSOR_PROJECT_DIR") != "" || os.Getenv("CURSOR_AGENT") != "" {
		return "cursor"
	}
	if os.Getenv("OPENCODE_VERSION") != "" || os.Getenv("OPENCODE_ROOT") != "" {
		return "opencode"
	}
	if os.Getenv("GEMINI_CLI") != "" || os.Getenv("ANTIGRAVITY_AGENT") != "" {
		return "antigravity"
	}
	return "unknown"
}

// detectSessionID tenta inferir o identificador da sessão atual.
func detectSessionID() string {
	if v := strings.TrimSpace(os.Getenv("AGENT_SYNC_SESSION_ID")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("SESSION_ID")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("CLAUDE_SESSION_ID")); v != "" {
		return v
	}
	return "unknown"
}

// extractTarget extrai o argumento principal (path ou symbol) para a telemetria.
func extractTarget(toolName string, args json.RawMessage) string {
	if len(args) == 0 {
		return ""
	}
	switch toolName {
	case "get_file_impact":
		var in struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(args, &in); err == nil {
			return in.Path
		}
	case "get_symbol_callers":
		var in struct {
			Symbol string `json:"symbol"`
		}
		if err := json.Unmarshal(args, &in); err == nil {
			return in.Symbol
		}
	case "repo_summary":
		return "(all)"
	}
	return ""
}

// recordTelemetry grava a invocação no arquivo JSONL de forma não-bloqueante/segura.
func recordTelemetry(targetFile string, entry GraphTelemetryEntry) {
	if targetFile == "" {
		return
	}

	telemetryMu.Lock()
	defer telemetryMu.Unlock()

	dir := filepath.Dir(targetFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "repo-map telemetry: erro ao criar diretorio %s: %v\n", dir, err)
		return
	}

	f, err := os.OpenFile(targetFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "repo-map telemetry: erro ao abrir %s: %v\n", targetFile, err)
		return
	}
	defer f.Close()

	if entry.TS == "" {
		entry.TS = time.Now().UTC().Format(time.RFC3339)
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	_, _ = f.Write(append(data, '\n'))
}
