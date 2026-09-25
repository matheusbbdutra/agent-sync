// Package jsonschema exposes schemas JSON embarcados em binario via embed.FS
// e um helper Validate(name, payload) que retorna *ValidationError da lib
// github.com/santhosh-tekuri/jsonschema/v6 (Apache-2.0, sem CGO).
//
// Schemas vivem em ./schemas/<name>.json e sao declarados como JSON Schema
// draft 2020-12. Carrega-los via embed garante que o binario agent-sync
// carrega o schema da mesma forma em qualquer ambiente, sem dependência de
// leitura em disco.
//
// Decisao ADR-001: schemas sao "fechados por padrao" (additionalProperties:
// false no nivel raiz e em tipos compostos). A unica excecao registrada e
// 'details' em eventos da ADR-003, justificada pela evolucao de payloads
// por kind.
package jsonschema

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"

	jsonschemav6 "github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schemas/*.json
var schemasFS embed.FS

// Load compila o schema nomeado (ex.: "session-state") e devolve o
// *jsonschemav6.Schema pronto para Validar instancias.
//
// O schema e lido do embed.FS embutido neste pacote, parseado como JSON,
// registrado no Compiler sob a URL `<name>` e compilado. AssertFormat e
// ativado para validar formatos como date-time.
func Load(name string) (*jsonschemav6.Schema, error) {
	raw, err := schemasFS.ReadFile("schemas/" + name + ".json")
	if err != nil {
		return nil, fmt.Errorf("jsonschema: schema %q nao encontrado: %w", name, err)
	}

	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("jsonschema: schema %q invalido: %w", name, err)
	}

	compiler := jsonschemav6.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource(name, doc); err != nil {
		return nil, fmt.Errorf("jsonschema: registrar %q: %w", name, err)
	}

	schema, err := compiler.Compile(name)
	if err != nil {
		return nil, fmt.Errorf("jsonschema: compilar %q: %w", name, err)
	}
	return schema, nil
}

// ValidationError isola o tipo de erro estruturado da lib subjacente, para
// que callers nao importem a lib diretamente.
type ValidationError = jsonschemav6.ValidationError

// Validate carrega o schema pelo nome e valida payload. Retorna nil se a
// instancia for valida; retorna *ValidationError (erro estruturado com
// InstanceLocation, ErrorKind e Causes) caso contrario.
//
// payload pode ser qualquer tipo Go encodavel em JSON (struct, map, slice).
// E marshalado para bytes e re-decodado em arvore generica antes de ser
// passado para a lib subjacente, que opera em map[string]any.
//
// name deve ser o basename de schemas/<name>.json (sem extensao).
func Validate(name string, payload any) error {
	schema, err := Load(name)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("jsonschema: marshal %q: %w", name, err)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return fmt.Errorf("jsonschema: unmarshal %q: %w", name, err)
	}
	if err := schema.Validate(decoded); err != nil {
		var verr *ValidationError
		if errors.As(err, &verr) {
			return verr
		}
		return fmt.Errorf("jsonschema: validar %q: %w", name, err)
	}
	return nil
}
