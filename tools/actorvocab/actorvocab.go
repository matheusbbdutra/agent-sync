// Package actorvocab é a fonte única de verdade do vocabulário de "actor"
// (origem de um evento/telemetria do agent-sync): quais CLIs existem, como o
// nome é validado nos schemas JSON e como é normalizado para o vocabulário do
// memory-mcp.
//
// # Por que existe (A-82)
//
// A mesma lista de CLIs estava duplicada em 6 lugares — 4 schemas JSON
// (`session-event.actor`, `token-budget-status.actor`,
// `precompact-snapshot.actor`, `agent_tasks.cli`), `actorToAgent` em
// internal/event/store.go e `agentEnum` em tools/cmd/memory-mcp — com **2
// vocabulários divergentes**: os schemas usam `claude`/`agy`; o memory-mcp usa
// `claude-code`/`antigravity`. Adicionar a 6ª CLI (Cline) exigiu editar vários
// pontos e 2 ficaram para trás, quebrando telemetria (`agent_tasks`) e nudge
// (`token-budget-status`) **em silêncio**.
//
// # Regra de evolução (revisa a ADR-003, Decisão 2)
//
// Adicionar um CLI é **aditivo**: basta incluir o slug em `clis` (o schema
// aceita o valor novo sem bump). Origens fora do conjunto core usam o escape
// hatch namespaced `cli:<slug>`. MAJOR bump fica reservado para **remoção ou
// renomeação** de um valor já aceito — não para adição.
//
// # Onde é usado
//
//   - `tools/jsonschema/schemas/*.json`: os enums são gerados/guardados por um
//     teste de paridade (jsonschema_actorvocab_test.go);
//   - `tools/cmd/memory-mcp`: `agentEnum` derivado de MemoryAgents();
//   - root module: `internal/event/store.go` (actorToAgent) e as help strings
//     de `-actor` em internal/state/snapshot.go e internal/budget/status.go.
//
// O módulo `tools` é dependência do root (go.mod replace => ./tools), então
// este pacote é importável pelos dois lados. Não pode ficar em `tools/internal`
// justamente porque o root precisa importá-lo.
package actorvocab

import (
	"regexp"
	"strings"
)

// Namespace é o prefixo do escape hatch para origens fora do core.
const Namespace = "cli"

// clis é a lista canônica das CLIs wiraveis pelo agent-sync (ordem estável:
// a mesma ordem de internal/target.GetTargets, com "agy" no lugar de
// "antigravity" porque este é o nome usado nos eventos).
var clis = []string{"claude", "codex", "opencode", "cursor", "agy", "cline"}

// nonCLI são os actors que não representam uma CLI.
var nonCLI = []string{"user", "tool", "agent-sync"}

// mcpNames mapeia o nome canônico (usado nos JSONL/schemas) para o vocabulário
// do memory-mcp, que historicamente divergiu (`claude-code`, `antigravity`).
var mcpNames = map[string]string{
	"claude":   "claude-code",
	"codex":    "codex",
	"opencode": "opencode",
	"cursor":   "cursor",
	"agy":      "antigravity",
	"cline":    "cline",
}

// SlugPattern casa o slug de uma CLI dentro do namespace (`cli:<slug>`), em
// regex **ancorada** (uso direto no JSON Schema, que não ancora por conta).
const SlugPattern = "^[a-z][a-z0-9-]{0,31}$"

// NamespacedPattern casa o escape hatch completo (`cli:<slug>`).
const NamespacedPattern = "^" + Namespace + ":[a-z][a-z0-9-]{0,31}$"

var (
	slugRe       = regexp.MustCompile(SlugPattern)
	namespacedRe = regexp.MustCompile(NamespacedPattern)
)

// aliases mapeia nomes históricos do vocabulário memory-mcp (`claude-code`,
// `antigravity`) para o nome canônico de eventos. Bug achado no smoke de A-23
// (2026-09-30, D-116): o wiramento Antigravity exporta
// AGENT_SYNC_AGENT_KIND=antigravity (target.AgentKind, internal/target/
// target.go:69 — valor também consumido por `ctx-window hook`, que usa
// "antigravity"), e token-nudge.check.sh o repassa como `budget nudge -actor`.
// O schema token-budget-status rejeitava em silêncio (hook exit 0 sem nudge).
// O alias normaliza na fronteira do vocabulário — fonte única — sem exigir
// edição nos N pontos de wiramento.
var aliases = map[string]string{
	"claude-code": "claude",
	"antigravity": "agy",
}

// CanonicalActor mapeia aliases conhecidos para o nome canônico de eventos;
// devolve v inalterado quando não há alias.
func CanonicalActor(v string) string {
	if c, ok := aliases[strings.TrimSpace(v)]; ok {
		return c
	}
	return v
}

// CLIs devolve uma cópia da lista canônica de CLIs.
func CLIs() []string {
	out := make([]string, len(clis))
	copy(out, clis)
	return out
}

// Actors devolve o enum completo aceito no campo `actor` dos eventos:
// user/tool/agent-sync + as CLIs.
func Actors() []string {
	out := make([]string, 0, len(nonCLI)+len(clis))
	out = append(out, nonCLI...)
	out = append(out, clis...)
	return out
}

// MemoryAgents devolve o enum de `agent` do memory-mcp, derivado do
// vocabulário canônico (evita a divergência histórica).
func MemoryAgents() []string {
	out := make([]string, 0, len(nonCLI)+len(clis))
	out = append(out, nonCLI...)
	for _, cli := range clis {
		out = append(out, mcpNames[cli])
	}
	return out
}

// IsCLI informa se o valor é uma das CLIs canônicas.
func IsCLI(v string) bool {
	for _, cli := range clis {
		if v == cli {
			return true
		}
	}
	return false
}

// IsActor informa se o valor é um actor core (user/tool/agent-sync/CLI).
func IsActor(v string) bool {
	for _, a := range Actors() {
		if v == a {
			return true
		}
	}
	return false
}

// IsNamespaced informa se o valor usa o escape hatch `cli:<slug>`.
func IsNamespaced(v string) bool { return namespacedRe.MatchString(v) }

// UsageList devolve os actors core como "a|b|c" para help strings de `-actor`.
func UsageList() string { return strings.Join(Actors(), "|") }

// UsageCLIList devolve as CLIs canônicas como "a|b|c" para help strings.
func UsageCLIList() string { return strings.Join(clis, "|") }

// SlugOf devolve o slug contido em `cli:<slug>` (strings vazias se não for um
// valor namespaced válido).
func SlugOf(v string) string {
	if !IsNamespaced(v) {
		return ""
	}
	return strings.TrimPrefix(v, Namespace+":")
}

// NormalizeActor valida e normaliza o campo `actor`: aceita os actors core
// (bare) ou o escape hatch `cli:<slug>`. Devolve ok=false para qualquer outro
// valor — o schema é "fechado por padrão" fora do namespace (ADR-001).
func NormalizeActor(v string) (string, bool) {
	v = strings.TrimSpace(v)
	v = CanonicalActor(v)
	if v == "" {
		return "agent-sync", true
	}
	if IsActor(v) || IsNamespaced(v) {
		return v, true
	}
	return "", false
}

// NormalizeCLI valida e normaliza o campo `cli` da telemetria de tasks: aceita
// uma CLI canônica ou qualquer slug bem formado (um CLI novo é só um slug —
// aditivo, sem bump de schema).
func NormalizeCLI(v string) (string, bool) {
	v = strings.TrimSpace(v)
	v = CanonicalActor(v)
	if v == "" {
		return "", false
	}
	if IsCLI(v) || slugRe.MatchString(v) {
		return v, true
	}
	return "", false
}

// MemoryAgent normaliza um actor para o vocabulário do memory-mcp:
//   - mapeia `claude`→`claude-code` e `agy`→`antigravity`;
//   - `cli:<slug>` vira o nome do MCP quando conhecido, senão **preserva o
//     slug** (forward-compatible: um CLI novo via escape hatch não perde
//     identidade — o enum anunciado do MCP é advisory, não é enforçado);
//   - vazio ou bare desconhecido vira "agent-sync".
func MemoryAgent(actor string) string {
	actor = strings.TrimSpace(actor)
	// D-119: aceita também o vocabulário de eventos como entrada (agy) e os
	// aliases memory (antigravity/claude-code) — sem isso, MemoryAgent("antigravity")
	// cairia no fallback e perderia identidade silenciosamente ("agent-sync").
	actor = CanonicalActor(actor)
	if slug := SlugOf(actor); slug != "" {
		if name, ok := mcpNames[slug]; ok {
			return name
		}
		return slug
	}
	if name, ok := mcpNames[actor]; ok {
		return name
	}
	for _, a := range nonCLI {
		if a == actor {
			return actor
		}
	}
	return "agent-sync"
}
