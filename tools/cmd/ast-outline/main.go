package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/matheusdutra/token-tools/internal/astoutline"
)

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		fmt.Println("Uso: ast-outline <arquivo>")
		os.Exit(1)
	}

	filePath := args[0]
	out, err := astoutline.Extract(filePath, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	fmt.Println(out)
}
