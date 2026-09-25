package astoutline

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Extract analisa o arquivo ou conteudo e retorna o esqueleto em texto.
func Extract(filePath string, src []byte) (string, error) {
	if src == nil {
		var err error
		src, err = os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("erro ao abrir arquivo: %w", err)
		}
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return outlineGo(filePath, src)
	case ".py":
		return outlineRegex(filePath, src, `(?m)^(?:class\s+\w+|def\s+\w+[\w\s,=*()]*:)`)
	case ".js", ".ts", ".jsx", ".tsx":
		return outlineRegex(filePath, src, `(?m)^(?:export\s+)?(?:default\s+)?(?:class\s+\w+|function\s+\w+|interface\s+\w+|type\s+\w+|const\s+\w+\s*=\s*(?:async\s*)?\([^)]*\)\s*=>)`)
	case ".php":
		return outlineRegex(filePath, src, `(?m)^\s*(?:(?:final|abstract|readonly)\s+)?(?:class|interface|trait|enum)\s+\w+|^\s*(?:public|protected|private)?\s*(?:static)?\s*function\s+\w+`)
	default:
		return outlineRegex(filePath, src, `(?i)(class\s+\w+|def\s+\w+|function\s+\w+|struct\s+\w+)`)
	}
}

func outlineGo(filePath string, src []byte) (string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		return outlineRegex(filePath, src, `(?i)(type\s+\w+\s+(?:struct|interface)|func\s+(?:\([^)]+\)\s+)?\w+)`)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "📦 Arquivo: %s (Go - Pacote: %s)\n", filePath, node.Name.Name)

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
					line := fset.Position(typeSpec.Pos()).Line
					fmt.Fprintf(&b, "L%-4d: %s %s\n", line, kind, typeSpec.Name.Name)
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
			line := fset.Position(d.Pos()).Line
			fmt.Fprintf(&b, "L%-4d: func %s%s(...)\n", line, recv, d.Name.Name)
		}
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func outlineRegex(filePath string, src []byte, pattern string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "📄 Arquivo: %s\n", filePath)

	scanner := bufio.NewScanner(strings.NewReader(string(src)))
	lineNum := 1
	for scanner.Scan() {
		line := scanner.Text()
		if re.MatchString(line) {
			fmt.Fprintf(&b, "L%-4d: %s\n", lineNum, strings.TrimSpace(line))
		}
		lineNum++
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
