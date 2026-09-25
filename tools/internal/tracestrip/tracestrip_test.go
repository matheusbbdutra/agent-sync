package tracestrip

import (
	"strings"
	"testing"
)

func TestStripText(t *testing.T) {
	trace := `Error: connection failed
    at run (/app/src/index.js:10:5)
    at Object.<anonymous> (/app/node_modules/express/lib/router.js:45:12)
    at Module._compile (internal/process/esm_loader.js:12:3)
    at handler (/app/src/auth.js:25:9)`

	result := StripText(trace, 10)
	if !strings.Contains(result, "src/index.js") {
		t.Errorf("deveria manter src/index.js")
	}
	if !strings.Contains(result, "src/auth.js") {
		t.Errorf("deveria manter src/auth.js")
	}
	if strings.Contains(result, "node_modules/express") {
		t.Errorf("não deveria conter node_modules")
	}
	if strings.Contains(result, "internal/process") {
		t.Errorf("não deveria conter internal/process")
	}
	if !strings.Contains(result, "linhas de frameworks/bibliotecas/vendor ocultadas") {
		t.Errorf("deveria conter resumo de linhas ocultadas")
	}
}
