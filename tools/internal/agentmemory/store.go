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
	"os"
	"path/filepath"
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
	for _, column := range []string{"pc", "project_path", "project_id"} {
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
		INSERT INTO memories (id, agent, session_id, pc, project_path, project_id, type, name, description, content, scratch, embedding_json, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?, ?)
        ON CONFLICT(project_id, type, name) DO UPDATE SET
            agent = excluded.agent,
            session_id = excluded.session_id,
            pc = excluded.pc,
            project_path = excluded.project_path,
            description = excluded.description,
			content = excluded.content,
			scratch = excluded.scratch,
			updated_at = excluded.updated_at
	`, m.ID, m.Agent, m.SessionID, m.PC, m.ProjectPath, m.ProjectID, m.Type, m.Name, m.Description, m.Content, m.Scratch, now, now)
	if err != nil {
		return fmt.Errorf("agentmemory: upsert %s/%s: %w", m.Type, m.Name, err)
	}
	return nil
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
	rows, err := s.db.Query(`SELECT id, agent, session_id, pc, project_path, project_id, type, name, description, content, scratch, created_at, updated_at FROM memories WHERE name = ? LIMIT 2`, name)
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
	return &items[0], nil
}

func (s *Store) GetScoped(name, projectID string) (*Memory, error) {
	row := s.db.QueryRow(`SELECT id, agent, session_id, pc, project_path, project_id, type, name, description, content, scratch, created_at, updated_at FROM memories WHERE name = ? AND project_id = ? LIMIT 1`, name, projectID)
	var m Memory
	var created, updated string
	if err := row.Scan(&m.ID, &m.Agent, &m.SessionID, &m.PC, &m.ProjectPath, &m.ProjectID, &m.Type, &m.Name, &m.Description, &m.Content, &m.Scratch, &created, &updated); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339, created)
	m.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	return &m, nil
}

// ScopeFilter restringe a origem sem alterar a busca textual.
type ScopeFilter struct{ ProjectID, PC, ProjectPath string }

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
		SELECT id, agent, session_id, pc, project_path, project_id, type, name, description, content, scratch, created_at, updated_at
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

// Search faz busca por relevância (FTS5/BM25) em name+description+content.
func (s *Store) Search(query, agent, typ string, limit int, filters ...ScopeFilter) ([]Memory, error) {
	var filter ScopeFilter
	if len(filters) > 0 {
		filter = filters[0]
	}
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(`
		SELECT m.id, m.agent, m.session_id, m.pc, m.project_path, m.project_id, m.type, m.name, m.description, m.content, m.scratch, m.created_at, m.updated_at
		FROM memories_fts f
		JOIN memories m ON m.rowid = f.rowid
		WHERE memories_fts MATCH ?
			AND (? = '' OR m.agent = ?)
			AND (? = '' OR m.type = ?)
            AND (? = '' OR m.project_id = ?) AND (? = '' OR m.pc = ?) AND (? = '' OR m.project_path = ?)
		ORDER BY bm25(memories_fts)
		LIMIT ?
	`, ftsQuery(query), agent, agent, typ, typ, filter.ProjectID, filter.ProjectID, filter.PC, filter.PC, filter.ProjectPath, filter.ProjectPath, limit)
	if err != nil {
		return nil, fmt.Errorf("agentmemory: search %q: %w", query, err)
	}
	defer rows.Close()
	return scanMemories(rows)
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
		var created, updated string
		if err := rows.Scan(&m.ID, &m.Agent, &m.SessionID, &m.PC, &m.ProjectPath, &m.ProjectID, &m.Type, &m.Name, &m.Description, &m.Content, &m.Scratch, &created, &updated); err != nil {
			return nil, fmt.Errorf("agentmemory: scan: %w", err)
		}
		m.CreatedAt, _ = time.Parse(time.RFC3339, created)
		m.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
		out = append(out, m)
	}
	return out, rows.Err()
}
