package agentmemory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/matheusdutra/token-tools/internal/secretscan"
)

const (
	syncPageSize          = 100
	maxSyncRecords        = 10000
	maxMemoryContentBytes = 1024 * 1024
)

const remoteSchema = `CREATE TABLE IF NOT EXISTS agent_sync_memories (
	id TEXT NOT NULL, agent TEXT NOT NULL, session_id TEXT NOT NULL,
	type TEXT NOT NULL, name TEXT NOT NULL, description TEXT NOT NULL,
	content TEXT NOT NULL, content_hash TEXT NOT NULL,
	PRIMARY KEY(type, name)
)`

const remoteSchemaV2 = `CREATE TABLE IF NOT EXISTS agent_sync_memories_v2 (
    id TEXT NOT NULL, agent TEXT NOT NULL, session_id TEXT NOT NULL,
    pc TEXT NOT NULL, project_path TEXT NOT NULL, project_id TEXT NOT NULL,
    type TEXT NOT NULL, name TEXT NOT NULL, description TEXT NOT NULL,
    content TEXT NOT NULL, content_hash TEXT NOT NULL,
    PRIMARY KEY(project_id, type, name)
)`

func prepareRemote(ctx context.Context, db *sql.DB) error {
	for _, stmt := range []string{remoteSchema, remoteSchemaV2,
		`INSERT OR IGNORE INTO agent_sync_memories_v2(id,agent,session_id,pc,project_path,project_id,type,name,description,content,content_hash) SELECT id,agent,session_id,'','','',type,name,description,content,content_hash FROM agent_sync_memories`} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("preparar sincronização remota: %w", err)
		}
	}
	return nil
}

type memoryKey struct{ projectID, typ, name string }

type syncEntry struct {
	Memory
	hash string
}

func keyFor(m Memory) memoryKey { return memoryKey{m.ProjectID, m.Type, m.Name} }

func conflictID(key memoryKey) string {
	sum := sha256.Sum256([]byte(key.projectID + "\x00" + key.typ + "\x00" + key.name))
	return hex.EncodeToString(sum[:6])
}

func validateSyncMemory(m Memory) error {
	if m.Type == "" || m.Name == "" || len(m.Type) > 64 || len(m.Name) > 256 || len(m.Agent) > 64 || len(m.SessionID) > 256 || len(m.Description) > 16384 || len(m.PC) > 256 || len(m.ProjectPath) > 4096 || len(m.ProjectID) > 256 || len(m.Content) > maxMemoryContentBytes {
		return fmt.Errorf("registro de memória excede os limites de sincronização")
	}
	return nil
}

func contentHash(m Memory) string {
	values := []string{m.Agent, m.SessionID, m.Type, m.Name, m.Description, m.Content}
	if m.PC != "" || m.ProjectPath != "" || m.ProjectID != "" {
		values = append(values, m.PC, m.ProjectPath, m.ProjectID)
	}
	data, _ := json.Marshal(values)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// OpenRemote abre um banco Turso/libSQL remoto sem colocar o token em logs.
func OpenRemote(address, token string) (*sql.DB, error) {
	parsed, err := url.Parse(address)
	if err != nil || parsed == nil || parsed.Scheme != "libsql" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || token == "" {
		return nil, fmt.Errorf("URL Turso ou token inválido")
	}
	query := parsed.Query()
	query.Set("authToken", token)
	parsed.RawQuery = query.Encode()
	db, err := sql.Open("libsql", parsed.String())
	if err != nil {
		return nil, fmt.Errorf("abrir banco Turso: %w", err)
	}
	return db, nil
}

func loadLocal(ctx context.Context, db *sql.DB) (map[memoryKey]syncEntry, error) {
	items := map[memoryKey]syncEntry{}
	for offset := 0; ; offset += syncPageSize {
		if offset >= maxSyncRecords {
			return nil, fmt.Errorf("limite de memórias para sincronização excedido")
		}
		rows, err := db.QueryContext(ctx, `SELECT id, agent, session_id, pc, project_path, project_id, type, name, description, content FROM memories WHERE scratch = 0 ORDER BY project_id, type, name LIMIT ? OFFSET ?`, syncPageSize, offset)
		if err != nil {
			return nil, err
		}
		count := 0
		for rows.Next() {
			var m Memory
			if err := rows.Scan(&m.ID, &m.Agent, &m.SessionID, &m.PC, &m.ProjectPath, &m.ProjectID, &m.Type, &m.Name, &m.Description, &m.Content); err != nil {
				rows.Close()
				return nil, err
			}
			if err := validateSyncMemory(m); err != nil {
				rows.Close()
				return nil, err
			}
			items[keyFor(m)] = syncEntry{m, contentHash(m)}
			count++
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		if count < syncPageSize {
			return items, nil
		}
	}
}

func loadRemote(ctx context.Context, db *sql.DB) (map[memoryKey]syncEntry, error) {
	items := map[memoryKey]syncEntry{}
	for offset := 0; ; offset += syncPageSize {
		if offset >= maxSyncRecords {
			return nil, fmt.Errorf("limite de memórias para sincronização excedido")
		}
		rows, err := db.QueryContext(ctx, `SELECT id, agent, session_id, pc, project_path, project_id, type, name, description, content, content_hash FROM agent_sync_memories_v2 ORDER BY project_id, type, name LIMIT ? OFFSET ?`, syncPageSize, offset)
		if err != nil {
			return nil, err
		}
		count := 0
		for rows.Next() {
			var entry syncEntry
			m := &entry.Memory
			if err := rows.Scan(&m.ID, &m.Agent, &m.SessionID, &m.PC, &m.ProjectPath, &m.ProjectID, &m.Type, &m.Name, &m.Description, &m.Content, &entry.hash); err != nil {
				rows.Close()
				return nil, err
			}
			if err := validateSyncMemory(*m); err != nil {
				rows.Close()
				return nil, err
			}
			if entry.hash != contentHash(*m) {
				rows.Close()
				return nil, fmt.Errorf("registro remoto inconsistente")
			}
			items[keyFor(*m)] = entry
			count++
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		if count < syncPageSize {
			return items, nil
		}
	}
}

func loadBaseline(ctx context.Context, db *sql.DB) (map[memoryKey]string, error) {
	items := map[memoryKey]string{}
	for offset := 0; ; offset += syncPageSize {
		if offset >= maxSyncRecords {
			return nil, fmt.Errorf("limite de memórias para sincronização excedido")
		}
		rows, err := db.QueryContext(ctx, `SELECT project_id, type, name, content_hash FROM memory_sync_state_v2 ORDER BY project_id, type, name LIMIT ? OFFSET ?`, syncPageSize, offset)
		if err != nil {
			return nil, err
		}
		count := 0
		for rows.Next() {
			var key memoryKey
			var hash string
			if err := rows.Scan(&key.projectID, &key.typ, &key.name, &hash); err != nil {
				rows.Close()
				return nil, err
			}
			items[key] = hash
			count++
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		if count < syncPageSize {
			return items, nil
		}
	}
}

func saveBaseline(ctx context.Context, db *sql.DB, key memoryKey, hash string) error {
	_, err := db.ExecContext(ctx, `INSERT INTO memory_sync_state_v2(project_id, type, name, content_hash) VALUES (?, ?, ?, ?) ON CONFLICT(project_id, type, name) DO UPDATE SET content_hash = excluded.content_hash`, key.projectID, key.typ, key.name, hash)
	return err
}

func sortedKeys(items map[memoryKey]syncEntry) []memoryKey {
	keys := make([]memoryKey, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].projectID != keys[j].projectID {
			return keys[i].projectID < keys[j].projectID
		}
		if keys[i].typ != keys[j].typ {
			return keys[i].typ < keys[j].typ
		}
		return keys[i].name < keys[j].name
	})
	return keys
}

// Pull traz registros remotos sem sobrescrever alterações locais divergentes.
func (s *Store) Pull(ctx context.Context, remote *sql.DB) (int, error) {
	if err := prepareRemote(ctx, remote); err != nil {
		return 0, err
	}
	local, err := loadLocal(ctx, s.db)
	if err != nil {
		return 0, err
	}
	cloud, err := loadRemote(ctx, remote)
	if err != nil {
		return 0, err
	}
	baseline, err := loadBaseline(ctx, s.db)
	if err != nil {
		return 0, err
	}
	for _, key := range sortedKeys(cloud) {
		remoteEntry := cloud[key]
		localEntry, exists := local[key]
		baseHash, hadBase := baseline[key]
		if exists && localEntry.hash != remoteEntry.hash && (!hadBase || localEntry.hash != baseHash) {
			if hadBase && remoteEntry.hash == baseHash {
				continue
			}
			return 0, fmt.Errorf("conflito de memória %s; nenhuma alteração aplicada", conflictID(key))
		}
	}
	applied := 0
	for _, key := range sortedKeys(cloud) {
		remoteEntry := cloud[key]
		localEntry, exists := local[key]
		if !exists || localEntry.hash != remoteEntry.hash {
			if exists && baseline[key] == remoteEntry.hash {
				continue
			}
			if err := s.Upsert(remoteEntry.Memory); err != nil {
				return applied, err
			}
			applied++
		}
		if err := saveBaseline(ctx, s.db, key, remoteEntry.hash); err != nil {
			return applied, err
		}
	}
	return applied, nil
}

// Push envia alterações locais apenas se o remoto não mudou desde a última sincronização.
func (s *Store) Push(ctx context.Context, remote *sql.DB) (int, error) {
	if err := prepareRemote(ctx, remote); err != nil {
		return 0, err
	}
	local, err := loadLocal(ctx, s.db)
	if err != nil {
		return 0, err
	}
	cloud, err := loadRemote(ctx, remote)
	if err != nil {
		return 0, err
	}
	baseline, err := loadBaseline(ctx, s.db)
	if err != nil {
		return 0, err
	}
	for _, key := range sortedKeys(local) {
		entry := local[key]
		if _, found := secretscan.Redact(strings.Join([]string{entry.Name, entry.Description, entry.Content}, "\n")); len(found) > 0 {
			return 0, fmt.Errorf("memória contém possível segredo; envio cancelado")
		}
		remoteEntry, exists := cloud[key]
		if exists && entry.hash == remoteEntry.hash {
			continue
		}
		baseHash, hadBase := baseline[key]
		if (exists && (!hadBase || remoteEntry.hash != baseHash)) || (!exists && hadBase && entry.hash != baseHash) {
			return 0, fmt.Errorf("conflito de memória %s; nenhuma alteração enviada", conflictID(key))
		}
	}
	tx, err := remote.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("iniciar envio: %w", err)
	}
	defer tx.Rollback()
	sent := 0
	for _, key := range sortedKeys(local) {
		entry := local[key]
		remoteEntry, exists := cloud[key]
		if exists && remoteEntry.hash == entry.hash {
			continue
		}
		var result sql.Result
		if exists {
			result, err = tx.ExecContext(ctx, `UPDATE agent_sync_memories_v2 SET agent=?, session_id=?, pc=?, project_path=?, description=?, content=?, content_hash=? WHERE project_id=? AND type=? AND name=? AND content_hash=?`, entry.Agent, entry.SessionID, entry.PC, entry.ProjectPath, entry.Description, entry.Content, entry.hash, key.projectID, key.typ, key.name, remoteEntry.hash)
		} else {
			result, err = tx.ExecContext(ctx, `INSERT INTO agent_sync_memories_v2(id, agent, session_id, pc, project_path, project_id, type, name, description, content, content_hash) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(project_id, type, name) DO NOTHING`, entry.ID, entry.Agent, entry.SessionID, entry.PC, entry.ProjectPath, entry.ProjectID, entry.Type, entry.Name, entry.Description, entry.Content, entry.hash)
		}
		if err != nil {
			return 0, fmt.Errorf("enviar memória: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		if affected != 1 {
			return 0, fmt.Errorf("conflito de memória %s durante envio; nenhuma alteração enviada", conflictID(key))
		}
		sent++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("confirmar envio: %w", err)
	}
	confirmed, err := loadRemote(ctx, remote)
	if err != nil {
		return sent, fmt.Errorf("verificar envio remoto: %w", err)
	}
	for _, key := range sortedKeys(local) {
		entry, exists := confirmed[key]
		if !exists || entry.hash != local[key].hash {
			return sent, fmt.Errorf("envio remoto não confirmado para memória %s", conflictID(key))
		}
	}
	for _, key := range sortedKeys(local) {
		if err := saveBaseline(ctx, s.db, key, local[key].hash); err != nil {
			return sent, err
		}
	}
	return sent, nil
}

// Resolve aplica a escolha explícita do usuário para uma memória em conflito.
func (s *Store) Resolve(ctx context.Context, remote *sql.DB, id, choice string) error {
	if id == "" || (choice != "local" && choice != "remote") {
		return fmt.Errorf("informe conflito e escolha local ou remote")
	}
	if err := prepareRemote(ctx, remote); err != nil {
		return err
	}
	local, err := loadLocal(ctx, s.db)
	if err != nil {
		return err
	}
	cloud, err := loadRemote(ctx, remote)
	if err != nil {
		return err
	}
	var selected *memoryKey
	for key := range local {
		if conflictID(key) == id {
			copy := key
			selected = &copy
		}
	}
	for key := range cloud {
		if conflictID(key) == id && selected == nil {
			copy := key
			selected = &copy
		}
	}
	if selected == nil {
		return fmt.Errorf("conflito não encontrado")
	}
	if choice == "remote" {
		entry, exists := cloud[*selected]
		if !exists {
			return fmt.Errorf("versão remota ausente")
		}
		if err := s.Upsert(entry.Memory); err != nil {
			return err
		}
		return saveBaseline(ctx, s.db, *selected, entry.hash)
	}
	entry, exists := local[*selected]
	if !exists {
		return fmt.Errorf("versão local ausente")
	}
	if _, found := secretscan.Redact(strings.Join([]string{entry.Name, entry.Description, entry.Content}, "\n")); len(found) > 0 {
		return fmt.Errorf("memória contém possível segredo; envio cancelado")
	}
	_, err = remote.ExecContext(ctx, `INSERT INTO agent_sync_memories_v2(id, agent, session_id, pc, project_path, project_id, type, name, description, content, content_hash) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(project_id, type, name) DO UPDATE SET agent=excluded.agent, session_id=excluded.session_id, pc=excluded.pc, project_path=excluded.project_path, description=excluded.description, content=excluded.content, content_hash=excluded.content_hash`, entry.ID, entry.Agent, entry.SessionID, entry.PC, entry.ProjectPath, entry.ProjectID, entry.Type, entry.Name, entry.Description, entry.Content, entry.hash)
	if err != nil {
		return fmt.Errorf("resolver conflito remoto: %w", err)
	}
	return saveBaseline(ctx, s.db, *selected, entry.hash)
}
