package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
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

// filterLines remove ruído de bibliotecas/vendor e limita a saída a maxLines.
func filterLines(lines []string, maxLines int) (kept []string, omitted int) {
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

func main() {
	maxLines := flag.Int("max", 25, "Número máximo de linhas de stack trace a exibir")
	flag.Parse()

	var reader io.Reader = os.Stdin
	args := flag.Args()
	if len(args) > 0 {
		f, err := os.Open(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro ao abrir arquivo: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		reader = f
	}

	scanner := bufio.NewScanner(reader)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	keptLines, omittedCount := filterLines(lines, *maxLines)

	fmt.Println("🔍 --- STACK TRACE RELEVANTE (Filtrado para economia de tokens) ---")
	for _, l := range keptLines {
		fmt.Println(l)
	}
	if omittedCount > 0 {
		fmt.Printf("\n⚡ [%d linhas de frameworks/bibliotecas/vendor ocultadas]\n", omittedCount)
	}
}
