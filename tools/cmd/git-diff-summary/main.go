// Command git-diff-summary resume um git diff unificado em arquivos alterados,
// status ([M]/[A]/[D]) e, por hunk, o cabeçalho de função e a contagem de linhas
// adicionadas (+) e removidas (-). Lê de stdin, de um arquivo de patch ou,
// quando sem argumentos e sem stdin, executa `git diff`.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

const (
	statusModified = "M"
	statusAdded    = "A"
	statusDeleted  = "D"

	devNull     = "/dev/null"
	noHeader    = "(sem cabeçalho)"
	noHunks     = "(sem hunks)"
	noPath      = "(sem nome)"
	scannerInit = 64 * 1024
	scannerMax  = 1024 * 1024
)

var hunkHeaderRe = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@ *(.*)$`)

// HunkSummary descreve um trecho @@ do diff.
type HunkSummary struct {
	Header  string
	Added   int
	Removed int
}

// FileSummary descreve um arquivo alterado e seus hunks.
type FileSummary struct {
	Path   string
	Status string
	Hunks  []HunkSummary
}

type hunkState struct {
	summary      *HunkSummary
	oldRemaining int
	newRemaining int
}

func (h *hunkState) finished() bool {
	return h.oldRemaining <= 0 && h.newRemaining <= 0
}

func newFileSummary(path string) FileSummary {
	return FileSummary{Path: path, Status: statusModified}
}

// ParseDiff lê um diff unificado e devolve o resumo por arquivo.
func ParseDiff(r io.Reader) ([]FileSummary, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, scannerInit), scannerMax)

	var (
		files                []FileSummary
		current              *FileSummary
		hunk                 *hunkState
		awaitingPaths        bool
		srcPrefix, dstPrefix string
	)

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")

		switch {
		case strings.HasPrefix(line, "diff --git "):
			src, dst := splitGitHeaderPaths(line)
			srcPrefix, dstPrefix = pathPrefix(src), pathPrefix(dst)
			files = append(files, newFileSummary(stripPrefix(dst, dstPrefix)))
			current = &files[len(files)-1]
			hunk = nil
			awaitingPaths = true

		case strings.HasPrefix(line, "@@"):
			if current == nil {
				continue
			}
			summary := HunkSummary{}
			current.Hunks = append(current.Hunks, summary)
			hunk = newHunkState(&current.Hunks[len(current.Hunks)-1], line)

		case strings.HasPrefix(line, "--- ") && fileHeaderContext(hunk):
			if current == nil || !awaitingPaths {
				files = append(files, newFileSummary(""))
				current = &files[len(files)-1]
				hunk = nil
				awaitingPaths = true
			}
			applyOldPath(current, line, srcPrefix)

		case strings.HasPrefix(line, "+++ ") && fileHeaderContext(hunk):
			if current != nil {
				applyNewPath(current, line, dstPrefix)
				awaitingPaths = false
			}

		case strings.HasPrefix(line, "+") && hunk != nil:
			hunk.summary.Added++
			hunk.newRemaining--

		case strings.HasPrefix(line, "-") && hunk != nil:
			hunk.summary.Removed++
			hunk.oldRemaining--

		case strings.HasPrefix(line, " ") && hunk != nil:
			hunk.oldRemaining--
			hunk.newRemaining--
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return files, nil
}

func fileHeaderContext(hunk *hunkState) bool {
	return hunk == nil || hunk.finished()
}

func newHunkState(summary *HunkSummary, line string) *hunkState {
	state := &hunkState{summary: summary, oldRemaining: 1, newRemaining: 1}
	match := hunkHeaderRe.FindStringSubmatch(line)
	if match == nil {
		return state
	}
	if match[2] != "" {
		state.oldRemaining = atoi(match[2])
	}
	if match[4] != "" {
		state.newRemaining = atoi(match[4])
	}
	summary.Header = strings.TrimSpace(match[5])
	return state
}

func atoi(value string) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return n
}

// splitGitHeaderPaths extrai os dois caminhos de "diff --git <src> <dst>",
// respeitando caminhos com aspas quando contêm espaços.
func splitGitHeaderPaths(line string) (src, dst string) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "diff --git "))
	src, rest = takeGitPath(rest)
	dst, _ = takeGitPath(rest)
	return src, dst
}

func takeGitPath(text string) (path, rest string) {
	text = strings.TrimLeft(text, " ")
	if text == "" {
		return "", ""
	}
	if text[0] != '"' {
		if idx := strings.IndexByte(text, ' '); idx >= 0 {
			return text[:idx], text[idx+1:]
		}
		return text, ""
	}
	escaped := false
	for i := 1; i < len(text); i++ {
		switch {
		case escaped:
			escaped = false
		case text[i] == '\\':
			escaped = true
		case text[i] == '"':
			raw := text[:i+1]
			unquoted, err := strconv.Unquote(raw)
			if err != nil {
				unquoted = strings.Trim(raw, `"`)
			}
			return unquoted, text[i+1:]
		}
	}
	return text, ""
}

// pathPrefix devolve o diretório-prefixo do git (ex.: "a/", "b/", "i/", "w/").
func pathPrefix(path string) string {
	if idx := strings.IndexByte(path, '/'); idx >= 0 {
		return path[:idx+1]
	}
	return ""
}

func stripPrefix(path, prefix string) string {
	if prefix != "" && strings.HasPrefix(path, prefix) {
		return path[len(prefix):]
	}
	return path
}

func applyOldPath(file *FileSummary, line, prefix string) {
	path := parseHeaderPath(strings.TrimPrefix(line, "--- "), prefix)
	if path == devNull {
		file.Status = statusAdded
		return
	}
	if file.Path == "" {
		file.Path = path
	}
}

func applyNewPath(file *FileSummary, line, prefix string) {
	path := parseHeaderPath(strings.TrimPrefix(line, "+++ "), prefix)
	if path == devNull {
		file.Status = statusDeleted
		return
	}
	file.Path = path
}

// parseHeaderPath extrai o caminho de uma linha "--- "/"+++ ", removendo
// aspas de escape do git e timestamps de patches produzidos por `diff -u`.
func parseHeaderPath(field, prefix string) string {
	field = strings.TrimSpace(field)
	if field == "" {
		return ""
	}
	if strings.HasPrefix(field, `"`) {
		if unquoted, err := strconv.Unquote(field); err == nil {
			field = unquoted
		} else {
			field = strings.Trim(field, `"`)
		}
	} else if idx := strings.IndexByte(field, '\t'); idx >= 0 {
		field = field[:idx]
	}
	return stripPrefix(field, prefix)
}

// FormatSummary renderiza o resumo em texto compacto.
func FormatSummary(files []FileSummary) string {
	var out strings.Builder
	for _, file := range files {
		path := file.Path
		if path == "" {
			path = noPath
		}
		fmt.Fprintf(&out, "[%s] %s\n", file.Status, path)
		if len(file.Hunks) == 0 {
			fmt.Fprintf(&out, "  %s\n", noHunks)
			continue
		}
		for _, hunk := range file.Hunks {
			header := hunk.Header
			if header == "" {
				header = noHeader
			}
			fmt.Fprintf(&out, "  @@ %s (+%d/-%d)\n", header, hunk.Added, hunk.Removed)
		}
	}
	return out.String()
}

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Uso: git-diff-summary [arquivo.patch]")
		fmt.Fprintln(os.Stderr, "  sem argumentos lê de stdin (se houver pipe) ou executa `git diff`")
		flag.PrintDefaults()
	}
	flag.Parse()

	if err := run(flag.Args(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "erro: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdin *os.File, stdout io.Writer) error {
	reader, closer, err := openInput(args, stdin)
	if err != nil {
		return err
	}
	defer closer()

	files, err := ParseDiff(reader)
	if err != nil {
		return fmt.Errorf("processar diff: %w", err)
	}
	_, err = io.WriteString(stdout, FormatSummary(files))
	return err
}

func openInput(args []string, stdin *os.File) (io.Reader, func(), error) {
	if len(args) > 0 {
		file, err := os.Open(args[0])
		if err != nil {
			return nil, nil, fmt.Errorf("abrir patch %q: %w", args[0], err)
		}
		return file, func() { _ = file.Close() }, nil
	}

	if stdin != nil && !stdinIsTerminal(stdin) {
		return stdin, func() {}, nil
	}

	output, err := exec.Command("git", "diff").Output()
	if err != nil {
		return nil, nil, fmt.Errorf("executar git diff: %w", err)
	}
	return bytes.NewReader(output), func() {}, nil
}

func stdinIsTerminal(stdin *os.File) bool {
	info, err := stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
