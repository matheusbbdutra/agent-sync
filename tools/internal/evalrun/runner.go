package evalrun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/matheusdutra/token-tools/internal/evalsuite"
)

// Runner produz a saida que um caso vai avaliar. No modo offline (sem
// --runner-cmd) o eval-run pontua o conteudo do --target; com runner, cada caso
// e enviado a um processo externo e a saida dele e o que se avalia.
type Runner interface {
	Run(c evalsuite.Case) (string, error)
}

// ExecRunner roda um processo externo por caso: escreve {"case":...} em stdin e
// le a saida do caso em stdout. Sem SDK de provider e sem HTTP — gerar a saida
// e responsabilidade de quem passa --runner-cmd, que decide qual CLI usar.
type ExecRunner struct {
	Command string
	Timeout time.Duration
}

// Run implementa Runner. Erro aqui e erro de configuracao/infra do runner, nao
// veredito: o caller aborta (exit 2) em vez de marcar o caso como fail.
func (r ExecRunner) Run(c evalsuite.Case) (string, error) {
	payload, err := json.Marshal(map[string]any{"case": c})
	if err != nil {
		return "", fmt.Errorf("empacotar payload do runner: %w", err)
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", r.Command)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("runner-cmd falhou: %w (stderr: %s)", err, truncate(stderr.String(), 400))
	}
	return stdout.String(), nil
}
