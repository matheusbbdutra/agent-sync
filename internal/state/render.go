package state

// state_render.go: subcommands CLI para .agent-sync/session-state.json.
//
// Subcommands: read, validate, render, write, migrate-from-md, snapshot,
// next-action. Saida pode ser JSON canonico (-json ou stdout via
// state read) ou STATE.md renderizado (state render). Migrado de
// state_cli.go em 2026-09-21 (Fase 3 do refator por feature - sem
// subpasta porque Go trata cada diretorio como package; prefixo state_
// identifica coesao).
//
// Renomeado de state_cli.go -> state_render.go: prefixo ja tinha 'state',
// 'render' descreve a funcao principal (render STATE.md a partir do JSON).
// state_apply.go: subcommands CLI para .agent-sync/session-state.json.
//
// Subcommands: read, validate, render, write, migrate-from-md, snapshot,
// next-action. Saida pode ser JSON canonico (-json ou stdout via
// state read) ou STATE.md renderizado (state render). Migrado de
// state_cli.go em 2026-09-21 (Fase 3).
//
// Renomeado de state_cli.go -> state_apply.go: prefixo ja tinha 'state',
// 'apply' descreve a funcao (CLI que aplica estado).
import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/matheusdutra/agent-sync/internal/event"
	"github.com/matheusdutra/agent-sync/internal/pathutil"
	"github.com/matheusdutra/token-tools/jsonschema"
)

func RunCommand(args []string) error {
	if len(args) == 0 {
		return stateUsage(os.Stderr)
	}
	switch args[0] {
	case "read":
		return runStateRead(args[1:])
	case "render":
		return runStateRender(args[1:])
	case "validate":
		return runStateValidate(args[1:])
	case "next-action":
		return runStateNextAction(args[1:])
	case "briefing":
		return runStateBriefing(args[1:])
	case "snapshot":
		return runStateSnapshot(args[1:])
	case "write":
		return runStateWrite(args[1:])
	case "migrate-from-md":
		return runStateMigrateFromMD(args[1:])
	case "migrate-taxonomy":
		return runStateMigrateTaxonomy(args[1:])
	case "claim":
		return runStateClaim(args[1:])
	case "release":
		return runStateRelease(args[1:])
	case "help", "-h", "--help":
		return stateUsage(os.Stdout)
	default:
		return fmt.Errorf("state: subcommand desconhecido: %q", args[0])
	}
}

func stateUsage(w io.Writer) error {
	fmt.Fprintf(w, "Uso: agent-sync state <subcommand> [-root <path>]\n\n")
	fmt.Fprintf(w, "Subcommands:\n")
	fmt.Fprintf(w, "  read          Lê .agent-sync/session-state.json e imprime JSON formatado em stdout\n")
	fmt.Fprintf(w, "  render        Imprime STATE.md renderizado a partir do JSON canônico\n")
	fmt.Fprintf(w, "  validate      Valida o JSON contra schema v1 sem escrever\n")
	fmt.Fprintf(w, "  next-action   Imprime a primeira action pendente (id + title + status) ou nada\n")
	fmt.Fprintf(w, "  briefing      Visão agregada: eventos recentes + next-action + tasks pendentes + erros de hooks\n")
	fmt.Fprintf(w, "  snapshot      Imprime precompact-snapshot.json (ADR-precompact-snapshot-cross-cli) com decisao allow|block|advise_only\n")
	fmt.Fprintf(w, "  write         Substitui o JSON canônico a partir de um arquivo (-from <path>) ou stdin\n")
	fmt.Fprintf(w, "  migrate-from-md  Stub: implementado no Estágio B da ADR-002\n")
	fmt.Fprintf(w, "  migrate-taxonomy  Migra D/A/B -> decisions/tasks/issues (ADR D-46)\n")
	fmt.Fprintf(w, "  claim <A-N>      Reivindica tarefa (claim-once handoff, A-52)\n")
	fmt.Fprintf(w, "  release <A-N>    Libera claim da tarefa (-by <kind> para identificar)\n")
	fmt.Fprintf(w, "\nFlags:\n")
	fmt.Fprintf(w, "  -root <path>   Project root (default: resolveBaseDir do main)\n")
	return nil
}

type stateFlags struct {
	fs   *flag.FlagSet
	root string
	from string
}

func newStateFlags(name string) *stateFlags {
	f := &stateFlags{}
	f.fs = flag.NewFlagSet(name, flag.ContinueOnError)
	f.fs.StringVar(&f.root, "root", "", "Project root (default: derivado do binário + AGENT_SYNC_HOME + cwd)")
	f.fs.StringVar(&f.from, "from", "", "Caminho de arquivo JSON (apenas para 'write'); vazio = stdin")
	return f
}

func (f *stateFlags) parse(args []string, w io.Writer) error {
	if err := f.fs.Parse(args); err != nil {
		return err
	}
	return nil
}

func runStateRead(args []string) error {
	f := newStateFlags("agent-sync state read")
	if err := f.parse(args, os.Stderr); err != nil {
		return err
	}
	root, err := pathutil.ResolveStateRoot(f.root)
	if err != nil {
		return err
	}
	// Valida JSON cru do disco antes de desserializar (mesma razao do
	// runStateValidate: pegar violacoes de additionalProperties:false que
	// json.Unmarshal descartaria silenciosamente). Em seguida carrega o
	// state canonicamente para re-marshalizar pretty-printed (D-48.x).
	data, err := os.ReadFile(statePath(root))
	if err != nil {
		return err
	}
	if err := jsonschemaValidateRaw(data); err != nil {
		return err
	}
	s, err := ReadSessionState(root)
	if err != nil {
		return err
	}
	s.normalize()
	out, err := json.MarshalIndent(&s, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Println(string(out))
	return err
}

func runStateRender(args []string) error {
	f := newStateFlags("agent-sync state render")
	if err := f.parse(args, os.Stderr); err != nil {
		return err
	}
	root, err := pathutil.ResolveStateRoot(f.root)
	if err != nil {
		return err
	}
	s, err := ReadSessionState(root)
	if err != nil {
		return err
	}
	_, err = fmt.Print(RenderStateMD(s))
	return err
}

func runStateValidate(args []string) error {
	f := newStateFlags("agent-sync state validate")
	if err := f.parse(args, os.Stderr); err != nil {
		return err
	}
	root, err := pathutil.ResolveStateRoot(f.root)
	if err != nil {
		return err
	}
	// Valida o JSON cru do disco, NAO a struct desserializada. Motivo:
	// json.Unmarshal descarta silenciosamente campos desconhecidos
	// ('extra':true nao chega na struct), mascarando violacoes de
	// additionalProperties:false. Ler raw + validar via schema pega
	// o arquivo como ele realmente e (D-48.x, 2026-09-23).
	data, err := os.ReadFile(statePath(root))
	if err != nil {
		return fmt.Errorf("state: ler session-state.json: %w", err)
	}
	if err := jsonschemaValidateRaw(data); err != nil {
		return fmt.Errorf("state: schema inválido: %w", err)
	}
	fmt.Println("ok")
	return nil
}

func runStateNextAction(args []string) error {
	f := newStateFlags("agent-sync state next-action")
	if err := f.parse(args, os.Stderr); err != nil {
		return err
	}
	root, err := pathutil.ResolveStateRoot(f.root)
	if err != nil {
		return err
	}
	s, err := ReadSessionState(root)
	if err != nil {
		return err
	}
	a, ok := NextAction(s)
	if !ok {
		return nil
	}
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Println(string(data))
	return err
}

func runStateWrite(args []string) error {
	f := newStateFlags("agent-sync state write")
	if err := f.parse(args, os.Stderr); err != nil {
		return err
	}
	root, err := pathutil.ResolveStateRoot(f.root)
	if err != nil {
		return err
	}
	var raw []byte
	if f.from != "" {
		raw, err = os.ReadFile(f.from)
		if err != nil {
			return fmt.Errorf("state write: ler %s: %w", f.from, err)
		}
	} else {
		raw, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("state write: ler stdin: %w", err)
		}
	}
	// Valida o JSON cru antes de desserializar: pega violacoes de
	// additionalProperties:false que json.Unmarshal descartaria (D-48.x).
	if err := jsonschemaValidateRaw(raw); err != nil {
		return fmt.Errorf("state write: rejeitado pelo schema: %w", err)
	}
	var s SessionState
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("state write: JSON inválido: %w", err)
	}
	s.normalize()
	if err := WriteSessionState(root, s); err != nil {
		return err
	}
	fmt.Println("ok")
	return nil
}

// jsonschemaValidate valida v contra o schema session-state embedded.
// Recebe SessionState e re-marshaliza para JSON cru antes de passar para
// a lib santhosh-tekuri/jsonschema. Motivo: a lib, ao receber struct Go
// via reflection, serializa campos nao-setados como 'null' mesmo quando
// o JSON original NAO continha a chave (slices zero-value viram null,
// mas o schema exige array). Re-marshal preserva o estado ja normalizado
// (normalize() no caller) como [] explicito, validando corretamente.
// D-48 fatia 1a (2026-09-23).
func jsonschemaValidate(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("re-marshal para schema: %w", err)
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("re-parse para schema: %w", err)
	}
	if err := jsonschema.Validate("session-state", raw); err != nil {
		return err
	}
	return nil
}

// jsonschemaValidateRaw valida bytes JSON crus contra o schema
// session-state embedded. Usado quando queremos validar o arquivo como
// ele existe no disco (sem passar por json.Unmarshal para struct), o que
// e a unica forma de pegar violacoes de additionalProperties:false (json
// Unmarshal descarta silenciosamente campos desconhecidos). Preferir
// esta funcao em vez de jsonschemaValidate quando o input ja e bytes
// crus. D-48.x (2026-09-23).
func jsonschemaValidateRaw(data []byte) error {
	normalized, err := normalizeRawJSON(data)
	if err != nil {
		return fmt.Errorf("normalize raw JSON: %w", err)
	}
	var raw any
	if err := json.Unmarshal(normalized, &raw); err != nil {
		return fmt.Errorf("parse JSON para schema: %w", err)
	}
	if err := jsonschema.Validate("session-state", raw); err != nil {
		return err
	}
	return nil
}

// normalizeRawJSON substitui null por [] nos campos array declarados no
// schema session-state. Necessario porque arquivos 1.0 (pre-bump) e
// snapshots parciais as vezes tem "tasks":null / "issues":null, e o
// schema 1.1 rejeita null quando o campo e array. Equivalente raw do
// (*SessionState).normalize() (que opera em struct). D-48.x.
func normalizeRawJSON(data []byte) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	sliceFields := []string{"decisions", "tasks", "issues", "open_questions"}
	for _, k := range sliceFields {
		if v, ok := m[k]; ok && v == nil {
			m[k] = []any{}
		}
	}
	// Auto-migration (D-69, A-37, schema cleanup commit): strip-a campos top-level
	// nao declarados no schema session-state.json. Schema aceita apenas 8 campos
	// (additionalProperties:false no raiz). Manter validFields sincronizado com
	// tools/jsonschema/schemas/session-state.json ao adicionar campos novos.
	validFields := map[string]bool{
		"schema_version": true,
		"project":        true,
		"git":            true,
		"session":        true,
		"decisions":      true,
		"tasks":          true,
		"issues":         true,
		"open_questions": true,
	}
	for k := range m {
		if !validFields[k] {
			delete(m, k)
		}
	}
	return json.Marshal(m)
}

// --- Briefing (A-51, S-0.2 Entrega 5) ---
//
// runStateBriefing agrega 4 visões: eventos recentes, next-action,
// tasks pendentes, erros de hooks. Output JSON estruturado (default).
//
// Helpers abaixo (hookErrorEvent, resolveHookLogPath, readHookEvents)
// sao DUPLICADOS de internal/apply/observability.go (package-private,
// zero acoplamento entre pacotes — D-47/A-36 package-by-feature).

type hookErrorEvent struct {
	Code       string `json:"code"`
	Stage      string `json:"stage"`
	CLI        string `json:"cli,omitempty"`
	Tool       string `json:"tool,omitempty"`
	DurationMs *int   `json:"duration_ms,omitempty"`
}

func resolveHookLogPath() (string, error) {
	if p := os.Getenv("AGENT_SYNC_HOOK_LOG"); p != "" {
		return p, nil
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "agent-sync", "hooks", "errors.jsonl"), nil
}

func readHookEvents(path string) ([]hookErrorEvent, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var events []hookErrorEvent
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var ev hookErrorEvent
		if json.Unmarshal(scanner.Bytes(), &ev) == nil {
			events = append(events, ev)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

type briefingTask struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type briefingNextAction struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type briefingOutput struct {
	EventsRecent  []event.SessionEvent  `json:"events_recent"`
	NextAction    *briefingNextAction   `json:"next_action"`
	TasksPending  []briefingTask        `json:"tasks_pending"`
	ErrorsRecent  []hookErrorEvent      `json:"errors_recent"`
}

const briefingLimit = 10

func runStateBriefing(args []string) error {
	root, err := pathutil.ResolveStateRoot("")
	if err != nil {
		return err
	}

	// 1. events_recent: ultimas 10 de event read.
	events, err := event.ReadEvents(root, event.EventReadOptions{Last: briefingLimit})
	if err != nil {
		return fmt.Errorf("event read: %w", err)
	}
	if events == nil {
		events = []event.SessionEvent{}
	}

	// 2. next_action: shim em model.go (NextAction ja filtra pending).
	statePath := filepath.Join(root, pathutil.SessionStateDirName, "session-state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}
	var sess SessionState
	if err := json.Unmarshal(data, &sess); err != nil {
		return fmt.Errorf("parse state: %w", err)
	}
	var na *briefingNextAction
	if action, ok := NextAction(sess); ok {
		na = &briefingNextAction{ID: action.ID, Title: action.Title, Status: action.Status}
	}

	// 3. tasks_pending: primeiras 10 com status=pending.
	var tasks []briefingTask
	for _, t := range sess.Tasks {
		if t.Status != "pending" {
			continue
		}
		tasks = append(tasks, briefingTask{ID: t.ID, Title: t.Title})
		if len(tasks) >= briefingLimit {
			break
		}
	}
	if tasks == nil {
		tasks = []briefingTask{}
	}

	// 4. errors_recent: ultimas 10 do errors.jsonl.
	var errorsRecent []hookErrorEvent
	hookPath, herr := resolveHookLogPath()
	if herr == nil {
		all, _ := readHookEvents(hookPath)
		if len(all) > briefingLimit {
			errorsRecent = all[len(all)-briefingLimit:]
		} else {
			errorsRecent = all
		}
	}
	if errorsRecent == nil {
		errorsRecent = []hookErrorEvent{}
	}

	out := briefingOutput{
		EventsRecent: events,
		NextAction:   na,
		TasksPending: tasks,
		ErrorsRecent: errorsRecent,
	}
	data2, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data2))
	return nil
}
