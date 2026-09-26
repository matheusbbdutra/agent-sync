package jsonschema

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/matheusdutra/token-tools/actorvocab"
)

// enumEPatternDoCampo extrai o enum e o pattern declarados no campo `field` do
// schema `name`, assumindo a forma `anyOf: [{enum: [...]}, {pattern: "..."}]`
// usada pelos campos de vocabulário de actor/cli.
func enumEPatternDoCampo(t *testing.T, name, field string) (enum []string, pattern string) {
	t.Helper()
	raw, err := schemasFS.ReadFile("schemas/" + name + ".json")
	if err != nil {
		t.Fatalf("ler schema %s: %v", name, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse schema %s: %v", name, err)
	}
	props, ok := doc["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema %s sem properties", name)
	}
	fieldDoc, ok := props[field].(map[string]any)
	if !ok {
		t.Fatalf("schema %s sem campo %q", name, field)
	}
	branches, ok := fieldDoc["anyOf"].([]any)
	if !ok {
		t.Fatalf("campo %s.%s sem anyOf (esperado enum + pattern)", name, field)
	}
	for _, b := range branches {
		branch, ok := b.(map[string]any)
		if !ok {
			continue
		}
		if e, ok := branch["enum"].([]any); ok {
			for _, v := range e {
				if s, ok := v.(string); ok {
					enum = append(enum, s)
				}
			}
		}
		if p, ok := branch["pattern"].(string); ok {
			pattern = p
		}
	}
	return enum, pattern
}

func iguais(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	as := append([]string(nil), a...)
	bs := append([]string(nil), b...)
	sort.Strings(as)
	sort.Strings(bs)
	return strings.Join(as, ",") == strings.Join(bs, ",")
}

// TestVocabularioDosSchemasBateComRegistry é o guarda contra a regressão que
// motivou o A-82: a lista de CLIs estava duplicada em 4 schemas + 2 pontos em
// Go e o Cline (6ª CLI) ficou de fora de dois deles, quebrando telemetria e
// nudge em silêncio. Estes testes falham se um schema divergir de
// tools/actorvocab (fonte única).
func TestVocabularioDosSchemasBateComRegistry(t *testing.T) {
	cases := []struct {
		schema    string
		field     string
		esperado  []string
		pattern   string
		descricao string
	}{
		{"session-event", "actor", actorvocab.Actors(), actorvocab.NamespacedPattern, "actor core + escape hatch"},
		{"precompact-snapshot", "actor", actorvocab.Actors(), actorvocab.NamespacedPattern, "actor core + escape hatch"},
		{"token-budget-status", "actor", actorvocab.Actors(), actorvocab.NamespacedPattern, "actor core + escape hatch"},
		{"agent_tasks", "cli", actorvocab.CLIs(), actorvocab.SlugPattern, "CLIs canônicas + slug livre"},
	}
	for _, tc := range cases {
		t.Run(tc.schema+"."+tc.field, func(t *testing.T) {
			enum, pattern := enumEPatternDoCampo(t, tc.schema, tc.field)
			if !iguais(enum, tc.esperado) {
				t.Errorf("enum de %s.%s divergiu do registry (%s):\n schema=%v\n registry=%v",
					tc.schema, tc.field, tc.descricao, enum, tc.esperado)
			}
			if pattern != tc.pattern {
				t.Errorf("pattern de %s.%s = %q, esperado %q (actorvocab)",
					tc.schema, tc.field, pattern, tc.pattern)
			}
		})
	}
}

// TestSchemasAceitamRegistryEscapeHatch valida de ponta a ponta (via a lib) que
// a expansão do enum é aditiva e que o escape hatch funciona nos 3 schemas de
// actor.
func TestSchemasAceitamRegistryEscapeHatch(t *testing.T) {
	eventos := func(actor string) map[string]any {
		return map[string]any{
			"schema_version": "1.0",
			"ts":             "2026-09-26T12:00:00Z",
			"kind":           "state_render",
			"ref":            "S-1",
			"title":          "probe",
			"actor":          actor,
		}
	}
	for _, actor := range actorvocab.Actors() {
		if err := Validate("session-event", eventos(actor)); err != nil {
			t.Errorf("session-event deveria aceitar actor=%q (registry): %v", actor, err)
		}
	}
	if err := Validate("session-event", eventos("cli:meu-cli-novo")); err != nil {
		t.Errorf("escape hatch 'cli:<slug>' deveria ser aceito: %v", err)
	}
	// Fechado por padrão: fora do core e fora do namespace continua rejeitado.
	if err := Validate("session-event", eventos("meu-cli-novo")); err == nil {
		t.Error("actor bare desconhecido deveria ser rejeitado (precisa do namespace 'cli:')")
	}
}
