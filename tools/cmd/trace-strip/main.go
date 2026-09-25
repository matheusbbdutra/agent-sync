package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/matheusdutra/token-tools/internal/tracestrip"
)

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

	result := tracestrip.StripReader(reader, *maxLines)
	fmt.Println(result)
}
