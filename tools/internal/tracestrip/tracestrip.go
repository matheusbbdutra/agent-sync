package tracestrip

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var ignoredPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(node_modules|vendor\/|site-packages|\/usr\/local\/go|\/lib\/python)`),
	regexp.MustCompile(`(?i)(internal\/process|runtime\/asm_|runtime\/proc\.go)`),
	regexp.MustCompile(`(?i)(org\.springframework|jakarta\.|javax\.)`),
}

func shouldIgnore(line string) bool {
	for _, re := range ignoredPatterns {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

// FilterLines remove ruído de bibliotecas/vendor e limita a saída a maxLines.
func FilterLines(lines []string, maxLines int) (kept []string, omitted int) {
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if shouldIgnore(line) {
			omitted++
			continue
		}
		kept = append(kept, line)
		if len(kept) >= maxLines {
			break
		}
	}
	return kept, omitted
}

// StripText processa o texto bruto de stack trace e formata o resultado compacto.
func StripText(text string, maxLines int) string {
	return StripReader(strings.NewReader(text), maxLines)
}

// StripReader lê de um io.Reader e formata o resultado compacto.
func StripReader(r io.Reader, maxLines int) string {
	scanner := bufio.NewScanner(r)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	keptLines, omittedCount := FilterLines(lines, maxLines)

	var b strings.Builder
	b.WriteString("🔍 --- STACK TRACE RELEVANTE (Filtrado para economia de tokens) ---\n")
	for _, l := range keptLines {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	if omittedCount > 0 {
		fmt.Fprintf(&b, "\n⚡ [%d linhas de frameworks/bibliotecas/vendor ocultadas]\n", omittedCount)
	}
	return strings.TrimRight(b.String(), "\n")
}
