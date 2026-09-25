package state

// state_migrate.go: migracao de STATE.md (markdown pre-ADR-002) para
// .agent-sync/session-state.json (canonico via ADR-002 Estagio B).
//
// Stub hoje - ADR-002 Estagio A fechou smoke 5 CLIs sem migracao;
// Estagio B fechou cross-CLI handoff. Migracao MD->JSON em entrega
// futura quando usuario pedir.
//
// Renomeado de state_migrate.go (preservado como mesmo nome - sem risco
// de conflito). Migrado em 2026-09-21 (Fase 3).
import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// runStateMigrateFromMD implementa o subcommand `state migrate-from-md`
// da ADR-002 Estágio B Decisão 4. Lê STATE.md, extrai decisions/actions/
// blockers/open_questions por heurística de cabeçalhos e bullets, gera
// session-state.json canônico via WriteSessionState (que valida contra
// schema v1 antes do atomic rename). Heurística é best-effort — emite
// warnings para seções que não conseguiu extrair.
//
// Padrões reconhecidos:
//
//	Próximas ações:  "1. ✅ **<title>** ..."  |  "1. ❌ <title>"  |  "1. <title>"
//	Decisões:        "- **<title>** — <rationale> (<date>)"
//	Bloqueios:       "- **<title>** — bloqueia: A-1, A-2"
//	Open questions:  "- <texto livre>"
//
// branch/head do git via `git -C <root>` (best-effort: sem git, fields vazios).
func runStateMigrateFromMD(args []string) error {
	f := newStateFlags("agent-sync state migrate-from-md")
	if err := f.parse(args, os.Stderr); err != nil {
		return err
	}
	root, err := pathutil.ResolveStateRoot(f.root)
	if err != nil {
		return err
	}

	mdPath := filepath.Join(root, "STATE.md")
	data, err := os.ReadFile(mdPath)
	if err != nil {
		return fmt.Errorf("ler STATE.md em %s: %w", mdPath, err)
	}

	s, warnings := parseStateMD(data, root)
	if err := WriteSessionState(root, s); err != nil {
		return fmt.Errorf("escrita do session-state.json: %w", err)
	}

	fmt.Println("ok")
	fmt.Fprintf(os.Stderr, "extraido: %d decisions, %d open_questions\n",
		len(s.Decisions), len(s.OpenQuestions))
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	return nil
}

func parseStateMD(data []byte, root string) (SessionState, []string) {
	now := time.Now().UTC()
	warnings := []string{}
	sections := splitMDSections(string(data))

	s := SessionState{
		SchemaVersion: SessionStateSchemaVersion,
		Project: SessionProject{
			Name: extractProjectName(string(data), root),
			Root: root,
		},
		Git: extractGitFromRoot(root),
		Session: SessionMeta{
			ID:        "sess-" + now.Format("20060102-150405"),
			StartedAt: now,
			UpdatedAt: now,
		},
	}

	actionSection, _ := lookupSection(sections, "próximas ações")
	if actionSection == "" {
		warnings = append(warnings, "seção '## Próximas ações' não encontrada; tasks vazio")
	}

	decSection, _ := lookupSection(sections, "decisões")
	if decSection == "" {
		warnings = append(warnings, "seção '## Decisões' não encontrada; decisions vazio")
	}
	s.Decisions = extractDecisionsFromSection(decSection)

	blSection, _ := lookupSection(sections, "bloqueios")
	if blSection == "" {
		warnings = append(warnings, "seção '## Bloqueios' não encontrada; issues vazio")
	}

	oqSection, _ := lookupSection(sections, "perguntas em aberto")
	if oqSection == "" {
		warnings = append(warnings, "seção '## Perguntas em aberto' não encontrada; open_questions vazio")
	}
	s.OpenQuestions = extractOpenQuestionsFromSection(oqSection)

	return s, warnings
}

// splitMDSections retorna map de section-key (lowercased, sufixo entre
// parênteses removido) → texto da seção (sem o header). Apenas headers
// "## ..." nível 2 são reconhecidos (ignora h1 e h3+).
func splitMDSections(md string) map[string]string {
	out := map[string]string{}
	var current string
	var buf strings.Builder
	flush := func() {
		if current != "" {
			out[current] = buf.String()
		}
		current = ""
		buf.Reset()
	}
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "## ") {
			flush()
			h := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if i := strings.Index(h, " ("); i > 0 {
				h = h[:i]
			}
			current = strings.ToLower(h)
		} else if current != "" {
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}
	flush()
	return out
}

// lookupSection retorna o texto da primeira section cuja chave começa com
// o prefixo fornecido (case-insensitive). Permite match tolerante: query
// "decisões" casa com "decisões ativas (resumo)"; query "bloqueios" casa
// com "bloqueios / perguntas abertas". Match exato tem prioridade.
func lookupSection(sections map[string]string, prefix string) (string, bool) {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if v, ok := sections[prefix]; ok {
		return v, true
	}
	for k, v := range sections {
		if strings.HasPrefix(k, prefix) {
			return v, true
		}
	}
	return "", false
}

func extractProjectName(md, root string) string {
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "# STATE — ") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "# STATE — "))
			if name != "" {
				return name
			}
		}
	}
	return filepath.Base(root)
}

func extractGitFromRoot(root string) SessionGit {
	g := SessionGit{}
	if branch, err := gitOutput(root, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		g.Branch = branch
	}
	if head, err := gitOutput(root, "rev-parse", "HEAD"); err == nil {
		g.Head = head
	}
	if summary, err := gitOutput(root, "status", "--porcelain"); err == nil {
		if summary == "" {
			g.WorkingTreeSummary = "limpo"
		} else {
			g.WorkingTreeSummary = strings.TrimSpace(summary)
		}
	} else {
		g.WorkingTreeSummary = "(git indisponível)"
	}
	return g
}

func gitOutput(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

var (
	reNumberedPrefix = regexp.MustCompile(`^(\d+)\.\s+`)
	reBullet         = regexp.MustCompile(`^-\s+`)
	reBoldTitle      = regexp.MustCompile(`^\*\*([^*]+)\*\*\s*(.*)$`)
)

func extractDecisionsFromSection(text string) []SessionDecision {
	var out []SessionDecision
	if text == "" {
		return out
	}
	n := 1
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !reBullet.MatchString(line) {
			continue
		}
		body := strings.TrimSpace(reBullet.ReplaceAllString(line, ""))
		if body == "" {
			continue
		}
		title := body
		rationale := ""
		if m := reBoldTitle.FindStringSubmatch(body); m != nil {
			title = strings.TrimSpace(m[1])
			rationale = strings.TrimSpace(m[2])
		}
		if i := strings.Index(rationale, " — "); i >= 0 {
			rationale = strings.TrimSpace(rationale[i+len(" — "):])
		}
		out = append(out, SessionDecision{
			ID:        fmt.Sprintf("D-%d", n),
			Title:     title,
			Rationale: rationale,
			MadeAt:    time.Now().UTC(),
		})
		n++
	}
	return out
}

func extractOpenQuestionsFromSection(text string) []string {
	var out []string
	if text == "" {
		return out
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !reBullet.MatchString(line) {
			continue
		}
		body := strings.TrimSpace(reBullet.ReplaceAllString(line, ""))
		if body == "" {
			continue
		}
		out = append(out, body)
	}
	return out
}
