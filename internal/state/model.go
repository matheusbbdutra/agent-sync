package state

// state_model.go: modelo canonico do estado do agente-sync em JSON.
//
// Schema (.agent-sync/session-state.json) e primitivas de serializacao.
// Fonte de verdade das decisoes A-N, D-N, bloqueadores B-N em
// estrutura tipada e versionada (schema_version=1.1). Migrado de
// session_state.go em 2026-09-21 (Fase 3 do refator por feature - sem
// subpasta porque Go trata cada diretorio como package; prefixo state_
// identifica coesao).
//
// Renomeado de session_state.go -> state_model.go para reduzir redundancia:
// o prefixo 'state_' identifica o dominio; 'model' descreve a funcao
// (modelo vs apply CLI).
//
// Bump 1.0 -> 1.1 (D-48, 2026-09-23): adicionados campos tasks e issues
// (aditivo, sem breaking change). validate() compara apenas major para
// aceitar leitura de arquivos 1.0 com binario 1.1.
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/matheusdutra/agent-sync/internal/event"
	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

const (
	SessionStateSchemaVersion = "1.2"
	SessionStateFileName      = "session-state.json"
	SessionStateDirName       = pathutil.SessionStateDirName
	SessionStateLockName      = "session-state.lock"
	SessionStateLockTimeout   = 5 * time.Second
	SessionStateLockEnvVar    = "AGENT_SYNC_SESSION_LOCK"
	SessionStateTmpAttempts   = 3
	SessionStateTmpBackoff    = 10 * time.Millisecond
	SessionStateLockPoll      = 50 * time.Millisecond
	StateMDFileName           = "STATE.md"
)

type SessionState struct {
	SchemaVersion string            `json:"schema_version"`
	Project       SessionProject    `json:"project"`
	Git           SessionGit        `json:"git"`
	Session       SessionMeta       `json:"session"`
	Decisions     []SessionDecision `json:"decisions"`
	Tasks         []SessionTask     `json:"tasks"`
	Issues        []SessionIssue    `json:"issues"`
	OpenQuestions []string          `json:"open_questions"`
}

type SessionProject struct {
	Name string `json:"name"`
	Root string `json:"root"`
}

type SessionGit struct {
	Branch             string `json:"branch"`
	Head               string `json:"head"`
	WorkingTreeSummary string `json:"working_tree_summary"`
}

type SessionMeta struct {
	ID        string    `json:"id"`
	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SessionDecision struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Rationale  string    `json:"rationale"`
	MadeAt     time.Time `json:"made_at"`
	LegacyKind string    `json:"legacy_kind,omitempty"`
	Links      []string  `json:"links,omitempty"`
}

type SessionAction struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Status     string   `json:"status"`
	BlockerRef string   `json:"blocker_ref,omitempty"`
	DependsOn  []string `json:"depends_on,omitempty"`
}

// SessionTask representa uma unidade de trabalho técnica em andamento ou
// concluída. Diferente de SessionAction (próximo passo do plano), SessionTask
// é persistente e granular — vive durante toda a sessão e serve como
// histórico de execução. Adicionada no bump de schema 1.0 → 1.1 (D-48).
type SessionTask struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Status     string     `json:"status"`
	LegacyKind string     `json:"legacy_kind,omitempty"`
	BranchRef  string     `json:"branch_ref,omitempty"`
	PRRef      string     `json:"pr_ref,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Links      []string   `json:"links,omitempty"`
	// ClaimedBy/ClaimedAt sao o estado de claim-once handoff (A-52).
	// ClaimedBy = string CLI kind que reivindicou (vazio = sem claim).
	// ClaimedAt = RFC3339 do momento do claim. Lockless (D-2): confianca
	// por timeout configuravel via AGENT_SYNC_HANDOFF_TIMEOUT (default 1h).
	// Backward compat: tasks legadas tem ClaimedBy='' (read ignora).
	ClaimedBy string    `json:"claimed_by,omitempty"`
	ClaimedAt time.Time `json:"claimed_at,omitempty"`
}

// SessionIssue representa um problema/observação identificado durante a
// sessão. Pode estar resolvido (resolvida=true) ou em aberto. Adicionada
// no bump de schema 1.0 → 1.1 (D-48).
type SessionIssue struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Severity    string     `json:"severity"`
	LegacyKind  string     `json:"legacy_kind,omitempty"`
	Description string     `json:"description"`
	DetectedAt  time.Time  `json:"detected_at"`
	Resolved    bool       `json:"resolved"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

func statePath(projectRoot string) string {
	return filepath.Join(projectRoot, pathutil.SessionStateDirName, SessionStateFileName)
}

func lockPath(projectRoot string) string {
	return filepath.Join(projectRoot, pathutil.SessionStateDirName, SessionStateLockName)
}

func ensureStateDir(projectRoot string) error {
	return pathutil.EnsureStateDir(projectRoot)
}

// normalize auto-popula slices nil como [] para satisfazer jsonschema
// com additionalProperties:false. Go marshaliza slice nil como null, e o
// schema rejeita null quando o campo é declarado como array. Aplica-se
// aos 4 slices do modelo (Decisions, Tasks, Issues, OpenQuestions).
// NextActions/Blockers foram removidos em A-37 (D-69/D-70). Idempotente:
// reler/escrever não muda estado material. Chamado em ReadSessionState,
// WriteSessionState e runStateWrite antes de qualquer validate/
// jsonschemaValidate (D-48 fatia 1a).
func (s *SessionState) normalize() {
	if s.Decisions == nil {
		s.Decisions = []SessionDecision{}
	}
	if s.Tasks == nil {
		s.Tasks = []SessionTask{}
	}
	if s.Issues == nil {
		s.Issues = []SessionIssue{}
	}
	if s.OpenQuestions == nil {
		s.OpenQuestions = []string{}
	}
}

func (s *SessionState) validate() error {
	if !acceptSchemaVersion(s.SchemaVersion, SessionStateSchemaVersion) {
		return fmt.Errorf("schema_version %q incompativel com %q", s.SchemaVersion, SessionStateSchemaVersion)
	}
	if s.Project.Name == "" {
		return fmt.Errorf("project.name vazio")
	}
	if s.Project.Root == "" {
		return fmt.Errorf("project.root vazio")
	}
	if s.Session.ID == "" {
		return fmt.Errorf("session.id vazio")
	}
	return nil
}

func ReadSessionState(projectRoot string) (SessionState, error) {
	var s SessionState
	data, err := os.ReadFile(statePath(projectRoot))
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("parse session-state.json: %w", err)
	}
	if err := s.validate(); err != nil {
		return s, fmt.Errorf("session-state.json inválido: %w", err)
	}
	return s, nil
}

func WriteSessionState(projectRoot string, s SessionState) error {
	old, _ := readSessionStateIfExists(projectRoot)

	s.Session.UpdatedAt = time.Now().UTC()
	if s.SchemaVersion == "" {
		s.SchemaVersion = SessionStateSchemaVersion
	}
	s.normalize()
	if err := s.validate(); err != nil {
		return fmt.Errorf("session-state inválido: %w", err)
	}
	if err := pathutil.EnsureStateDir(projectRoot); err != nil {
		return err
	}
	data, err := json.MarshalIndent(&s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session-state: %w", err)
	}

	var release func()
	if os.Getenv(SessionStateLockEnvVar) == "1" {
		var err error
		release, err = acquireSessionLock(projectRoot)
		if err != nil {
			return err
		}
		defer func() {
			if release != nil {
				release()
			}
		}()
	}

	final := statePath(projectRoot)
	var lastErr error
	for attempt := 0; attempt < SessionStateTmpAttempts; attempt++ {
		tmp := uniqueTmpPath(final)
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("write tmp %s: %w", tmp, err)
		}
		if err := os.Rename(tmp, final); err == nil {
			appendStateDiffEvents(projectRoot, old, s)
			return nil
		} else {
			lastErr = err
			_ = os.Remove(tmp)
			time.Sleep(SessionStateTmpBackoff)
		}
	}
	return fmt.Errorf("rename %s apos %d tentativas: %w", final, SessionStateTmpAttempts, lastErr)
}

// readSessionStateIfExists retorna o estado anterior se existir e for
// válido. Erros (arquivo inexistente, JSON inválido) são silenciados —
// o caller trata old como zero-value, fazendo o diff emitir todos os
// items como "novos".
func readSessionStateIfExists(projectRoot string) (SessionState, bool) {
	data, err := os.ReadFile(statePath(projectRoot))
	if err != nil {
		return SessionState{}, false
	}
	var s SessionState
	if err := json.Unmarshal(data, &s); err != nil {
		return SessionState{}, false
	}
	return s, true
}

// appendStateDiffEvents emite eventos no log append-only após write
// bem-sucedido. Emite sempre um state_render + N per-item (decisions/
// tasks/issues/open_questions) para items novos ou alterados.
// actions/blockers viraram tasks/issues em A-37 (D-69/D-70). Falha no
// log é reportada em stderr mas não aborta — o snapshot canônico é o
// source of truth; perda de evento é gap aceitável (ADR-003 Decisão 3).
func appendStateDiffEvents(projectRoot string, old, new SessionState) {
	now := time.Now().UTC()
	sessionID := new.Session.ID

	render := event.SessionEvent{
		Kind:      "state_render",
		Ref:       "S-" + now.Format("20060102150405"),
		Title:     "session-state.json atualizado",
		Actor:     "agent-sync",
		SessionID: sessionID,
		Details: map[string]interface{}{
			"branch":          new.Git.Branch,
			"head":            new.Git.Head,
			"decisions_total": len(new.Decisions),
			"open_questions":  len(new.OpenQuestions),
		},
	}
	if err := event.AppendEvent(projectRoot, render); err != nil {
		fmt.Fprintf(os.Stderr, "event log: %v\n", err)
	}

	for _, d := range diffDecisions(old.Decisions, new.Decisions) {
		ev := event.SessionEvent{
			Kind:      "decision",
			Ref:       d.ID,
			Title:     d.Title,
			Actor:     "agent-sync",
			SessionID: sessionID,
			Details: map[string]interface{}{
				"rationale_len": len(d.Rationale),
			},
		}
		if err := event.AppendEvent(projectRoot, ev); err != nil {
			fmt.Fprintf(os.Stderr, "event log: %v\n", err)
		}
	}
	for _, q := range diffOpenQuestions(old.OpenQuestions, new.OpenQuestions) {
		ev := event.SessionEvent{
			Kind:      "open_question",
			Ref:       q.ref,
			Title:     q.text,
			Actor:     "agent-sync",
			SessionID: sessionID,
		}
		if err := event.AppendEvent(projectRoot, ev); err != nil {
			fmt.Fprintf(os.Stderr, "event log: %v\n", err)
		}
	}
}

func diffDecisions(oldList, newList []SessionDecision) []SessionDecision {
	oldByID := map[string]SessionDecision{}
	for _, d := range oldList {
		oldByID[d.ID] = d
	}
	var out []SessionDecision
	for _, d := range newList {
		o, exists := oldByID[d.ID]
		if !exists || o.Rationale != d.Rationale || o.Title != d.Title {
			out = append(out, d)
		}
	}
	return out
}

type openQuestionDiff struct {
	ref  string
	text string
}

func diffOpenQuestions(oldList, newList []string) []openQuestionDiff {
	oldSet := map[string]bool{}
	for _, q := range oldList {
		oldSet[q] = true
	}
	var out []openQuestionDiff
	for i, q := range newList {
		if !oldSet[q] {
			out = append(out, openQuestionDiff{ref: fmt.Sprintf("Q-%d", i+1), text: q})
		}
	}
	return out
}

// uniqueTmpPath retorna path único por PID+nanoTimestamp. Garante que dois
// processos (ou dois writes no mesmo processo) não colidam no tmpfile.
// Atomic rename no final torna a serialização segura (último vence — ADR-002
// Decisão 3 lockless default).
func uniqueTmpPath(final string) string {
	return fmt.Sprintf("%s.tmp.%d.%d", final, os.Getpid(), time.Now().UnixNano())
}

// acquireSessionLock implementa o lock opcional do WriteSessionState
// (ADR-002 Decisão 3 revisada). Ativado por AGENT_SYNC_SESSION_LOCK=1;
// default é lockless com tmpfile único por PID+nanoTimestamp. Lockless é
// seguro porque atomic rename garante serialização (último vence).
//
// Quando ativo, cria <root>/.agent-sync/session-state.lock via O_CREATE|O_EXCL.
// Em contenção, retry com poll até SessionStateLockTimeout. Sem detecção de
// PID órfão (sem flock(2)): em caso de crash do holder, novo write fica
// bloqueado até o timeout. Aceitável — lock é opcional e path raro.
func acquireSessionLock(projectRoot string) (release func(), err error) {
	if err := pathutil.EnsureStateDir(projectRoot); err != nil {
		return nil, err
	}
	path := lockPath(projectRoot)
	deadline := time.Now().Add(SessionStateLockTimeout)
	for {
		f, openErr := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if openErr == nil {
			_, _ = f.WriteString(fmt.Sprintf("pid=%d\ntime=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339)))
			return func() {
				_ = f.Close()
				_ = os.Remove(path)
			}, nil
		}
		if !os.IsExist(openErr) {
			return nil, fmt.Errorf("acquireSessionLock %s: %w", path, openErr)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout (%s) aguardando lock %s", SessionStateLockTimeout, path)
		}
		time.Sleep(SessionStateLockPoll)
	}
}

// NextAction retorna a primeira SessionAction com status "pending" derivada
// de s.Tasks (legacy_kind=todo|delivery). Shim que preserva o contrato do
// subcommand 'state next-action' e do hook Cursor apos A-37 Etapas 2-4
// removerem o campo NextActions do struct. Converte SessionTask ->
// SessionAction mapeando apenas ID/Title/Status (descarta LegacyKind,
// BranchRef, PRRef, StartedAt, FinishedAt, Links).
// Migrar para retorno SessionTask em sprint futura (criar NextTask,
// remover este shim).
func NextAction(s SessionState) (SessionAction, bool) {
	for _, t := range s.Tasks {
		if t.Status != "pending" {
			continue
		}
		if t.LegacyKind != "todo" && t.LegacyKind != "delivery" {
			continue
		}
		return SessionAction{
			ID:     t.ID,
			Title:  t.Title,
			Status: t.Status,
		}, true
	}
	return SessionAction{}, false
}

func RenderStateMD(s SessionState) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# AUTO-GENERATED — edite .agent-sync/session-state.json e rode 'agent-sync state render'\n\n")
	fmt.Fprintf(&b, "# STATE — %s\n\n", s.Project.Name)
	fmt.Fprintf(&b, "> Fonte de verdade: `.agent-sync/session-state.json` (schema v%s).\n", s.SchemaVersion)
	fmt.Fprintf(&b, "> Renderizado por `agent-sync state render` em %s. Não edite à mão.\n\n", s.Session.UpdatedAt.Format(time.RFC3339))

	fmt.Fprintf(&b, "## Estado do repositório\n\n")
	fmt.Fprintf(&b, "- Branch: `%s`\n", s.Git.Branch)
	fmt.Fprintf(&b, "- HEAD: `%s`\n", s.Git.Head)
	fmt.Fprintf(&b, "- Working tree: %s\n\n", s.Git.WorkingTreeSummary)

	fmt.Fprintf(&b, "## Sessão atual\n\n")
	fmt.Fprintf(&b, "- ID: `%s`\n", s.Session.ID)
	fmt.Fprintf(&b, "- Início: %s\n", s.Session.StartedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- Última atualização: %s\n\n", s.Session.UpdatedAt.Format(time.RFC3339))

	if len(s.Decisions) > 0 {
		fmt.Fprintf(&b, "## Decisões\n\n")
		ids := sortedIDs(s.Decisions)
		for _, id := range ids {
			d := findDecision(s.Decisions, id)
			fmt.Fprintf(&b, "- **%s** %s — %s (%s)\n", d.ID, d.Title, d.Rationale, d.MadeAt.Format("2006-01-02"))
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(s.Tasks) > 0 {
		fmt.Fprintf(&b, "## Tarefas\n\n")
		ids := sortedTaskIDs(s.Tasks)
		for _, id := range ids {
			t := findTask(s.Tasks, id)
			fmt.Fprintf(&b, "- **%s** [%s] %s\n", t.ID, t.Status, t.Title)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(s.Issues) > 0 {
		fmt.Fprintf(&b, "## Issues\n\n")
		ids := sortedIssueIDs(s.Issues)
		for _, id := range ids {
			iss := findIssue(s.Issues, id)
			resolvedTag := ""
			if iss.Resolved {
				resolvedTag = " ✅"
			}
			fmt.Fprintf(&b, "- **%s** [%s]%s %s\n", iss.ID, iss.Severity, resolvedTag, iss.Title)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(s.OpenQuestions) > 0 {
		fmt.Fprintf(&b, "## Perguntas em aberto\n\n")
		for _, q := range s.OpenQuestions {
			fmt.Fprintf(&b, "- %s\n", q)
		}
		fmt.Fprintf(&b, "\n")
	}

	return b.String()
}

func sortedIDs(items interface{}) []string {
	var ids []string
	switch v := items.(type) {
	case []SessionDecision:
		for _, x := range v {
			ids = append(ids, x.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

func findDecision(items []SessionDecision, id string) SessionDecision {
	for _, x := range items {
		if x.ID == id {
			return x
		}
	}
	return SessionDecision{}
}

func findTask(items []SessionTask, id string) SessionTask {
	for _, x := range items {
		if x.ID == id {
			return x
		}
	}
	return SessionTask{}
}

func findIssue(items []SessionIssue, id string) SessionIssue {
	for _, x := range items {
		if x.ID == id {
			return x
		}
	}
	return SessionIssue{}
}

func sortedTaskIDs(items []SessionTask) []string {
	ids := make([]string, 0, len(items))
	for _, x := range items {
		ids = append(ids, x.ID)
	}
	sort.Strings(ids)
	return ids
}

func sortedIssueIDs(items []SessionIssue) []string {
	ids := make([]string, 0, len(items))
	for _, x := range items {
		ids = append(ids, x.ID)
	}
	sort.Strings(ids)
	return ids
}

// splitVersion quebra "1.2" em ("1", "2"). Retorna ("","") se não for
// major.minor (formato esperado pelo schema). Usado por acceptSchemaVersion
// para comparar apenas major — bump minor é aditivo, sem breaking change.
func splitVersion(v string) (string, string) {
	parts := strings.SplitN(v, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", ""
	}
	return parts[0], parts[1]
}

// acceptSchemaVersion devolve true se a versão atual do binário aceita
// ler/escrever payloads com a versão dada. Política: comparar apenas o
// major. Bumps minor (1.0 → 1.1) são aditivos e retrocompatíveis; bumps
// major exigem migração explícita (futuro migrate.go).
func acceptSchemaVersion(payload, current string) bool {
	payloadMajor, _ := splitVersion(payload)
	currentMajor, _ := splitVersion(current)
	if payloadMajor == "" || currentMajor == "" {
		return false
	}
	return payloadMajor == currentMajor
}
