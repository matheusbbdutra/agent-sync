package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// bashSafeCommands é a whitelist de comandos read-only/idempotentes cuja
// saída pode ser comprimida sem risco de alucinação. Qualquer outro comando
// (incluindo npm, cargo, go test, docker, kubectl, git, qualquer pipe `|`
// ou redirecionamento `>` `<`) passa verbatim — fail-open.
//
// Inspirado no `bash_compress.py` do token-optimizer externo, mas com
// whitelist muito mais conservadora: preferimos errar pelo lado da
// segurança (não comprimir) do que pelo lado da economia (comprimir
// output crítico).
var bashSafeCommands = map[string]bool{
	"ls": true, "cat": true, "head": true, "tail": true,
	"find": true, "grep": true, "egrep": true, "fgrep": true,
	"wc": true, "file": true, "stat": true, "du": true, "df": true,
	"tree": true, "pwd": true, "whoami": true, "date": true,
	"echo": true, "printf": true, "true": true, "false": true,
}

// bashUnsafePatterns são indicadores de comandos que NUNCA devem ser
// comprimidos, mesmo que o binário base esteja na whitelist. Avaliados
// no comando completo (não só no argv[0]).
var bashUnsafePatterns = []string{
	"|", "&&", "||", ";", ">", "<", "$", "`",
	"sudo", "rm ", "mv ", "cp ", "chmod", "chown",
	"curl", "wget", "ssh", "scp", "rsync",
	"npm", "yarn", "pnpm", "pip", "pip3", "cargo", "go ",
	"docker", "kubectl", "terraform", "ansible", "make", "cmake",
	"git", "python", "python3", "node", "ruby", "php", "perl",
	"bash ", "sh ", "zsh ", "fish ",
	"tee", "xargs", "exec", "eval", "source",
}

// BashCompressResult agrega o que CompressBashOutput fez.
type BashCompressResult struct {
	Cmd        string `json:"cmd"`
	OrigBytes  int    `json:"orig_bytes"`
	OutBytes   int    `json:"out_bytes"`
	ReductionPct int  `json:"reduction_pct"`
	Applied    bool   `json:"applied"` // false = passou verbatim (whitelist ou unsafe)
	Reason     string `json:"reason"`  // human-readable
	Output     string `json:"output"`
}

const (
	bashKeepHead = 5
	bashKeepTail = 3
)

// CompressBashOutput decide se `output` pode ser comprimido e aplica o
// compressor específico do comando. Função pura, sem side effects.
//
// Política: a compressão só é aplicada se (1) `cmd` é seguro pela
// whitelist E (2) o comando NÃO contém padrões unsafe (pipelines,
// redirecionamentos, downloads, etc.). Caso contrário, devolve
// output verbatim com Applied=false.
func CompressBashOutput(cmd, output string) BashCompressResult {
	res := BashCompressResult{Cmd: cmd, OrigBytes: len(output), Output: output}
	bin, ok := bashBin(cmd)
	if !ok {
		res.Applied = false
		res.Reason = "no whitelist match (fail-open: passes verbatim)"
		res.OutBytes = len(output)
		return res
	}
	if reason := bashUnsafeReason(cmd); reason != "" {
		res.Applied = false
		res.Reason = "unsafe pattern: " + reason
		res.OutBytes = len(output)
		return res
	}
	// comprime
	compressed := compressByCommand(bin, output)
	res.Output = compressed
	res.OutBytes = len(compressed)
	res.Applied = len(compressed) < len(output)
	if !res.Applied {
		res.Reason = "compressor returned same length (no benefit)"
	} else {
		res.Reason = "compressed via " + bin
	}
	if res.OrigBytes > 0 {
		res.ReductionPct = (res.OrigBytes - res.OutBytes) * 100 / res.OrigBytes
	}
	return res
}

// bashBin devolve o argv[0] (binário base) de um comando.
func bashBin(cmd string) (string, bool) {
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return "", false
	}
	// pode ter path absoluto ou relativo
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return "", false
	}
	base := fields[0]
	// basename
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	_, ok := bashSafeCommands[base]
	return base, ok
}

// bashUnsafeReason devolve "" se cmd é seguro; caso contrário, o primeiro
// padrão unsafe encontrado.
func bashUnsafeReason(cmd string) string {
	lower := strings.ToLower(cmd)
	for _, p := range bashUnsafePatterns {
		if strings.Contains(lower, p) {
			return p
		}
	}
	return ""
}

// compressByCommand despacha para o compressor do binário. Compressores
// genéricos (head/tail) usam headTail. ls usa listTrim.
func compressByCommand(bin, output string) string {
	switch bin {
	case "ls":
		return compressList(output)
	case "cat", "head", "tail":
		return headTail(output)
	case "find":
		return compressFind(output)
	case "grep", "egrep", "fgrep":
		return compressGrep(output)
	default:
		return headTail(output) // fallback conservador para outros safe bins
	}
}

// compressList remove linhas com `.`, `..`, totals e mantém entries.
func compressList(output string) string {
	lines := strings.Split(output, "\n")
	var kept []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		// pula entradas triviais
		if strings.HasPrefix(trimmed, "total ") {
			continue
		}
		kept = append(kept, l)
	}
	if len(kept) == len(lines) {
		return output // nada a cortar
	}
	return strings.Join(kept, "\n")
}

// headTail mantém primeiras bashKeepHead e últimas bashKeepTail linhas
// não-vazias, omite o meio com marcador "[... N lines omitted ...]".
func headTail(output string) string {
	lines := strings.Split(output, "\n")
	n := len(lines)
	if n <= bashKeepHead+bashKeepTail+1 {
		return output
	}
	head := lines[:bashKeepHead]
	tail := lines[n-bashKeepTail:]
	omitted := n - bashKeepHead - bashKeepTail
	var b strings.Builder
	for _, l := range head {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "\n[... %d lines omitted ...]\n\n", omitted)
	for _, l := range tail {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// compressFind remove entradas em diretórios de sistema e mantém só path + type.
func compressFind(output string) string {
	lines := strings.Split(output, "\n")
	if len(lines) <= bashKeepHead+bashKeepTail+1 {
		return output
	}
	// mantém só primeiros/últimos paths; omite o meio
	return headTail(output)
}

// compressGrep omite matches intermediários quando output tem muitas linhas.
func compressGrep(output string) string {
	lines := strings.Split(output, "\n")
	n := len(lines)
	if n <= bashKeepHead+bashKeepTail+1 {
		return output
	}
	head := lines[:bashKeepHead]
	tail := lines[n-bashKeepTail:]
	omitted := n - bashKeepHead - bashKeepTail
	var b strings.Builder
	for _, l := range head {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "\n[... %d matching lines omitted ...]\n\n", omitted)
	for _, l := range tail {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// Write imprime o BashCompressResult em plain ou JSON.
func (r *BashCompressResult) Write(w io.Writer, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}
	fmt.Fprintf(w, "bash-compress: %s\n", r.Cmd)
	fmt.Fprintf(w, "  applied:     %v\n", r.Applied)
	fmt.Fprintf(w, "  reason:      %s\n", r.Reason)
	fmt.Fprintf(w, "  reduction:   %d → %d bytes (%d%%)\n", r.OrigBytes, r.OutBytes, r.ReductionPct)
	fmt.Fprintln(w, "\n--- output ---")
	fmt.Fprintln(w, r.Output)
	return nil
}