package audit

import (
	"path/filepath"
	"strings"
)

// EvidenceClass é uma das 9 classes de evidência reconhecidas pelo
// removal audit (7 portadas do cartographer + 2 custom Go).
type EvidenceClass string

const (
	ClassDocsActive         EvidenceClass = "docs-active"
	ClassDocsHistorical     EvidenceClass = "docs-historical"
	ClassUnknownLiteralHit  EvidenceClass = "unknown-literal-hit"
	ClassPackageDependency  EvidenceClass = "package-dependency"
	ClassLockfileReference  EvidenceClass = "lockfile-reference"
	ClassEnvVar             EvidenceClass = "env-var"
	ClassImportOrSDKClient  EvidenceClass = "import-or-sdk-client"
	ClassGoPackageReference EvidenceClass = "go-package-reference"
	ClassSQLTableReference  EvidenceClass = "sql-table-reference"
)

// AllClasses devolve a ordem canônica das 9 classes (mesma ordem usada em
// BuildRemovalAudit para preencher o ledger).
func AllClasses() []EvidenceClass {
	return []EvidenceClass{
		ClassPackageDependency,
		ClassLockfileReference,
		ClassGoPackageReference,
		ClassImportOrSDKClient,
		ClassEnvVar,
		ClassDocsActive,
		ClassDocsHistorical,
		ClassSQLTableReference,
		ClassUnknownLiteralHit,
	}
}

// ClassifyHit atribui uma classe ao hit textual. O target é necessário
// para algumas heurísticas (env-var UPPER_SNAKE, go-package-reference).
//
// A precedência é importante: caminhos mais específicos são tentados antes
// do default `unknown-literal-hit`. Cada regra é uma função local que
// devolve (classe, true) se casar.
//
// Ordem (risco de colisão decrescente):
//  1. lockfile (mais restritivo)
//  2. .sql (sempre SQL)
//  3. go-package-reference (path/segmento Go)
//  4. package.json (npm dep)
//  5. sql-table-reference (tabela do schema em .go/.ts/.js)
//  6. env-var (.env ou UPPER_SNAKE_*)
//  7. import-or-sdk-client (JS/TS/PHP/Python — Go já tratado antes)
//  8. docs (active/historical)
//  9. default: unknown-literal-hit
func ClassifyHit(hit FileHit, target AuditTarget, sqlTables []string) EvidenceClass {
	path := strings.ToLower(hit.Path)
	line := hit.LineText
	ext := strings.ToLower(filepath.Ext(path))

	if isLockfile(path) {
		return ClassLockfileReference
	}
	if ext == ".sql" {
		return ClassSQLTableReference
	}
	if isGoPackageReference(path, line, target) {
		return ClassGoPackageReference
	}
	if isPackageJSON(path) {
		return ClassPackageDependency
	}
	if isSQLTableReference(path, line, sqlTables) {
		return ClassSQLTableReference
	}
	if isEnvVar(path, line, target) {
		return ClassEnvVar
	}
	if isImportOrSDKClient(path, line) {
		return ClassImportOrSDKClient
	}
	if isDocsPath(path) {
		if isHistoricalDocPath(path) {
			return ClassDocsHistorical
		}
		return ClassDocsActive
	}
	return ClassUnknownLiteralHit
}

func isPackageJSON(path string) bool {
	return strings.HasSuffix(path, "/package.json") || path == "package.json"
}

// isLockfile reconhece bun.lock, package-lock.json, pnpm-lock.yaml,
// yarn.lock, go.sum e go.mod (estes 2 últimos são Go-nativos).
func isLockfile(path string) bool {
	base := filepath.Base(path)
	switch base {
	case "bun.lock", "package-lock.json", "pnpm-lock.yaml",
		"yarn.lock", "composer.lock", "go.sum", "go.mod":
		return true
	}
	return false
}

// isGoPackageReference detecta refs a packages internos Go via caminho
// `internal/X/Y` ou nome curto do target. Heurística:
//
//  1. arquivo .go contém `internal/<segmento>/` no path
//  2. linha contém `import "<...>/<segmento>"` ou `package <segmento>`
//  3. nome curto do target aparece como segmento de path Go
func isGoPackageReference(path, line string, target AuditTarget) bool {
	if !strings.HasSuffix(path, ".go") {
		return false
	}
	base := target.TargetBase()
	if base == "" {
		return false
	}
	// path do hit inclui "internal/<base>/..." (em qualquer profundidade)
	// ou começa com "internal/<base>/" sem prefixo.
	if strings.Contains(path, "/internal/"+base+"/") ||
		strings.HasPrefix(path, "internal/"+base+"/") ||
		strings.Contains(path, "/internal/"+base+".") {
		return true
	}
	if strings.HasSuffix(path, "/"+base+".go") {
		return true
	}
	if strings.HasSuffix(path, "/"+base+"/") {
		return true
	}
	// linha contém import Go ou referência de package
	if strings.Contains(line, "import ") && strings.Contains(line, "/"+base) {
		return true
	}
	if strings.Contains(line, "package "+base) {
		return true
	}
	return false
}

// isImportOrSDKClient detecta imports em JS/TS/PHP/Python. Go é tratado
// exclusivamente por isGoPackageReference (precedência) e SQL por
// isSQLTableReference (também precedência).
func isImportOrSDKClient(path, line string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts",
		".py", ".php":
	default:
		return false
	}
	lower := strings.ToLower(line)
	// JS/TS: "import x from 'y'", "require('y')"
	// Python: "from x import y", "import x"
	// PHP: "use Foo\Bar;"
	if strings.Contains(lower, "import ") ||
		strings.Contains(lower, "from ") ||
		strings.Contains(lower, "require(") ||
		strings.Contains(lower, "use ") {
		return true
	}
	return false
}

// isSQLTableReference detecta refs a nomes de tabela do schema libsql.
// Se sqlTables for vazio (sem introspection), classifica qualquer hit em
// .sql como potencial referência SQL.
func isSQLTableReference(path, line string, sqlTables []string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".sql" {
		return true
	}
	if ext == ".go" || ext == ".ts" || ext == ".js" {
		if len(sqlTables) == 0 {
			return false
		}
		lower := strings.ToLower(line)
		for _, tbl := range sqlTables {
			t := strings.ToLower(tbl)
			if t != "" && strings.Contains(lower, t) {
				return true
			}
		}
	}
	return false
}

// isEnvVar detecta env-vars tanto no path (.env, .env.example) quanto
// no padrão UPPER_SNAKE_* derivado do target.
func isEnvVar(path, line string, target AuditTarget) bool {
	base := filepath.Base(path)
	if base == ".env" || strings.HasSuffix(base, ".env") {
		return true
	}
	upper := upperSnake(target.Raw)
	if upper == "" || upper == target.Raw {
		return false
	}
	return strings.Contains(line, upper+"_")
}

// isDocsPath cobre .md, .mdx, .rst, .txt e diretório docs/.
func isDocsPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md", ".mdx", ".rst", ".txt":
		return true
	}
	return strings.HasPrefix(path, "docs/") || strings.Contains(path, "/docs/")
}

// isHistoricalDocPath: subdiretórios archive, history, historical, deprecated.
func isHistoricalDocPath(path string) bool {
	lower := strings.ToLower(path)
	for _, marker := range []string{"/archive/", "/history/", "/historical/", "/deprecated/"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	if strings.HasPrefix(lower, "archive/") || strings.HasPrefix(lower, "history/") ||
		strings.HasPrefix(lower, "historical/") || strings.HasPrefix(lower, "deprecated/") {
		return true
	}
	return false
}
