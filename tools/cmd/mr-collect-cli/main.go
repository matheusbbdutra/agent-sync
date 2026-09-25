// Command mr-collect-cli coleta uma comparação Git para revisão por qualquer CLI.
//
// Provider alternativo ao mr-review-local: delega para glab (GitLab) ou gh
// (GitHub) em vez de HTTP puro. Vantagem: zero-config para self-hosted (glab
// já sabe o host via ~/.config/glab-cli/config.yml, gh idem).
//
// Read-only POR CONSTRUÇÃO: cada provider mantém uma allowlist hardcoded de
// subcomandos. Qualquer subcomando fora da lista falha antes de chamar exec.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

const defaultMaxBytes = 256 * 1024

type collectInput struct {
	Provider string // "gitlab" | "github"
	Repo     string // path do checkout local (obrigatório para parse de remote)
	MRIID    string // IID da MR (GitLab) ou número da PR (GitHub)
	Host     string // opcional: força host (ex.: https://gitlab.empresa.com)
	MaxBytes int
	Timeout  time.Duration
}

// reviewChange é o mesmo contrato de mr-review-local, para que o agente
// (atual ou novo) consuma sem alteração.
type reviewChange struct {
	Provider       string   `json:"provider"`
	Repository     string   `json:"repository"`
	BaseRef        string   `json:"base_ref"`
	HeadRef        string   `json:"head_ref"`
	BaseSHA        string   `json:"base_sha"`
	HeadSHA        string   `json:"head_sha"`
	MergeBaseSHA   string   `json:"merge_base_sha"`
	FetchedRemotes []string `json:"fetched_remotes"`
	Stat           string   `json:"stat"`
	Diff           string   `json:"diff"`
	Truncated      bool     `json:"truncated"`
	Redacted       bool     `json:"redacted"`
}

func main() {
	provider := flag.String("provider", "", "gitlab ou github")
	repo := flag.String("repo", "", "path do checkout local (para parse de remote)")
	mrIID := flag.String("mr-iid", "", "IID/número da MR/PR")
	host := flag.String("host", "", "host explícito (opcional; override do parse de remote)")
	maxBytes := flag.Int("max-bytes", defaultMaxBytes, "limite do patch em bytes")
	timeout := flag.Duration("timeout", 30*time.Second, "timeout total")
	flag.Parse()

	if *provider == "" {
		fail("requer -provider=gitlab|github")
	}
	if *repo == "" {
		fail("requer -repo (path do checkout para parse de remote)")
	}
	if *mrIID == "" {
		fail("requer -mr-iid (IID da MR ou número da PR)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	var (
		change reviewChange
		err    error
	)
	switch *provider {
	case "gitlab":
		change, err = collectGitLab(ctx, collectInput{
			Provider: "gitlab",
			Repo:     *repo,
			MRIID:    *mrIID,
			Host:     *host,
			MaxBytes: *maxBytes,
		})
	case "github":
		change, err = collectGitHub(ctx, collectInput{
			Provider: "github",
			Repo:     *repo,
			MRIID:    *mrIID,
			Host:     *host,
			MaxBytes: *maxBytes,
		})
	default:
		fail(fmt.Sprintf("provider desconhecido: %q (use gitlab ou github)", *provider))
	}
	if err != nil {
		fail(err.Error())
	}

	if err := json.NewEncoder(os.Stdout).Encode(change); err != nil {
		fail("encode json: " + err.Error())
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "mr-collect-cli: "+msg)
	os.Exit(2)
}

// trimSpace é split reutilizável (evita imports em mais de um arquivo).
func trimSpace(s string) string { return strings.TrimSpace(s) }
