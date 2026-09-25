// Package agentmemory implementa a camada de memória compartilhada entre CLIs
// (Claude Code, Codex, Antigravity/agy, OpenCode, Cursor) sobre libSQL local (sem sync remoto).
//
// A busca hoje é FTS5 (BM25) — sem embedding real. O schema já reserva uma coluna
// vetorial (embedding_json) para uma fase futura de busca semântica, mas ela não é
// usada por nenhuma consulta ainda.
package agentmemory

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/tursodatabase/go-libsql"
)

// DefaultDBPath devolve o caminho padrão do banco de memória compartilhada
// (~/.cache/agent-sync/memory.db), criando o diretório se necessário.
func DefaultDBPath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", fmt.Errorf("agentmemory: sem diretório de cache/home: %w", err)
		}
		dir = filepath.Join(home, ".cache")
	}
	dir = filepath.Join(dir, "agent-sync")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("agentmemory: criar %s: %w", dir, err)
	}
	return filepath.Join(dir, "memory.db"), nil
}

// Memory é uma entrada de memória compartilhada entre agentes.
type Memory struct {
	ID          string
	Agent       string // quem gravou: claude-code | codex | antigravity | opencode | cursor
	SessionID   string
	PC          string
	ProjectPath string
	ProjectID   string
	Type        string // user | feedback | project | reference
	Name        string
	Description string
	Content     string
	Scratch     bool // marca a memória como descartável; só memórias com Scratch=true podem ser removidas via Delete
	CreatedAt   time.Time
	UpdatedAt   time.Time
	// AccessedAt registra a última leitura ou escrita; alimenta o decaimento
	// de relevância no Search e a poda de PruneScratch.
	AccessedAt time.Time
}

// Meia-vida do decaimento de relevância (inspirado no Ebbinghaus/MemoryBank,
// arXiv 2305.10250): memórias não acessadas há ~30 dias recebem pouca penalidade,
// memórias tocadas ontem pesam mais que as tocadas há um mês.
// Relevância textual (bm25) continua dominando; recência só desempata/dá empurrão.
const searchDecayHalfLife = 30 * 24 * time.Hour

// searchDecayWeight limita a contribuição máxima do frescor ao ranking.
// bm25 típico fica em -1..-20; 0.5 move ~1 casa sem dominar a relevância textual.
const searchDecayWeight = 0.5

// scoredMemory é um resultado do Search com o score bm25 bruto preservado
// para o re-rank em Go (a query SQL não aplica o decaimento direto).
type scoredMemory struct {
	Memory
	bm25 float64 // bm25() do SQLite é NEGATIVO: menor = mais relevante
}

// schemaStatements executa uma DDL por vez: o driver libsql não aceita múltiplas
// statements separadas por ';' em um único Exec (falha silenciosamente após a 1ª).
var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS memories (
		id          TEXT PRIMARY KEY,
		agent       TEXT NOT NULL,
		session_id  TEXT NOT NULL DEFAULT '',
		type        TEXT NOT NULL,
		name        TEXT NOT NULL,
		description TEXT NOT NULL,
		content     TEXT NOT NULL,
		scratch     INTEGER NOT NULL DEFAULT 0,
		embedding_json TEXT NOT NULL DEFAULT '',
		created_at  TEXT NOT NULL,
		updated_at  TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS memory_sync_state (type TEXT NOT NULL, name TEXT NOT NULL, content_hash TEXT NOT NULL, PRIMARY KEY(type, name))`,
	`CREATE TABLE IF NOT EXISTS memory_sync_state_v2 (project_id TEXT NOT NULL, type TEXT NOT NULL, name TEXT NOT NULL, content_hash TEXT NOT NULL, PRIMARY KEY(project_id, type, name))`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
		name, description, content, content='memories', content_rowid='rowid'
	)`,
	`CREATE TRIGGER IF NOT EXISTS memories_ai AFTER INSERT ON memories BEGIN
		INSERT INTO memories_fts(rowid, name, description, content) VALUES (new.rowid, new.name, new.description, new.content);
	END`,
	`CREATE TRIGGER IF NOT EXISTS memories_ad AFTER DELETE ON memories BEGIN
		INSERT INTO memories_fts(memories_fts, rowid, name, description, content) VALUES ('delete', old.rowid, old.name, old.description, old.content);
	END`,
	`CREATE TRIGGER IF NOT EXISTS memories_au AFTER UPDATE ON memories BEGIN
		INSERT INTO memories_fts(memories_fts, rowid, name, description, content) VALUES ('delete', old.rowid, old.name, old.description, old.content);
		INSERT INTO memories_fts(rowid, name, description, content) VALUES (new.rowid, new.name, new.description, new.content);
	END`,
}

func applySchema(db *sql.DB) error {
	for _, stmt := range schemaStatements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("agentmemory: aplicar schema: %w", err)
		}
	}
	// Migração leve para bancos criados antes da coluna 'scratch' existir.
	// CREATE TABLE IF NOT EXISTS não adiciona colunas a uma tabela já existente.
	if _, err := db.Exec(`ALTER TABLE memories ADD COLUMN scratch INTEGER NOT NULL DEFAULT 0`); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("agentmemory: migrar coluna scratch: %w", err)
		}
	}
	for _, column := range []string{"pc", "project_path", "project_id", "accessed_at"} {
		if _, err := db.Exec("ALTER TABLE memories ADD COLUMN " + column + " TEXT NOT NULL DEFAULT ''"); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("agentmemory: migrar coluna %s: %w", column, err)
		}
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS memories_type_name`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS memories_project_type_name ON memories(project_id, type, name)`); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO memory_sync_state_v2(project_id,type,name,content_hash) SELECT '',type,name,content_hash FROM memory_sync_state`); err != nil {
		return err
	}
	return nil
}

// Store é a conexão com o banco de memória compartilhada.
type Store struct {
	db *sql.DB
}

// Open abre (criando se necessário) o banco de memória em dbPath.
func Open(dbPath string) (*Store, error) {
	db, err := sql.Open("libsql", "file:"+dbPath)
	if err != nil {
		return nil, fmt.Errorf("agentmemory: abrir %s: %w", dbPath, err)
	}
	if err := applySchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close fecha a conexão com o banco.
func (s *Store) Close() error {
	return s.db.Close()
}

// Upsert grava ou atualiza uma memória (chave única: type+name).
func (s *Store) Upsert(m Memory) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if m.ID == "" {
		m.ID = fmt.Sprintf("%s:%s:%d", m.Type, m.Name, time.Now().UnixNano())
	}
	_, err := s.db.Exec(`
		INSERT INTO memories (id, agent, session_id, pc, project_path, project_id, type, name, description, content, scratch, embedding_json, created_at, updated_at, accessed_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?, ?, ?)
        ON CONFLICT(project_id, type, name) DO UPDATE SET
            agent = excluded.agent,
            session_id = excluded.session_id,
            pc = excluded.pc,
            project_path = excluded.project_path,
            description = excluded.description,
			content = excluded.content,
			scratch = excluded.scratch,
			updated_at = excluded.updated_at
	`, m.ID, m.Agent, m.SessionID, m.PC, m.ProjectPath, m.ProjectID, m.Type, m.Name, m.Description, m.Content, m.Scratch, now, now, now)
	if err != nil {
		return fmt.Errorf("agentmemory: upsert %s/%s: %w", m.Type, m.Name, err)
	}
	return nil
}

// Touch registra um acesso a uma memória (updated accessed_at para o timestamp
// atual UTC RFC3339). O erro é retornado ao chamador; leituras (Get/Search)
// chamam Touch mas não falham se ele falhar — apenas logam em stderr.
func (s *Store) Touch(projectID, typ, name string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`UPDATE memories SET accessed_at = ? WHERE project_id = ? AND type = ? AND name = ?`, now, projectID, typ, name); err != nil {
		return fmt.Errorf("agentmemory: touch %s/%s: %w", typ, name, err)
	}
	return nil
}

// PruneScratch remove memórias descartáveis (scratch=true) cujo último acesso
// seja mais antigo que olderThan, retornando quantas linhas foram removidas.
// Para linhas legadas sem accessed_at, usa updated_at como aproximação.
// Memórias permanentes (scratch=false) NUNCA são tocadas — mesma invariante
// de Delete/DeleteScoped, que recusam remoção manual (ErrNotScratch).
func (s *Store) PruneScratch(olderThan time.Duration) (int, error) {
	if olderThan <= 0 {
		return 0, fmt.Errorf("agentmemory: prune scratch com duração não-positiva")
	}
	cutoff := time.Now().UTC().Add(-olderThan).Format(time.RFC3339)
	res, err := s.db.Exec(`
		DELETE FROM memories WHERE scratch = 1 AND
			((accessed_at != '' AND accessed_at < ?) OR (accessed_at = '' AND updated_at < ?))
	`, cutoff, cutoff)
	if err != nil {
		return 0, fmt.Errorf("agentmemory: prune scratch: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("agentmemory: prune scratch: %w", err)
	}
	return int(n), nil
}

// ErrNotScratch é retornado quando Delete é chamado numa memória com Scratch=false:
// memórias "de verdade" (decisões, feedback, contexto de projeto) não são removíveis
// por essa via — só quem gravou como descartável pode limpar depois.
var ErrNotScratch = fmt.Errorf("agentmemory: memória não está marcada como scratch; remoção recusada")

// Delete remove uma memória pelo nome, mas só se ela foi gravada com Scratch=true.
// Retorna ErrNotScratch se a memória existir e não for scratch, e (false, nil) se não existir.
func (s *Store) Delete(name string) (bool, error) {
	m, err := s.Get(name)
	if err != nil {
		return false, err
	}
	if m == nil {
		return false, nil
	}
	if !m.Scratch {
		return false, ErrNotScratch
	}
	if _, err := s.db.Exec(`DELETE FROM memories WHERE project_id = ? AND type = ? AND name = ?`, m.ProjectID, m.Type, name); err != nil {
		return false, fmt.Errorf("agentmemory: delete %s: %w", name, err)
	}
	return true, nil
}

func (s *Store) DeleteScoped(name, projectID string) (bool, error) {
	m, err := s.GetScoped(name, projectID)
	if err != nil {
		return false, err
	}
	if m == nil {
		return false, nil
	}
	if !m.Scratch {
		return false, ErrNotScratch
	}
	if _, err := s.db.Exec(`DELETE FROM memories WHERE project_id=? AND type=? AND name=?`, projectID, m.Type, name); err != nil {
		return false, err
	}
	return true, nil
}

// Get busca pelo nome quando ele identifica apenas uma memória.
func (s *Store) Get(name string) (*Memory, error) {
	rows, err := s.db.Query(`SELECT id, agent, session_id, pc, project_path, project_id, type, name, description, content, scratch, created_at, updated_at, accessed_at FROM memories WHERE name = ? LIMIT 2`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items, err := scanMemories(rows)
	if err != nil {
		return nil, err
	}
	if len(items) > 1 {
		return nil, fmt.Errorf("agentmemory: nome ambíguo; informe o projeto")
	}
	if len(items) == 0 {
		return nil, nil
	}
	s.touchHit(items[0])
	return &items[0], nil
}

func (s *Store) GetScoped(name, projectID string) (*Memory, error) {
	row := s.db.QueryRow(`SELECT id, agent, session_id, pc, project_path, project_id, type, name, description, content, scratch, created_at, updated_at, accessed_at FROM memories WHERE name = ? AND project_id = ? LIMIT 1`, name, projectID)
	var m Memory
	var created, updated, accessed string
	if err := row.Scan(&m.ID, &m.Agent, &m.SessionID, &m.PC, &m.ProjectPath, &m.ProjectID, &m.Type, &m.Name, &m.Description, &m.Content, &m.Scratch, &created, &updated, &accessed); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339, created)
	m.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	m.AccessedAt, _ = time.Parse(time.RFC3339, accessed)
	s.touchHit(m)
	return &m, nil
}

// touchHit registra o acesso de uma leitura bem-sucedida. Falha de Touch não
// derruba a leitura que já aconteceu — só avisa em stderr, já que o dado
// pedido pelo chamador já foi retornado corretamente.
func (s *Store) touchHit(m Memory) {
	if err := s.Touch(m.ProjectID, m.Type, m.Name); err != nil {
		fmt.Fprintf(os.Stderr, "agentmemory: touch %s/%s: %v\n", m.Type, m.Name, err)
	}
}

// ScopeFilter restringe a origem sem alterar a busca textual.
type ScopeFilter struct{ ProjectID, PC, ProjectPath string }

// EventKind é o catálogo aceito em eventos emitidos pelos hooks/skills.
//
// Catálogo unificado (5 novos do memory-mcp + 5 herdados do JSONL legado
// session_event): decision, hypothesis_validated, task_completed,
// task_delegated, guard_nudge, action, blocker, open_question, state_render,
// note. Manter a união preserva o que WriteSessionState e os hooks de
// antigravity/claude/codex já gravavam antes da migração.
var EventKind = map[string]bool{
	"decision":             true,
	"hypothesis_validated": true,
	"task_completed":       true,
	"task_delegated":       true,
	"guard_nudge":          true,
	// herdados do JSONL session-event.jsonl (preservam semântica da migração):
	"action":        true,
	"blocker":       true,
	"open_question": true,
	"state_render":  true,
	"note":          true,
}

// ListEvents lista memórias do tipo "event" ordenadas do mais recente para o mais
// antigo. Filtra por kind (opcional, vazio = todos), since (RFC3339, opcional)
// e reaproveita o mesmo ScopeFilter de List/Search. Eventos são os "rastros"
// emitidos pelos hooks/skills e não substituem memórias duradouras — por isso
// o tipo é fixo em "event" aqui (a flag scratch fica a cargo do chamador).
func (s *Store) ListEvents(kind, since string, limit int, filters ...ScopeFilter) ([]Memory, error) {
	var filter ScopeFilter
	if len(filters) > 0 {
		filter = filters[0]
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`
		SELECT id, agent, session_id, pc, project_path, project_id, type, name, description, content, scratch, created_at, updated_at, accessed_at
		FROM memories
		WHERE type = 'event'
			AND (? = '' OR name LIKE ?)
			AND (? = '' OR updated_at >= ?)
			AND (? = '' OR project_id = ?)
			AND (? = '' OR pc = ?)
			AND (? = '' OR project_path = ?)
		ORDER BY updated_at DESC
		LIMIT ?
	`, kind, kind+"%", since, since, filter.ProjectID, filter.ProjectID, filter.PC, filter.PC, filter.ProjectPath, filter.ProjectPath, limit)
	if err != nil {
		return nil, fmt.Errorf("agentmemory: list events: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

// ListEventsBySession lista eventos filtrados por session_id (campos Memory.SessionID).
// Origem: A-57 memory_read_session (S-0.2 Camada 1, ses_f2b646cd, 2026-09-24).
// Adicionada como funcao NOVA (nao altera signature de ListEvents) para nao
// quebrar os 3 callsites de store_test.go (linhas 316/330/340) nem o call em
// memory-mcp main.go:471. sessionID vazio = sem filtro (= comportamento de ListEvents sem 'since').
func (s *Store) ListEventsBySession(sessionID, kind, since string, limit int, filters ...ScopeFilter) ([]Memory, error) {
	var filter ScopeFilter
	if len(filters) > 0 {
		filter = filters[0]
	}
	if limit <= 0 {
		limit = 100
	}
	if sessionID == "" {
		return nil, fmt.Errorf("agentmemory: ListEventsBySession requer sessionID nao vazio")
	}
	rows, err := s.db.Query(`
		SELECT id, agent, session_id, pc, project_path, project_id, type, name, description, content, scratch, created_at, updated_at, accessed_at
		FROM memories
		WHERE type = 'event'
			AND session_id = ?
			AND (? = '' OR name LIKE ?)
			AND (? = '' OR updated_at >= ?)
			AND (? = '' OR project_id = ?)
			AND (? = '' OR pc = ?)
			AND (? = '' OR project_path = ?)
		ORDER BY updated_at DESC
		LIMIT ?
	`, sessionID, kind, kind+"%", since, since, filter.ProjectID, filter.ProjectID, filter.PC, filter.PC, filter.ProjectPath, filter.ProjectPath, limit)
	if err != nil {
		return nil, fmt.Errorf("agentmemory: list events by session: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

// List lista memórias, opcionalmente filtrando por agente e/ou tipo (vazio = sem filtro).
func (s *Store) List(agent, typ string, limit int, filters ...ScopeFilter) ([]Memory, error) {
	var filter ScopeFilter
	if len(filters) > 0 {
		filter = filters[0]
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`
		SELECT id, agent, session_id, pc, project_path, project_id, type, name, description, content, scratch, created_at, updated_at, accessed_at
		FROM memories
		WHERE (? = '' OR agent = ?) AND (? = '' OR type = ?)
            AND (? = '' OR project_id = ?) AND (? = '' OR pc = ?) AND (? = '' OR project_path = ?)
		ORDER BY updated_at DESC
		LIMIT ?
	`, agent, agent, typ, typ, filter.ProjectID, filter.ProjectID, filter.PC, filter.PC, filter.ProjectPath, filter.ProjectPath, limit)
	if err != nil {
		return nil, fmt.Errorf("agentmemory: list: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

// Search faz busca por relevância (FTS5/BM25) em name+description+content,
// com um boost de recência (decaimento exponencial sobre accessed_at) que só
// desempata/ajusta o ranking — bm25 textual continua dominando.
//
// O decaimento não é expresso em SQL puro (libSQL sem funções exp/pow
// estáveis entre versões seria frágil); em vez disso trazemos até 3x o limite
// pedido já ordenado por bm25 e reordenamos em Go, que é simples de testar.
func (s *Store) Search(query, agent, typ string, limit int, filters ...ScopeFilter) ([]Memory, error) {
	var filter ScopeFilter
	if len(filters) > 0 {
		filter = filters[0]
	}
	if limit <= 0 {
		limit = 10
	}
	fetchLimit := limit * 3
	rows, err := s.db.Query(`
		SELECT m.id, m.agent, m.session_id, m.pc, m.project_path, m.project_id, m.type, m.name, m.description, m.content, m.scratch, m.created_at, m.updated_at, m.accessed_at, bm25(memories_fts) AS bm25
		FROM memories_fts f
		JOIN memories m ON m.rowid = f.rowid
		WHERE memories_fts MATCH ?
			AND (? = '' OR m.agent = ?)
			AND (? = '' OR m.type = ?)
            AND (? = '' OR m.project_id = ?) AND (? = '' OR m.pc = ?) AND (? = '' OR m.project_path = ?)
		ORDER BY bm25(memories_fts)
		LIMIT ?
	`, ftsQuery(query), agent, agent, typ, typ, filter.ProjectID, filter.ProjectID, filter.PC, filter.PC, filter.ProjectPath, filter.ProjectPath, fetchLimit)
	if err != nil {
		return nil, fmt.Errorf("agentmemory: search %q: %w", query, err)
	}
	defer rows.Close()
	scored, err := scanScoredMemories(rows)
	if err != nil {
		return nil, err
	}
	rankSearchResults(scored)
	if len(scored) > limit {
		scored = scored[:limit]
	}
	out := make([]Memory, len(scored))
	for i, sm := range scored {
		out[i] = sm.Memory
		s.touchHit(sm.Memory)
	}
	return out, nil
}

// rankSearchResults reordena por bm25 ajustado pelo decaimento de recência.
// bm25 do SQLite é NEGATIVO (mais negativo = mais relevante); o ajuste soma
// um valor >= 0 que cresce com a idade do acesso, então nunca torna um item
// mais relevante que outro com bm25 melhor por uma margem maior que
// searchDecayWeight — só desempata/reordena vizinhos próximos.
func rankSearchResults(scored []scoredMemory) {
	now := time.Now().UTC()
	sort.SliceStable(scored, func(i, j int) bool {
		return adjustedScore(scored[i], now) < adjustedScore(scored[j], now)
	})
}

func adjustedScore(sm scoredMemory, now time.Time) float64 {
	if sm.AccessedAt.IsZero() {
		return sm.bm25
	}
	age := now.Sub(sm.AccessedAt)
	if age < 0 {
		age = 0
	}
	decay := math.Exp2(-age.Hours() / searchDecayHalfLife.Hours())
	// decay em [0,1]: 1 = acesso agora, ->0 quanto mais velho. Penalidade
	// (valor positivo somado ao bm25 negativo) cresce conforme o acesso
	// envelhece: (1-decay) vai de 0 (recente) a 1 (muito antigo).
	return sm.bm25 + searchDecayWeight*(1-decay)
}

// ftsQuery escapa a query do usuário para MATCH tratando cada palavra como termo
// literal (aspas), evitando que caracteres como '-', ':' ou palavras reservadas
// (AND/OR/NOT) sejam interpretados como operadores de sintaxe do FTS5.
func ftsQuery(q string) string {
	words := strings.Fields(q)
	if len(words) == 0 {
		return `""`
	}
	quoted := make([]string, len(words))
	for i, w := range words {
		quoted[i] = `"` + strings.ReplaceAll(w, `"`, `""`) + `"`
	}
	return strings.Join(quoted, " ")
}

func scanMemories(rows *sql.Rows) ([]Memory, error) {
	var out []Memory
	for rows.Next() {
		var m Memory
		var created, updated, accessed string
		if err := rows.Scan(&m.ID, &m.Agent, &m.SessionID, &m.PC, &m.ProjectPath, &m.ProjectID, &m.Type, &m.Name, &m.Description, &m.Content, &m.Scratch, &created, &updated, &accessed); err != nil {
			return nil, fmt.Errorf("agentmemory: scan: %w", err)
		}
		m.CreatedAt, _ = time.Parse(time.RFC3339, created)
		m.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
		m.AccessedAt, _ = time.Parse(time.RFC3339, accessed)
		out = append(out, m)
	}
	return out, rows.Err()
}

func scanScoredMemories(rows *sql.Rows) ([]scoredMemory, error) {
	var out []scoredMemory
	for rows.Next() {
		var sm scoredMemory
		var created, updated, accessed string
		if err := rows.Scan(&sm.ID, &sm.Agent, &sm.SessionID, &sm.PC, &sm.ProjectPath, &sm.ProjectID, &sm.Type, &sm.Name, &sm.Description, &sm.Content, &sm.Scratch, &created, &updated, &accessed, &sm.bm25); err != nil {
			return nil, fmt.Errorf("agentmemory: scan: %w", err)
		}
		sm.CreatedAt, _ = time.Parse(time.RFC3339, created)
		sm.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
		sm.AccessedAt, _ = time.Parse(time.RFC3339, accessed)
		out = append(out, sm)
	}
	return out, rows.Err()
}
