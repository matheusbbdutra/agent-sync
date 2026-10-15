package hooks

import (
	"os"

	"github.com/matheusdutra/agent-sync/internal/target"
)

type TargetCLI = target.TargetCLI
type Target = target.Target

var DryRun bool

func shouldDryRun() bool {
	return DryRun || os.Getenv("AGENT_SYNC_DRY_RUN") == "1"
}

func SetDryRun(v bool) {
	DryRun = v
}

type Logger interface {
	Append(format string, args ...any)
}

type HookSpec struct {
	name       string
	fn         func(baseDir string, t TargetCLI) error
	formats    []string                 // filtra por target.HooksFormat; vazio = todos
	agentKinds []string                 // filtra por target.AgentKind; vazio = todos
	detail     func(t TargetCLI) string // string extra na linha de sucesso; "" ou nil = sem linha de sucesso
}

type hookSpec = HookSpec

func (s HookSpec) Name() string { return s.name }

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func (s HookSpec) AppliesTo(t TargetCLI) bool {
	if len(s.formats) > 0 && !containsString(s.formats, t.HooksFormat) {
		return false
	}
	if len(s.agentKinds) > 0 && !containsString(s.agentKinds, t.AgentKind) {
		return false
	}
	return true
}

func (s HookSpec) Run(baseDir string, t TargetCLI, log Logger) {
	if !s.AppliesTo(t) {
		return
	}
	if err := s.fn(baseDir, t); err != nil {
		log.Append("⚠️  [%s/%s] %v", t.Name, s.name, err)
		return
	}
	var d string
	if s.detail != nil {
		d = s.detail(t)
	}
	if d != "" {
		log.Append("✅ [%s/%s] %s", t.Name, s.name, d)
	}
}

// StandardHookNames retorna os nomes dos hooks configurados na tabela padrão.
func StandardHookNames() []string {
	names := make([]string, len(standardHooks))
	for i, h := range standardHooks {
		names[i] = h.name
	}
	return names
}

// RunStandardHooks executa os hooks padrão para todos os targets.
func RunStandardHooks(baseDir string, t TargetCLI, log Logger) {
	for _, h := range standardHooks {
		h.Run(baseDir, t, log)
	}
}
