// Package memory implementa `agent-sync memory <subcommand>`.
//
// Origem: A-55 (S-0.2 Entrega 3 Trilha A). Wrapper minimalista sobre
// internal/event que separa namespace sem acoplamento. Sem schema bump
// (não usa last_accessed_at nem score de autoridade).
package memory

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/matheusdutra/agent-sync/internal/event"
	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// RunCommand implementa `agent-sync memory <subcommand>`.
func RunCommand(args []string) error {
	if len(args) == 0 {
		return memoryUsage(os.Stderr)
	}
	switch args[0] {
	case "recent":
		return runMemoryRecent(args[1:])
	case "feedback":
		return runMemoryFeedback(args[1:])
	case "write-page":
		return runMemoryWritePage(args[1:])
	case "help", "-h", "--help":
		return memoryUsage(os.Stdout)
	default:
		return fmt.Errorf("memory: subcommand desconhecido: %q", args[0])
	}
}

func memoryUsage(w io.Writer) error {
	fmt.Fprintf(w, "Uso: agent-sync memory <subcommand> [-root <path>]\n\n")
	fmt.Fprintf(w, "Subcommands:\n")
	fmt.Fprintf(w, "  recent       Lista os N eventos mais recentes (wrapper sobre 'event read')\n")
	fmt.Fprintf(w, "  feedback     Registra feedback sobre uma memoria (append-only JSONL)\n")
	fmt.Fprintf(w, "  write-page   Grava uma pagina de memoria (append-only JSONL, formato F1)\n")
	fmt.Fprintf(w, "\nFlags (recent):\n")
	fmt.Fprintf(w, "  -last N      Apenas os ultimos N eventos (0 = todos, default 10)\n")
	fmt.Fprintf(w, "  -kind K      Filtra por kind (decision|action|blocker|open_question|state_render)\n")
	fmt.Fprintf(w, "  -since RFC   Apenas eventos com ts >= valor (RFC3339)\n")
	fmt.Fprintf(w, "  -root <path> Project root (default: derivado do binario + AGENT_SYNC_HOME + cwd)\n")
	fmt.Fprintf(w, "\nFlags (feedback):\n")
	fmt.Fprintf(w, "  <path>       Path/slug da memoria alvo (obrigatorio)\n")
	fmt.Fprintf(w, "  <signal>     Sinal: helpful|not_helpful|stale|wrong (obrigatorio)\n")
	fmt.Fprintf(w, "  -reason TXT  Texto livre explicando o feedback (opcional)\n")
	fmt.Fprintf(w, "  -agent X     Qual CLI gravou (default=derivado de AGENT_SYNC_AGENT ou 'agent-sync')\n")
	fmt.Fprintf(w, "  -cache-dir D Diretorio do JSONL (default: ~/.cache/agent-sync)\n")
	fmt.Fprintf(w, "\nFlags (write-page):\n")
	fmt.Fprintf(w, "  <path>       Path/slug da pagina (obrigatorio, ex.: 'adr/A-59-decisao')\n")
	fmt.Fprintf(w, "  -body MD     Corpo markdown da pagina (obrigatorio; usa YAML frontmatter)\n")
	fmt.Fprintf(w, "  -scope S     Escopo: project (default) | global\n")
	fmt.Fprintf(w, "  -expires-at T Expiracao RFC3339 ou date-only (opcional; A-53 ativa sweep)\n")
	fmt.Fprintf(w, "  -agent X     Qual CLI gravou (default=derivado de AGENT_SYNC_AGENT ou 'agent-sync')\n")
	fmt.Fprintf(w, "  -cache-dir D Diretorio do JSONL (default: ~/.cache/agent-sync)\n")
	fmt.Fprintf(w, "\n  ATENCAO: -body/-scope/-expires-at/-agent/-cache-dir devem vir ANTES de <path>.\n")
	fmt.Fprintf(w, "  Apos o primeiro posicional, o flag.Parse do Go para de interpretar flags.\n")
	fmt.Fprintf(w, "\nFormato do body (F1):\n")
	fmt.Fprintf(w, "  ---\\n")
	fmt.Fprintf(w, "  scope: <project|global>          # opcional se -scope foi passado\n")
	fmt.Fprintf(w, "  expires_at: <RFC3339|date-only> # opcional\n")
	fmt.Fprintf(w, "  ---\\n")
	fmt.Fprintf(w, "  <corpo markdown>\\n")
	return nil
}

type memoryFlags struct {
	fs    *flag.FlagSet
	root  string
	last  int
	kind  string
	since string
}

func newMemoryFlags(name string) *memoryFlags {
	f := &memoryFlags{}
	f.fs = flag.NewFlagSet(name, flag.ContinueOnError)
	f.fs.StringVar(&f.root, "root", "", "Project root")
	f.fs.IntVar(&f.last, "last", 10, "Ultimos N eventos (0 = todos)")
	f.fs.StringVar(&f.kind, "kind", "", "Filtra por kind")
	f.fs.StringVar(&f.since, "since", "", "Filtra por ts >= RFC3339")
	return f
}

func runMemoryRecent(args []string) error {
	f := newMemoryFlags("agent-sync memory recent")
	if err := f.fs.Parse(args); err != nil {
		return err
	}
	root, err := pathutil.ResolveStateRoot(f.root)
	if err != nil {
		return err
	}
	opts := event.EventReadOptions{Last: f.last, Kind: f.kind}
	if f.since != "" {
		ts, err := time.Parse(time.RFC3339, f.since)
		if err != nil {
			return fmt.Errorf("since invalido (esperado RFC3339): %w", err)
		}
		opts.Since = ts
	}
	events, err := event.ReadEvents(root, opts)
	if err != nil {
		return err
	}
	for _, e := range events {
		line, err := json.Marshal(&e)
		if err != nil {
			return err
		}
		fmt.Println(string(line))
	}
	return nil
}

// feedbackFlags agrega flags do subcommand feedback.
type feedbackFlags struct {
	fs       *flag.FlagSet
	reason   string
	agent    string
	cacheDir string
}

func newFeedbackFlags(name string) *feedbackFlags {
	f := &feedbackFlags{}
	f.fs = flag.NewFlagSet(name, flag.ContinueOnError)
	f.fs.StringVar(&f.reason, "reason", "", "Texto livre explicando o feedback")
	f.fs.StringVar(&f.agent, "agent", "", "Qual CLI gravou (default: $AGENT_SYNC_AGENT ou agent-sync)")
	f.fs.StringVar(&f.cacheDir, "cache-dir", "", "Diretorio do JSONL (default: ~/.cache/agent-sync)")
	return f
}

// validSignals lista os sinais aceitos. Definido como var para consistencia
// com a checagem e com a documentacao em memoryUsage.
var validSignals = map[string]bool{
	"helpful":     true,
	"not_helpful": true,
	"stale":       true,
	"wrong":       true,
}

// feedbackEntry e a estrutura gravada em ~/.cache/agent-sync/memory_feedback.jsonl.
// Schema minimo (5 campos): ts, path, signal, reason (opcional), agent.
type feedbackEntry struct {
	TS     time.Time `json:"ts"`
	Path   string    `json:"path"`
	Signal string    `json:"signal"`
	Reason string    `json:"reason,omitempty"`
	Agent  string    `json:"agent"`
}

// resolveCacheDir retorna o diretorio do JSONL de feedback.
// Se override vazio, usa UserCacheDir/agent-sync com fallback ~/.cache/agent-sync.
// Padrao identico ao DefaultDBPath em tools/internal/agentmemory/store.go:24-38.
func resolveCacheDir(override string) (string, error) {
	if override != "" {
		if abs, err := filepath.Abs(override); err == nil {
			if err := os.MkdirAll(abs, 0o755); err != nil {
				return "", fmt.Errorf("criar cache-dir %s: %w", abs, err)
			}
			return abs, nil
		}
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", fmt.Errorf("sem diretorio de cache/home: %w", err)
		}
		dir = filepath.Join(home, ".cache")
	}
	dir = filepath.Join(dir, "agent-sync")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("criar %s: %w", dir, err)
	}
	return dir, nil
}

// runMemoryFeedback registra feedback de uma memoria em JSONL append-only.
// Origem: A-58 (S-0.3 Camada 4, ses_f2b12742, 2026-09-24). Audit-only:
// helpful/not_helpful NAO tem efeito em ranking ate A-53 (schema bump deferido).
func runMemoryFeedback(args []string) error {
	f := newFeedbackFlags("agent-sync memory feedback")
	if err := f.fs.Parse(args); err != nil {
		return err
	}
	rest := f.fs.Args()
	if len(rest) < 2 {
		return fmt.Errorf("uso: agent-sync memory feedback <path> <signal> [-reason TXT]")
	}
	path := strings.TrimSpace(rest[0])
	signal := strings.TrimSpace(rest[1])
	if path == "" {
		return fmt.Errorf("path vazio")
	}
	if !validSignals[signal] {
		return fmt.Errorf("signal invalido: %q (esperado: helpful|not_helpful|stale|wrong)", signal)
	}

	agent := f.agent
	if agent == "" {
		agent = os.Getenv("AGENT_SYNC_AGENT")
	}
	if agent == "" {
		agent = "agent-sync"
	}

	dir, err := resolveCacheDir(f.cacheDir)
	if err != nil {
		return err
	}
	dst := filepath.Join(dir, "memory_feedback.jsonl")

	entry := feedbackEntry{
		TS:     time.Now().UTC(),
		Path:   path,
		Signal: signal,
		Reason: f.reason,
		Agent:  agent,
	}
	line, err := json.Marshal(&entry)
	if err != nil {
		return fmt.Errorf("marshal feedback: %w", err)
	}

	fh, err := os.OpenFile(dst, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("abrir %s: %w", dst, err)
	}
	defer fh.Close()
	if _, err := fh.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("escrever %s: %w", dst, err)
	}

	fmt.Fprintf(os.Stdout, "feedback registrado: signal=%s path=%s agent=%s\n", signal, path, agent)
	return nil
}

// writePageFlags agrega flags do subcommand write-page.
type writePageFlags struct {
	fs        *flag.FlagSet
	body      string
	scope     string
	expiresAt string
	agent     string
	cacheDir  string
}

func newWritePageFlags(name string) *writePageFlags {
	f := &writePageFlags{}
	f.fs = flag.NewFlagSet(name, flag.ContinueOnError)
	f.fs.StringVar(&f.body, "body", "", "Corpo markdown da pagina (obrigatorio; YAML frontmatter opcional)")
	f.fs.StringVar(&f.scope, "scope", "", "Escopo: project (default) | global")
	f.fs.StringVar(&f.expiresAt, "expires-at", "", "Expiracao RFC3339 ou date-only (opcional)")
	f.fs.StringVar(&f.agent, "agent", "", "Qual CLI gravou (default: $AGENT_SYNC_AGENT ou agent-sync)")
	f.fs.StringVar(&f.cacheDir, "cache-dir", "", "Diretorio do JSONL (default: ~/.cache/agent-sync)")
	return f
}

// validScopes lista os escopos aceitos. Definido como var para consistencia
// com a checagem e com a documentacao em memoryUsage.
var validScopes = map[string]bool{
	"project": true,
	"global":  true,
}

// pageEntry e a estrutura gravada em ~/.cache/agent-sync/memory_pages.jsonl.
// Origem: A-59 (S-0.3 Camada 4, ses_f2ad21a27ffeasIkrIiVwQc53m, 2026-09-24).
// Formato F1 (decisao fechada): body markdown COM frontmatter YAML no topo
// (entre "---" ... "---"). Compromise com A-56 (flag --with-frontmatter) e
// A-61 (lint de frontmatter invalido). SEM schema bump (cross-module bloqueado
// em implementacao; ver decision/a-59-revisado-jsonl-cross-module-bloqueado).
type pageEntry struct {
	TS        time.Time `json:"ts"`
	Path      string    `json:"path"`
	Scope     string    `json:"scope"`
	ExpiresAt string    `json:"expires_at,omitempty"`
	Body      string    `json:"body"`
	Agent     string    `json:"agent"`
}

// validateExpiresAt aceita RFC3339 ("2026-12-31T23:59:59Z") ou date-only
// ("2026-12-31"). Retorna string normalizada em RFC3339 (date-only vira
// "T00:00:00Z" do mesmo dia).
func validateExpiresAt(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}
	return "", fmt.Errorf("expires-at invalido %q (esperado RFC3339 ou YYYY-MM-DD)", raw)
}

// runMemoryWritePage grava uma pagina de memoria em JSONL append-only.
// Origem: A-59 (S-0.3 Camada 4, ses_f2ad21a27ffeasIkrIiVwQc53m, 2026-09-24).
// Padrao simetrico com runMemoryFeedback (A-58): mesma funcao resolveCacheDir,
// mesmo padrao de flags, mesmo append-only em ~/.cache/agent-sync/.
// Decisao de formato F1 gravada em memory-mcp: scope/expires_at como campos
// tipados do JSONL + frontmatter YAML dentro do body (lido por A-56/A-61).
func runMemoryWritePage(args []string) error {
	f := newWritePageFlags("agent-sync memory write-page")
	if err := f.fs.Parse(args); err != nil {
		return err
	}
	rest := f.fs.Args()
	if len(rest) < 1 {
		return fmt.Errorf("uso: agent-sync memory write-page <path> -body MD [-scope project|global] [-expires-at RFC3339|date-only]")
	}
	path := strings.TrimSpace(rest[0])
	if path == "" {
		return fmt.Errorf("path vazio")
	}
	if strings.TrimSpace(f.body) == "" {
		return fmt.Errorf("body vazio (use -body MD)")
	}

	scope := f.scope
	if scope == "" {
		scope = "project"
	}
	if !validScopes[scope] {
		return fmt.Errorf("scope invalido: %q (esperado: project|global)", scope)
	}

	expiresAt, err := validateExpiresAt(f.expiresAt)
	if err != nil {
		return err
	}

	agent := f.agent
	if agent == "" {
		agent = os.Getenv("AGENT_SYNC_AGENT")
	}
	if agent == "" {
		agent = "agent-sync"
	}

	dir, err := resolveCacheDir(f.cacheDir)
	if err != nil {
		return err
	}
	dst := filepath.Join(dir, "memory_pages.jsonl")

	entry := pageEntry{
		TS:        time.Now().UTC(),
		Path:      path,
		Scope:     scope,
		ExpiresAt: expiresAt,
		Body:      f.body,
		Agent:     agent,
	}
	line, err := json.Marshal(&entry)
	if err != nil {
		return fmt.Errorf("marshal page: %w", err)
	}

	fh, err := os.OpenFile(dst, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("abrir %s: %w", dst, err)
	}
	defer fh.Close()
	if _, err := fh.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("escrever %s: %w", dst, err)
	}

	fmt.Fprintf(os.Stdout, "page gravada: path=%s scope=%s agent=%s expires_at=%s\n", path, scope, agent, expiresAt)
	return nil
}
