// cmdcache_cli.go — superfície de CLI do estágio overwrite+cache (§1 da
// ADR-reuso-bash-sudo-agents), usada pelos adapters de hook e por inspeção
// manual (`shell-validate cache-get --cmd "uname -a"`).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
)

// runCacheGet imprime a saída cacheada do comando (se houve hit dentro do TTL).
// Exit 0 = hit (saída no stdout); erro = miss (nenhum cache).
func runCacheGet(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("cache-get", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cmdFlag := fs.String("cmd", "", "comando consultado no cache")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cmd := *cmdFlag
	if cmd == "" {
		raw, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("cache-get: read stdin: %w", err)
		}
		cmd = string(raw)
	}
	policy, ok := classifyCacheable(cmd)
	if !ok {
		return fmt.Errorf("cache-get: comando fora da allowlist: %q", strings.TrimSpace(cmd))
	}
	out, age, hit := cmdCacheGet(policy, cmd)
	if !hit {
		return fmt.Errorf("cache-get: miss (%s, ttl=%ds)", policy.kind, int(policy.ttl.Seconds()))
	}
	fmt.Fprintf(stderr, "cache-get: hit (%s, idade=%ds)\n", policy.kind, int(age.Seconds()))
	fmt.Fprint(stdout, out)
	return nil
}

// runCachePut grava a saída de um comando allowlistado. Dois modos:
//
//	cache-put --cmd "<cmd>"      texto do stdout vem do stdin
//	cache-put --from-hook        payload PostToolUse (Claude/Codex) no stdin
func runCachePut(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("cache-put", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cmdFlag := fs.String("cmd", "", "comando cujo stdout será cacheado")
	fromHook := fs.Bool("from-hook", false, "lê payload PostToolUse do stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	raw, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("cache-put: read stdin: %w", err)
	}
	cmd := strings.TrimSpace(*cmdFlag)
	out := string(raw)
	if *fromHook {
		cmd, out, err = extractPostToolUse(raw)
		if err != nil {
			return err
		}
	}
	policy, ok := classifyCacheable(cmd)
	if !ok {
		return fmt.Errorf("cache-put: comando fora da allowlist: %q", cmd)
	}
	if err := cmdCachePut(policy, cmd, out); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "cache-put: %s (%s, %d bytes)\n", "ok", policy.kind, len(out))
	return nil
}

// extractPostToolUse tira (comando, stdout) de um payload PostToolUse de
// Claude/Codex: {tool_name, tool_input:{command}, tool_response}. O
// tool_response pode ser string ou objeto; quando objeto, tentamos
// stdout/output/content/text/result (mesma ordem do docs-cache-write).
func extractPostToolUse(raw []byte) (string, string, error) {
	var payload struct {
		ToolName     string          `json:"tool_name"`
		ToolInput    json.RawMessage `json:"tool_input"`
		ToolResponse json.RawMessage `json:"tool_response"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", "", fmt.Errorf("cache-put: payload PostToolUse inválido: %w", err)
	}
	cmd, ok := extractBashCommand(preToolUsePayload{ToolName: payload.ToolName, ToolInput: payload.ToolInput})
	if !ok {
		return "", "", fmt.Errorf("cache-put: payload sem comando Bash")
	}
	return cmd, hookOutputText(payload.ToolResponse), nil
}

// hookOutputText achata o tool_response em texto.
func hookOutputText(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		for _, k := range []string{"stdout", "output", "content", "text", "result"} {
			if v, ok := obj[k]; ok {
				if err := json.Unmarshal(v, &s); err == nil && s != "" {
					return s
				}
			}
		}
	}
	return ""
}
