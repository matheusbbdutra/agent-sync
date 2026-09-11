package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		fmt.Println("Uso: ast-outline <arquivo>")
		os.Exit(1)
	}

	filePath := args[0]
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".go":
		outlineGo(filePath)
	case ".py":
		outlineRegex(filePath, `(?m)^(?:class\s+\w+|def\s+\w+[\w\s,=*()]*:)`)
	case ".js", ".ts", ".jsx", ".tsx":
		outlineRegex(filePath, `(?m)^(?:export\s+)?(?:default\s+)?(?:class\s+\w+|function\s+\w+|interface\s+\w+|type\s+\w+|const\s+\w+\s*=\s*(?:async\s*)?\([^)]*\)\s*=>)`)
	case ".php":
		outlineRegex(filePath, `(?m)^\s*(?:(?:final|abstract|readonly)\s+)?(?:class|interface|trait|enum)\s+\w+|^\s*(?:public|protected|private)?\s*(?:static)?\s*function\s+\w+`)
	default:
		outlineFallback(filePath)
	}
}

func outlineGo(filePath string) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao analisar Go: %v\n", err)
		outlineFallback(filePath)
		return
	}

	fmt.Printf("📦 Arquivo: %s (Go - Pacote: %s)\n", filePath, node.Name.Name)

	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					pos := fset.Position(typeSpec.Pos())
					kind := "type"
					switch typeSpec.Type.(type) {
					case *ast.StructType:
						kind = "struct"
					case *ast.InterfaceType:
						kind = "interface"
					}
					fmt.Printf("L%-4d: %s %s\n", pos.Line, kind, typeSpec.Name.Name)
				}
			}
		case *ast.FuncDecl:
			pos := fset.Position(d.Pos())
			var recv string
			if d.Recv != nil && len(d.Recv.List) > 0 {
				r := d.Recv.List[0]
				recvType := ""
				if star, ok := r.Type.(*ast.StarExpr); ok {
					if id, ok := star.X.(*ast.Ident); ok {
						recvType = "*" + id.Name
					}
				} else if id, ok := r.Type.(*ast.Ident); ok {
					recvType = id.Name
				}
				recv = fmt.Sprintf("(%s) ", recvType)
			}
			fmt.Printf("L%-4d: func %s%s(...)\n", pos.Line, recv, d.Name.Name)
		}
	}
}

func outlineRegex(filePath, pattern string) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao abrir arquivo: %v\n", err)
		return
	}
	defer file.Close()

	re := regexp.MustCompile(pattern)
	scanner := bufio.NewScanner(file)
	lineNum := 1

	fmt.Printf("📄 Arquivo: %s\n", filePath)
	for scanner.Scan() {
		line := scanner.Text()
		if re.MatchString(line) {
			fmt.Printf("L%-4d: %s\n", lineNum, strings.TrimSpace(line))
		}
		lineNum++
	}
}

func outlineFallback(filePath string) {
	outlineRegex(filePath, `(?i)(class\s+\w+|def\s+\w+|function\s+\w+|struct\s+\w+)`)
}
