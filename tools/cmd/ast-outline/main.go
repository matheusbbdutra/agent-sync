package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type outlineLine struct {
	Line int
	Text string
}

func (l outlineLine) String() string {
	return fmt.Sprintf("L%-4d: %s", l.Line, l.Text)
}

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

func goOutlineLines(src []byte) ([]outlineLine, string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return nil, "", err
	}

	var out []outlineLine
	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					kind := "type"
					switch typeSpec.Type.(type) {
					case *ast.StructType:
						kind = "struct"
					case *ast.InterfaceType:
						kind = "interface"
					}
					out = append(out, outlineLine{
						Line: fset.Position(typeSpec.Pos()).Line,
						Text: fmt.Sprintf("%s %s", kind, typeSpec.Name.Name),
					})
				}
			}
		case *ast.FuncDecl:
			var recv string
			if d.Recv != nil && len(d.Recv.List) > 0 {
				r := d.Recv.List[0]
				switch t := r.Type.(type) {
				case *ast.StarExpr:
					if id, ok := t.X.(*ast.Ident); ok {
						recv = fmt.Sprintf("(%s) ", "*"+id.Name)
					}
				case *ast.Ident:
					recv = fmt.Sprintf("(%s) ", t.Name)
				}
			}
			out = append(out, outlineLine{
				Line: fset.Position(d.Pos()).Line,
				Text: fmt.Sprintf("func %s%s(...)", recv, d.Name.Name),
			})
		}
	}
	return out, node.Name.Name, nil
}

func outlineGo(filePath string) {
	src, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao abrir arquivo: %v\n", err)
		return
	}

	lines, pkg, err := goOutlineLines(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao analisar Go: %v\n", err)
		outlineFallback(filePath)
		return
	}

	fmt.Printf("📦 Arquivo: %s (Go - Pacote: %s)\n", filePath, pkg)
	for _, l := range lines {
		fmt.Println(l.String())
	}
}

// regexOutline extrai linhas de outline casadas pelo padrão, preservando a numeração.
func regexOutline(r io.Reader, re *regexp.Regexp) []outlineLine {
	var out []outlineLine
	scanner := bufio.NewScanner(r)
	lineNum := 1
	for scanner.Scan() {
		line := scanner.Text()
		if re.MatchString(line) {
			out = append(out, outlineLine{Line: lineNum, Text: strings.TrimSpace(line)})
		}
		lineNum++
	}
	return out
}

func outlineRegex(filePath, pattern string) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao abrir arquivo: %v\n", err)
		return
	}
	defer file.Close()

	out := regexOutline(file, regexp.MustCompile(pattern))

	fmt.Printf("📄 Arquivo: %s\n", filePath)
	for _, l := range out {
		fmt.Println(l.String())
	}
}

func outlineFallback(filePath string) {
	outlineRegex(filePath, `(?i)(class\s+\w+|def\s+\w+|function\s+\w+|struct\s+\w+)`)
}
