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
// Inspirado no `bash_compress.py` do token-optimizer externo e nas 12
// estratégias do rtk (STATS EXTRACTION, FAILURE FOCUS, NDJSON STREAMING,
// PROGRESS FILTERING, etc.). Whitelist expandida em v2 para incluir
// comandos dev comuns (git/go/cargo/pytest/npm/pnpm/docker), mas com
// UNSAFE PATTERNS mais agressivos (qualquer pipe, redirecionamento, sudo,
// ou flag destrutiva = fail-open).
var bashSafeCommands = map[string]bool{
	"ls": true, "cat": true, "head": true, "tail": true,
	"find": true, "grep": true, "egrep": true, "fgrep": true,
	"wc": true, "file": true, "stat": true, "du": true, "df": true,
	"tree": true, "pwd": true, "whoami": true, "date": true,
	"echo": true, "printf": true, "true": true, "false": true,
	// v2 additions — dev tooling
	"git": true, "go": true, "cargo": true, "rustc": true,
	"pytest": true, "python": true, "python3": true,
	"docker": true, "kubectl": true, "podman": true,
	"npm": true, "pnpm": true, "yarn": true,
	"make": true, "cmake": true,
}

// bashUnsafePatterns são indicadores de comandos que NUNCA devem ser
// comprimidos, mesmo que o binário base esteja na whitelist. Avaliados
// no comando completo (não só no argv[0]).
//
// v2: removidos itens que estão na whitelist (git/go/cargo/npm/yarn/pnpm/
// python/python3/pytest/docker/kubectl/make/cmake) — caso contrário,
// unsafe_patterns vence e fail-open sempre. Esses comandos têm compressor
// próprio e seguro.
var bashUnsafePatterns = []string{
	"|", "&&", "||", ";", ">", "<", "$", "`",
	"sudo", "rm ", "mv ", "cp ", "chmod", "chown",
	"curl", "wget", "ssh", "scp", "rsync",
	"pip", "pip3", // download/install packages (state-changing)
	"terraform", "ansible", // infrastructure mutations
	"node", "ruby", "php", "perl", // scripts genéricos (comportamento imprevisível)
	"bash ", "sh ", "zsh ", "fish ",
	"tee", "xargs", "exec", "eval", "source",
}

// BashCompressResult agrega o que CompressBashOutput fez.
type BashCompressResult struct {
	Cmd          string `json:"cmd"`
	OrigBytes    int    `json:"orig_bytes"`
	OutBytes     int    `json:"out_bytes"`
	ReductionPct int    `json:"reduction_pct"`
	Applied      bool   `json:"applied"` // false = passou verbatim (whitelist ou unsafe)
	Reason       string `json:"reason"`  // human-readable
	Output       string `json:"output"`
}

const (
	bashKeepHead = 5
	bashKeepTail = 3
	// bashCompressMinBytes é o threshold mínimo para compressores
	// especializados (git/go/cargo/pytest/npm/docker) aplicarem. Outputs
	// menores passam verbatim — adicionar header tipo "[go test] ..."
	// para 30 chars cria overhead > benefício.
	bashCompressMinBytes = 200
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
// genéricos (head/tail) usam headTail. ls usa listTrim. Para git/go/cargo/
// pytest/npm/pnpm, detecta subcommand e despacha para compressor
// especializado (v2: inspirado nas 12 estratégias do rtk).
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
	case "git":
		return compressGit(output)
	case "go":
		return compressGoTest(output)
	case "cargo":
		return compressCargoTest(output)
	case "pytest", "python", "python3":
		return compressPytest(output)
	case "npm", "pnpm", "yarn":
		return compressProgressFilter(output)
	case "docker", "kubectl", "podman":
		return compressStructureOnly(output)
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

// compressGit despacha para compressor git específico baseado no subcommand.
// Inspirado em rtk STATS EXTRACTION (git status → "3 files, +142/-89").
func compressGit(output string) string {
	if len(output) < bashCompressMinBytes {
		return output // outputs curtos já são concisos
	}
	lines := strings.Split(output, "\n")
	// heurística simples: primeira linha relevante identifica o subcommand
	joined := strings.ToLower(strings.Join(lines[:min(5, len(lines))], " "))
	switch {
	case strings.Contains(joined, "changes not staged") || strings.Contains(joined, "modified:") || strings.Contains(joined, "untracked") || strings.Contains(joined, "no changes added"):
		return compressGitStatus(output)
	case strings.Contains(joined, "commit ") || strings.Contains(joined, "author:") || strings.Contains(joined, "date:"):
		return compressGitLog(output)
	default:
		return headTail(output) // fallback para outros git subcommands
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// compressGitStatus extrai contagem: modified/added/deleted/untracked.
// Output típico: "On branch X\nChanges not staged for commit:\n  modified: a\n...".
func compressGitStatus(output string) string {
	counts := map[string]int{
		"modified":  0,
		"added":     0,
		"deleted":   0,
		"renamed":   0,
		"untracked": 0,
	}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "modified:"):
			counts["modified"]++
		case strings.Contains(lower, "deleted:") || strings.Contains(lower, "deleted ") && !strings.Contains(lower, "no deletions"):
			counts["deleted"]++
		case strings.Contains(lower, "renamed:"):
			counts["renamed"]++
		case strings.HasPrefix(strings.TrimSpace(lower), "??") || strings.Contains(lower, "untracked"):
			counts["untracked"]++
		}
	}
	total := 0
	for _, v := range counts {
		total += v
	}
	if total == 0 {
		return output // git status clean ou formato diferente
	}
	var b strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "On branch ") {
			b.WriteString(trimmed)
			b.WriteByte('\n')
			break
		}
	}
	fmt.Fprintf(&b, "[git status] %d changes:", total)
	for _, k := range []string{"modified", "added", "deleted", "renamed", "untracked"} {
		if counts[k] > 0 {
			fmt.Fprintf(&b, " %s=%d", k, counts[k])
		}
	}
	b.WriteByte('\n')
	return strings.TrimRight(b.String(), "\n")
}

// compressGitLog mantém apenas primeira linha de cada commit (hash + título).
func compressGitLog(output string) string {
	var b strings.Builder
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "commit ") {
			b.WriteString(trimmed)
			b.WriteByte('\n')
		}
	}
	if b.Len() == 0 {
		return headTail(output)
	}
	return strings.TrimRight(b.String(), "\n")
}

// compressGoTest processa output de `go test` (NDJSON-like ou text).
// Estratégia: conta PASS/FAIL/SKIP por package, lista apenas falhas.
// Inspirado em rtk STATE MACHINE PARSING + NDJSON STREAMING.
func compressGoTest(output string) string {
	if len(output) < bashCompressMinBytes {
		return output
	}
	var pass, fail, skip, pkg int
	var failures []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "ok "):
			pkg++
		case strings.HasPrefix(trimmed, "FAIL"):
			pkg++
			fail++
			failures = append(failures, trimmed)
		case strings.HasPrefix(trimmed, "PASS"):
			pkg++
			pass++
		case strings.HasPrefix(trimmed, "SKIP"):
			skip++
		case strings.Contains(trimmed, "--- FAIL:"):
			failures = append(failures, trimmed)
		}
	}
	if pkg == 0 {
		return headTail(output) // não parece ser output de go test
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[go test] packages=%d pass=%d fail=%d skip=%d\n", pkg, pass, fail, skip)
	for _, f := range failures {
		b.WriteString(f)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// compressCargoTest processa output de `cargo test` (similar ao go test mas
// formato diferente: "test result: ok/FAILED" + contagens).
func compressCargoTest(output string) string {
	if len(output) < bashCompressMinBytes {
		return output
	}
	var pass, fail int
	var failures []string
	lines := strings.Split(output, "\n")
	inFailures := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "test result:") {
			if strings.Contains(trimmed, "ok") {
				pass++
			} else if strings.Contains(trimmed, "FAILED") {
				fail++
			}
		}
		if strings.HasPrefix(trimmed, "failures:") {
			inFailures = true
			continue
		}
		if strings.HasPrefix(trimmed, "test result:") {
			inFailures = false
		}
		if inFailures && trimmed != "" {
			failures = append(failures, trimmed)
		}
	}
	if pass+fail == 0 {
		return headTail(output)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[cargo test] suites pass=%d fail=%d\n", pass, fail)
	for _, f := range failures {
		b.WriteString(f)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// compressPytest processa output de `pytest`. Estratégia: conta passed/failed/
// errors/skipped + extrai apenas linhas com "FAILED <test>".
func compressPytest(output string) string {
	if len(output) < bashCompressMinBytes {
		return output
	}
	var passed, failed, errors, skipped int
	var failedTests []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// formato "= 5 passed, 1 failed in 0.5s ="
		if strings.Contains(trimmed, "passed") || strings.Contains(trimmed, "failed") ||
			strings.Contains(trimmed, "error") || strings.Contains(trimmed, "skipped") {
			lower := strings.ToLower(trimmed)
			passed += strings.Count(lower, "passed")
			failed += strings.Count(lower, "failed")
			errors += strings.Count(lower, "error")
			skipped += strings.Count(lower, "skipped")
		}
		// formato "FAILED tests/test_foo.py::test_bar"
		if strings.HasPrefix(trimmed, "FAILED ") && strings.Contains(trimmed, "::") {
			failedTests = append(failedTests, trimmed)
		}
	}
	total := passed + failed + errors + skipped
	if total == 0 {
		return headTail(output)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[pytest] passed=%d failed=%d errors=%d skipped=%d\n", passed, failed, errors, skipped)
	for _, t := range failedTests {
		b.WriteString(t)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// compressProgressFilter strip ANSI escape codes + progresso de output de
// npm/pnpm/yarn install. Mantém apenas as linhas com palavras-chave relevantes.
// Inspirado em rtk PROGRESS FILTERING.
func compressProgressFilter(output string) string {
	if len(output) < bashCompressMinBytes {
		return output
	}
	lines := strings.Split(output, "\n")
	var b strings.Builder
	kept := 0
	for _, line := range lines {
		// strip ANSI escape codes (regex-free, byte-level)
		cleaned := stripANSI(line)
		trimmed := strings.TrimSpace(cleaned)
		if trimmed == "" {
			continue
		}
		// mantém só linhas com keywords de status
		lower := strings.ToLower(trimmed)
		isStatus := strings.Contains(lower, "added") ||
			strings.Contains(lower, "removed") ||
			strings.Contains(lower, "changed") ||
			strings.Contains(lower, "warn") ||
			strings.Contains(lower, "error") ||
			strings.Contains(lower, "found ") ||
			strings.Contains(lower, "up to date") ||
			strings.HasPrefix(trimmed, "+") ||
			strings.HasPrefix(trimmed, "-")
		if !isStatus {
			continue
		}
		b.WriteString(trimmed)
		b.WriteByte('\n')
		kept++
	}
	if kept == 0 {
		// nenhum status identificável — fallback para última linha (resumo final)
		for i := len(lines) - 1; i >= 0; i-- {
			t := strings.TrimSpace(stripANSI(lines[i]))
			if t != "" {
				return t + "\n[... install progress stripped ...]"
			}
		}
		return output
	}
	return strings.TrimRight(b.String(), "\n")
}

// stripANSI remove códigos ANSI escape (cores, cursor moves).
// Implementação simples: pula ESC[...m sequences.
func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			// skip até 'm' (color) ou 'A-Z' (cursor)
			j := i + 2
			for j < len(s) {
				c := s[j]
				if c >= 0x40 && c <= 0x7e {
					j++
					break
				}
				j++
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// compressStructureOnly mantém apenas chaves de JSON-like (estrutura sem valores).
// Inspirado em rtk STRUCTURE ONLY.
func compressStructureOnly(output string) string {
	if len(output) < bashCompressMinBytes {
		return output
	}
	var b strings.Builder
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// mantém só linhas que começam com chave JSON ou ID-like (sem timestamps longos)
		if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") &&
			!strings.Contains(trimmed, " ID ") && !strings.HasPrefix(trimmed, "NAME") {
			continue
		}
		// strip tudo após primeiro ':' se for JSON-like
		if idx := strings.Index(trimmed, ":"); idx > 0 && (strings.HasPrefix(trimmed, "{") || strings.Contains(trimmed, "\"")) {
			b.WriteString(trimmed[:idx+1])
			b.WriteString("...")
			b.WriteByte('\n')
			continue
		}
		b.WriteString(trimmed)
		b.WriteByte('\n')
	}
	if b.Len() == 0 {
		return headTail(output)
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
