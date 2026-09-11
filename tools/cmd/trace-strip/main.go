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
	var keptLines []string
	omittedCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			continue
		}

		shouldIgnore := false
		for _, re := range ignoredPatterns {
			if re.MatchString(line) {
				shouldIgnore = true
				break
			}
		}

		if shouldIgnore {
			omittedCount++
			continue
		}

		keptLines = append(keptLines, line)
		if len(keptLines) >= *maxLines {
			break
		}
	}

	fmt.Println("🔍 --- STACK TRACE RELEVANTE (Filtrado para economia de tokens) ---")
	for _, l := range keptLines {
		fmt.Println(l)
	}
	if omittedCount > 0 {
		fmt.Printf("\n⚡ [%d linhas de frameworks/bibliotecas/vendor ocultadas]\n", omittedCount)
	}
}
