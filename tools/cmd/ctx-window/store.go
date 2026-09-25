package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Session represents the state of a session: working memory (last K
// tool calls) + incremental summary + metadata.
type Session struct {
	ID          string    `json:"id"`
	K           int       `json:"k"`
	Summarizer  string    `json:"summarizer"`
	CLIName     string    `json:"cli_name"` // which CLI owns this session: claude | codex | opencode | cursor | antigravity
	ProjectPath string    `json:"project_path,omitempty"`
	NudgeSent   bool      `json:"nudge_sent,omitempty"`
	Budget      int       `json:"budget"`
	Version     int       `json:"version"`
	Turns       []Turn    `json:"turns"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Turn is one entry of the working memory (verbatim).
type Turn struct {
	Role    string    `json:"role"`    // "tool" | "assistant" | "user"
	Content string    `json:"content"` // text of the tool call / response
	At      time.Time `json:"at"`
}

// cacheRoot lets tests isolate from the real cache (empty = real UserCacheDir).
var cacheRoot = ""

// SessionDir returns the directory of a session inside the agent-sync cache.
func SessionDir(id string) (string, error) {
	root := cacheRoot
	if root == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			home, herr := os.UserHomeDir()
			if herr != nil {
				return "", fmt.Errorf("ctx-window: no cache/home directory available: %w", err)
			}
			cache = filepath.Join(home, ".cache")
		}
		root = filepath.Join(cache, "agent-sync")
	}
	return filepath.Join(root, "ctx-window", id), nil
}

// EnsureSessionDir cria o diretório da sessão se não existir.
func EnsureSessionDir(id string) (string, error) {
	dir, err := SessionDir(id)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("ctx-window: criar diretório da sessão %q: %w", id, err)
	}
	return dir, nil
}

func metaPath(dir string) string    { return filepath.Join(dir, "meta.json") }
func turnsPath(dir string) string   { return filepath.Join(dir, "turns.jsonl") }
func summaryPath(dir string) string { return filepath.Join(dir, "summary.md") }
func versionPath(dir string, v int) string {
	return filepath.Join(dir, fmt.Sprintf("summary_v%d.md", v))
}

// EstimatedChars returns a rough size estimate of the working memory in
// characters (used by on-tool-call to decide auto-compaction).
func (s *Session) EstimatedChars() int {
	total := 0
	for _, t := range s.Turns {
		total += len(t.Content) + len(t.Role) + 32 // padding for JSON envelope
	}
	return total
}

func compactAtThreshold() int {
	if v := strings.TrimSpace(os.Getenv("AGENT_SYNC_CTX_COMPACT_AT")); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	return 200
}

// Load carrega uma sessão do disco. Cria nova se não existir.
func Load(id string) (*Session, error) {
	if strings.TrimSpace(id) == "" || strings.ContainsAny(id, "/\\\x00\r\n") {
		return nil, fmt.Errorf("ctx-window: invalid session id: %q", id)
	}
	dir, err := EnsureSessionDir(id)
	if err != nil {
		return nil, err
	}
	s := &Session{
		ID:         id,
		K:          DefaultK(),
		Summarizer: SummarizerFromConfig(),
		Budget:     DefaultBudget(),
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	meta, err := os.ReadFile(metaPath(dir))
	if err == nil {
		if jerr := json.Unmarshal(meta, s); jerr != nil {
			return nil, fmt.Errorf("ctx-window: meta.json corrompido: %w", jerr)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("ctx-window: ler meta.json: %w", err)
	}
	turns, err := readTurns(turnsPath(dir))
	if err != nil {
		return nil, err
	}
	s.Turns = turns
	return s, nil
}

// Save persiste os metadados da sessão E regrava turns.jsonl (idempotente).
// Use após operações que mudem Turns em memória (ex.: set-k + TrimTurns).
func (s *Session) Save() error {
	dir, err := EnsureSessionDir(s.ID)
	if err != nil {
		return err
	}
	s.UpdatedAt = time.Now().UTC()
	meta, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("ctx-window: serializar meta: %w", err)
	}
	if err := os.WriteFile(metaPath(dir), meta, 0o600); err != nil {
		return fmt.Errorf("ctx-window: gravar meta.json: %w", err)
	}
	// regrava turns.jsonl a partir de s.Turns (caso tenham sido trimmados)
	if err := writeTurns(filepath.Join(dir, "turns.jsonl"), s.Turns); err != nil {
		return err
	}
	return nil
}

// writeTurns sobrescreve turns.jsonl com a lista atual de turns.
func writeTurns(path string, turns []Turn) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("ctx-window: criar dir de turns: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("ctx-window: abrir turns.jsonl: %w", err)
	}
	defer f.Close()
	for _, tn := range turns {
		line, err := json.Marshal(tn)
		if err != nil {
			return fmt.Errorf("ctx-window: serializar turn: %w", err)
		}
		if _, err := f.Write(append(line, '\n')); err != nil {
			return fmt.Errorf("ctx-window: gravar turn: %w", err)
		}
	}
	return nil
}

// AddTurn acrescenta um tool call/turn à working memory e persiste.
// Se exceder K, descarta os mais antigos (TrimTurns).
func (s *Session) AddTurn(turn Turn) error {
	if turn.Role == "" {
		turn.Role = "tool"
	}
	if turn.At.IsZero() {
		turn.At = time.Now().UTC()
	}
	s.Turns = append(s.Turns, turn)
	s.TrimTurns()
	dir, err := EnsureSessionDir(s.ID)
	if err != nil {
		return err
	}
	return appendTurn(turnsPath(dir), turn)
}

// TrimTurns mantém apenas os últimos K turns na working memory.
func (s *Session) TrimTurns() {
	if s.K < 1 {
		s.K = DefaultK()
	}
	if len(s.Turns) > s.K {
		s.Turns = s.Turns[len(s.Turns)-s.K:]
	}
}

// AppendVersionedSummary writes the current summary to summary.md and
// creates a historical copy in summary_v{N}.md. Increments Version.
func (s *Session) AppendVersionedSummary(yaml string) error {
	dir, err := EnsureSessionDir(s.ID)
	if err != nil {
		return err
	}
	s.Version++
	if err := os.WriteFile(summaryPath(dir), []byte(yaml), 0o600); err != nil {
		return fmt.Errorf("ctx-window: gravar summary.md: %w", err)
	}
	if err := os.WriteFile(versionPath(dir, s.Version), []byte(yaml), 0o600); err != nil {
		return fmt.Errorf("ctx-window: gravar versão %d: %w", s.Version, err)
	}
	return nil
}

// ListVersions devolve os caminhos de todas as versões históricas,
// ordenadas crescentemente (mais antiga primeiro).
func (s *Session) ListVersions() ([]string, error) {
	dir, err := SessionDir(s.ID)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("ctx-window: listar versões: %w", err)
	}
	var versions []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "summary_v") && strings.HasSuffix(name, ".md") {
			versions = append(versions, filepath.Join(dir, name))
		}
	}
	sort.Strings(versions)
	return versions, nil
}

// WriteReport prints the current summary, working memory and versions to stdout.
func (s *Session) WriteReport(w io.Writer) error {
	dir, err := SessionDir(s.ID)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "# Sessão %q\n", s.ID)
	fmt.Fprintf(w, "- K=%d  summarizer=%q  budget=%d tokens  versão=%d\n", s.K, s.Summarizer, s.Budget, s.Version)
	fmt.Fprintf(w, "- criada em %s\n", s.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(w, "- atualizada em %s\n", s.UpdatedAt.Format(time.RFC3339))
	fmt.Fprintf(w, "\n## Current summary\n\n")
	if body, err := os.ReadFile(summaryPath(dir)); err == nil {
		w.Write(body)
	} else if os.IsNotExist(err) {
		fmt.Fprintln(w, "(no summary yet — run `ctx-window compact` to generate one)")
	} else {
		return fmt.Errorf("ctx-window: read summary.md: %w", err)
	}
	fmt.Fprintf(w, "\n## Working memory (%d/%d)\n\n", len(s.Turns), s.K)
	for _, tn := range s.Turns {
		fmt.Fprintf(w, "- [%s] %s\n", tn.At.Format(time.RFC3339), oneLine(tn.Content))
	}
	versions, err := s.ListVersions()
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "\n## Versões históricas (%d)\n\n", len(versions))
	for _, v := range versions {
		fmt.Fprintf(w, "- %s\n", filepath.Base(v))
	}
	return nil
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > 200 {
		return s[:197] + "..."
	}
	return s
}

func appendTurn(path string, t Turn) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("ctx-window: criar dir de turns: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("ctx-window: abrir turns.jsonl: %w", err)
	}
	defer f.Close()
	line, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("ctx-window: serializar turn: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("ctx-window: gravar turn: %w", err)
	}
	return nil
}

func readTurns(path string) ([]Turn, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("ctx-window: abrir turns.jsonl: %w", err)
	}
	defer f.Close()
	var out []Turn
	dec := json.NewDecoder(f)
	for dec.More() {
		var t Turn
		if err := dec.Decode(&t); err != nil {
			return nil, fmt.Errorf("ctx-window: decodificar turn: %w", err)
		}
		out = append(out, t)
	}
	return out, nil
}

// DefaultK devolve K da env ou fallback 5 (alinhado ao paper).
func DefaultK() int {
	if v := strings.TrimSpace(os.Getenv("AGENT_SYNC_CTX_K")); v != "" {
		var k int
		if _, err := fmt.Sscanf(v, "%d", &k); err == nil && k > 0 {
			return k
		}
	}
	return 5
}

// DefaultBudget devolve o teto de tokens da env ou fallback 1000.
func DefaultBudget() int {
	if v := strings.TrimSpace(os.Getenv("AGENT_SYNC_CTX_BUDGET")); v != "" {
		var b int
		if _, err := fmt.Sscanf(v, "%d", &b); err == nil && b > 0 {
			return b
		}
	}
	return 1000
}

// SummarizerFromConfig devolve o summarizer configurado: env
// AGENT_SYNC_SUMMARIZER tem prioridade; senão o campo "summarizer" do
// ~/.config/agent-sync/config.json; senão "agent".
func SummarizerFromConfig() string {
	if v := strings.TrimSpace(os.Getenv("AGENT_SYNC_SUMMARIZER")); v != "" {
		return v
	}
	if v := summarizerFromUserConfig(); v != "" {
		return v
	}
	return "agent"
}

// summarizerFromUserConfig lê o campo "summarizer" do config.json do
// usuário. Qualquer erro de leitura/parsing vira "" (fallback silencioso
// para "agent") — config ausente nunca pode quebrar o hook.
func summarizerFromUserConfig() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join(dir, "agent-sync", "config.json"))
	if err != nil {
		return ""
	}
	var cfg struct {
		Summarizer string `json:"summarizer"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.Summarizer)
}

// ollamaAvailable retorna true se `ollama` está no PATH.
func ollamaAvailable() bool {
	_, err := exec.LookPath("ollama")
	return err == nil
}

// LatestSessionForProject encontra a sessão mais recente salva no ctx-window para o caminho do projeto dado.
func LatestSessionForProject(projectPath string) (*Session, error) {
	if projectPath == "" {
		return nil, errors.New("ctx-window: caminho do projeto vazio")
	}
	projectPath = filepath.Clean(projectPath)
	root, err := SessionDir("placeholder")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Dir(root))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("ctx-window: nenhuma sessão encontrada para o projeto %s", projectPath)
		}
		return nil, err
	}
	var latestID string
	var latestTime time.Time
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(filepath.Dir(root), entry.Name())
		meta, err := os.ReadFile(metaPath(dir))
		if err != nil {
			continue
		}
		var session Session
		if json.Unmarshal(meta, &session) != nil || filepath.Clean(session.ProjectPath) != projectPath {
			continue
		}
		if session.UpdatedAt.After(latestTime) || latestID == "" {
			latestID = session.ID
			latestTime = session.UpdatedAt
		}
	}
	if latestID == "" {
		return nil, fmt.Errorf("ctx-window: nenhuma sessão encontrada para o projeto %s", projectPath)
	}
	return Load(latestID)
}

// FindProjectRoot locates the git repository root of dir, or falls back to dir.
func FindProjectRoot(dir string) string {
	if dir == "" {
		dir, _ = os.Getwd()
	}
	if dir == "" {
		return ""
	}
	dir = filepath.Clean(dir)
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err == nil {
		root := strings.TrimSpace(string(out))
		if root != "" {
			return root
		}
	}
	return dir
}

// ProjectSummaryPath returns the path to <projectRoot>/.agent-sync/summary.md
func ProjectSummaryPath(projectPath string) string {
	root := FindProjectRoot(projectPath)
	if root == "" {
		return ""
	}
	return filepath.Join(root, ".agent-sync", "summary.md")
}

// SaveProjectSummary writes the summary to <projectRoot>/.agent-sync/summary.md
// and ensures .agent-sync/.gitignore has "*" so it is never committed.
func SaveProjectSummary(projectPath, yaml string) error {
	root := FindProjectRoot(projectPath)
	if root == "" {
		return errors.New("ctx-window: project root not found")
	}
	agentSyncDir := filepath.Join(root, ".agent-sync")
	if err := os.MkdirAll(agentSyncDir, 0o755); err != nil {
		return fmt.Errorf("ctx-window: mkdir %s: %w", agentSyncDir, err)
	}
	gitignore := filepath.Join(agentSyncDir, ".gitignore")
	if _, err := os.Stat(gitignore); os.IsNotExist(err) {
		_ = os.WriteFile(gitignore, []byte("*\n"), 0o644)
	}
	return os.WriteFile(filepath.Join(agentSyncDir, "summary.md"), []byte(yaml), 0o644)
}

// LoadProjectSummary reads <projectRoot>/.agent-sync/summary.md if it exists.
func LoadProjectSummary(projectPath string) (string, error) {
	p := ProjectSummaryPath(projectPath)
	if p == "" {
		return "", nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
