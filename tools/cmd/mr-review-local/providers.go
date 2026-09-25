package main

import (
	"context"
	"errors"
	"fmt"
)

// ChangeProvider coleta uma mudança (PR/MR) e a normaliza para reviewChange.
type ChangeProvider interface {
	// Name retorna o identificador curto do provider (ex.: "local", "gitlab").
	Name() string
	// Fetch obtém a mudança e a devolve no formato reviewChange.
	// O parâmetro in contém dados específicos do provider (refs para local,
	// project/mr-iid para gitlab) e o maxBytes impõe limite de patch.
	Fetch(ctx context.Context, in reviewInput, maxBytes int) (reviewChange, error)
}

// dispatchProvider seleciona o provider conforme input.Provider.
// Para "local" (default) delega para collect(); para "gitlab" lê config
// carregado em input.GitLabConfig e instancia o adapter.
func dispatchProvider(in reviewInput) (ChangeProvider, error) {
	switch in.Provider {
	case "", "local":
		return localProvider{}, nil
	case "gitlab":
		if in.GitLabConfig == nil {
			return nil, errors.New("provider gitlab requer -config")
		}
		return newGitLabProvider(in.GitLabConfig)
	default:
		return nil, fmt.Errorf("provider desconhecido: %q", in.Provider)
	}
}

// localProvider implementa ChangeProvider para coleta via git local.
// Delega para collect() preservando flags de repo/base/head/fetch.
type localProvider struct{}

func (localProvider) Name() string { return "local" }

func (localProvider) Fetch(ctx context.Context, in reviewInput, _ int) (reviewChange, error) {
	return collect(ctx, in)
}
