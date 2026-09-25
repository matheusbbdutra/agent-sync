package apply

import (
	"fmt"
	"sync"

	"github.com/matheusdutra/agent-sync/internal/agents"
	"github.com/matheusdutra/agent-sync/internal/hooks"
	"github.com/matheusdutra/agent-sync/internal/opencode"
	"github.com/matheusdutra/agent-sync/internal/pathutil"
	"github.com/matheusdutra/agent-sync/internal/target"
)

type TargetCLI = target.TargetCLI

type workerLog struct {
	mu  sync.Mutex
	buf []string
}

func (l *workerLog) append(format string, args ...any) {
	l.mu.Lock()
	l.buf = append(l.buf, fmt.Sprintf(format, args...))
	l.mu.Unlock()
}

func (l *workerLog) Append(format string, args ...any) {
	l.append(format, args...)
}

func (l *workerLog) lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, len(l.buf))
	copy(out, l.buf)
	return out
}

type applyContext struct {
	baseDir      string
	rulesSource  string
	skillsSource string
	log          *workerLog
}

func (c applyContext) applyCommon(t TargetCLI) {
	var rulesErr error
	rulesErr = copyFile(c.rulesSource, t.RulesPath)
	if rulesErr != nil {
		c.log.append("⚠️  [%s/rules] %v", t.Name, rulesErr)
	} else {
		c.log.append("✅ [%s/rules] Regras atualizadas em: %s", t.Name, t.RulesPath)
		switch t.AgentKind {
		case "claude":
			if removed, err := removeLegacyClaudeRules(t.RulesPath); err != nil {
				c.log.append("⚠️  [%s/rules] legado CLAUDE.md: %v", t.Name, err)
			} else if removed {
				c.log.append("🧹 [%s/rules] CLAUDE.md legado removido (agora AGENTS.md)", t.Name)
			}
		case "antigravity":
			if removed, err := removeLegacyGeminiRules(t.RulesPath); err != nil {
				c.log.append("⚠️  [%s/rules] legado GEMINI.md: %v", t.Name, err)
			} else if removed {
				c.log.append("🧹 [%s/rules] GEMINI.md legado removido (agora AGENTS.md)", t.Name)
			}
		case "cursor":
			if removed, err := removeLegacyCursorRules(t.RulesPath); err != nil {
				c.log.append("⚠️  [%s/rules] legado .mdc: %v", t.Name, err)
			} else if removed {
				c.log.append("🧹 [%s/rules] .mdc legado removido (agora AGENTS.md)", t.Name)
			}
		}
	}

	if pathutil.IsProtectedSkillsDir(t.SkillsDir) {
		c.log.append("⛔ [%s/skills] Destino de skills protegido, ignorado: %s", t.Name, t.SkillsDir)
	} else if err := syncSkills(c.skillsSource, t.SkillsDir); err != nil {
		c.log.append("⚠️  [%s/skills] %v", t.Name, err)
	} else {
		c.log.append("✅ [%s/skills] Skills sincronizadas em: %s", t.Name, t.SkillsDir)
	}

	if numAgents, err := agents.SyncAgents(c.baseDir, t, shouldDryRun()); err != nil {
		c.log.append("⚠️  [%s/agents] %v", t.Name, err)
	} else if numAgents > 0 {
		c.log.append("✅ [%s/agents] %d agentes gerados em: %s", t.Name, numAgents, t.AgentsDir)
	}
}

func runCursorHooks(c applyContext, t TargetCLI) {
	hooks.RunCursorHooks(c.baseDir, t, c.log)
}

func runStandardHooks(c applyContext, t TargetCLI) {
	hooks.RunStandardHooks(c.baseDir, t, c.log)
}

func applyToTarget(c applyContext, t TargetCLI) error {
	c.applyCommon(t)
	if t.AgentKind == "opencode" && opencode.MajorVersion() >= 2 {
		for _, issue := range opencode.CheckRuntime(t.OpenCodePluginDir) {
			c.log.append("⚠️  [%s/runtime-ts/%s] %s — %s",
				t.Name, issue.Kind, issue.Detail, issue.Hint)
		}
	}
	if t.HooksFormat == "cursor" {
		runCursorHooks(c, t)
	} else {
		runStandardHooks(c, t)
	}
	if t.AgentKind == "codex" {
		if err := hooks.AdaptCodexPlugins(c.baseDir); err != nil {
			c.log.append("⚠️  [%s/codex-plugins] %v", t.Name, err)
		}
	}
	return nil
}
