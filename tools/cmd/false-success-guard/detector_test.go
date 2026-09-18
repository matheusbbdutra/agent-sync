package main

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		flagged bool
	}{
		{
			name:    "sucesso sem evidencia deve ser sinalizado",
			text:    "Tudo pronto, a funcionalidade foi implementada com sucesso.",
			flagged: true,
		},
		{
			name:    "sucesso com path:line nao deve ser sinalizado",
			text:    "Implementado. Ver main.go:42 e o teste passou (exit code 0).",
			flagged: false,
		},
		{
			name:    "sucesso com bloco de codigo colado nao deve ser sinalizado",
			text:    "Concluído, saída do comando:\n```\nPASS\n```",
			flagged: false,
		},
		{
			name:    "falha honesta nunca deve ser sinalizada",
			text:    "Não consegui concluir: o teste falhou e não verifiquei a causa raiz.",
			flagged: false,
		},
		{
			name:    "texto sem alegacao de sucesso nao deve ser sinalizado",
			text:    "Analisando o arquivo agora para entender a estrutura.",
			flagged: false,
		},
		{
			name:    "sucesso com codigo inline e passou em PT-BR nao deve ser sinalizado (regressao do falso positivo real)",
			text:    "Implementado. Verificado: `gofmt`, `go vet`, `go test` — tudo passou.",
			flagged: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(c.text)
			if got.Flagged != c.flagged {
				t.Errorf("Classify(%q).Flagged = %v, want %v (reason: %s)", c.text, got.Flagged, c.flagged, got.Reason)
			}
		})
	}
}
