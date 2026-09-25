// Command mr-review-local coleta uma comparação Git para revisão por qualquer CLI.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/matheusdutra/token-tools/internal/secretscan"
)

const defaultMaxBytes = 256 * 1024

var commitSHA = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

type reviewInput struct {
	Provider     string
	Repo         string
	Base         string
	Head         string
	Project      string
	MRIID        string
	Fetch        bool
	MaxBytes     int
	GitLabConfig *GitLabConfig
}

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

func git(ctx context.Context, repo string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repo}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s falhou: %w", args[0], err)
	}
	return strings.TrimSpace(string(output)), nil
}

func resolveCommit(ctx context.Context, repo, ref string) (string, error) {
	if ref == "" || strings.HasPrefix(ref, "-") || strings.ContainsAny(ref, "\r\n\x00") {
		return "", fmt.Errorf("referência Git inválida")
	}
	sha, err := git(ctx, repo, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	if err != nil || !commitSHA.MatchString(sha) {
		return "", fmt.Errorf("não foi possível resolver a referência %q como commit", ref)
	}
	return sha, nil
}

func fetchRemotes(ctx context.Context, repo, base, head string) ([]string, error) {
	listed, err := git(ctx, repo, "remote")
	if err != nil {
		return nil, err
	}
	available := map[string]bool{}
	for _, name := range strings.Split(listed, "\n") {
		available[name] = true
	}
	selected := []string{}
	seen := map[string]bool{}
	for _, ref := range []string{base, head} {
		remote, _, ok := strings.Cut(ref, "/")
		if !ok || !available[remote] {
			continue
		}
		if strings.HasPrefix(remote, "-") || strings.ContainsAny(remote, "\r\n\x00") {
			return nil, fmt.Errorf("nome de remoto inválido")
		}
		if !seen[remote] {
			selected = append(selected, remote)
			seen[remote] = true
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("nenhum remoto identificado nas referências; informe refs remotas explícitas ou omita -fetch")
	}
	for _, remote := range selected {
		if _, err := git(ctx, repo, "fetch", "--no-tags", remote); err != nil {
			return nil, fmt.Errorf("falha ao atualizar remoto %q: %w", remote, err)
		}
	}
	return selected, nil
}

type limitedBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - b.buffer.Len()
	if remaining <= 0 {
		b.truncated = true
		return 0, io.ErrShortWrite
	}
	if len(p) > remaining {
		b.truncated = true
		_, _ = b.buffer.Write(p[:remaining])
		return remaining, io.ErrShortWrite
	}
	return b.buffer.Write(p)
}

func gitDiff(ctx context.Context, repo, base, head string, maxBytes int, stat bool) (string, bool, error) {
	args := []string{"-C", repo, "diff", "--no-ext-diff", "--no-textconv", "--no-color", "--submodule=short"}
	if stat {
		args = append(args, "--stat")
	} else {
		args = append(args, "--unified=3")
	}
	args = append(args, base, head)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	output := &limitedBuffer{limit: maxBytes}
	cmd.Stdout = output
	if err := cmd.Run(); err != nil && !output.truncated {
		return "", false, fmt.Errorf("git diff falhou: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return "", false, fmt.Errorf("git diff interrompido: %w", err)
	}
	return output.buffer.String(), output.truncated, nil
}

func collect(ctx context.Context, input reviewInput) (reviewChange, error) {
	if input.Base == "" || input.Head == "" || input.MaxBytes < 1 {
		return reviewChange{}, fmt.Errorf("informe -base, -head e -max-bytes positivo")
	}
	repo, err := filepath.Abs(input.Repo)
	if err != nil {
		return reviewChange{}, err
	}
	root, err := git(ctx, repo, "rev-parse", "--show-toplevel")
	if err != nil {
		return reviewChange{}, fmt.Errorf("diretório não é um checkout Git: %w", err)
	}
	change := reviewChange{Provider: "local", Repository: root, BaseRef: input.Base, HeadRef: input.Head}
	if input.Fetch {
		change.FetchedRemotes, err = fetchRemotes(ctx, root, input.Base, input.Head)
		if err != nil {
			return reviewChange{}, err
		}
	}
	change.BaseSHA, err = resolveCommit(ctx, root, input.Base)
	if err != nil {
		return reviewChange{}, err
	}
	change.HeadSHA, err = resolveCommit(ctx, root, input.Head)
	if err != nil {
		return reviewChange{}, err
	}
	change.MergeBaseSHA, err = git(ctx, root, "merge-base", change.BaseSHA, change.HeadSHA)
	if err != nil {
		return reviewChange{}, fmt.Errorf("base e head não têm ancestral comum: %w", err)
	}
	var statTruncated bool
	change.Stat, statTruncated, err = gitDiff(ctx, root, change.MergeBaseSHA, change.HeadSHA, input.MaxBytes, true)
	if err != nil {
		return reviewChange{}, err
	}
	change.Diff, change.Truncated, err = gitDiff(ctx, root, change.MergeBaseSHA, change.HeadSHA, input.MaxBytes, false)
	change.Truncated = change.Truncated || statTruncated
	if err != nil {
		return reviewChange{}, err
	}
	if statTruncated {
		change.Stat = ""
	}
	if change.Truncated {
		change.Diff = ""
	}
	var found []string
	change.Stat, found = secretscan.Redact(change.Stat)
	change.Redacted = len(found) > 0
	change.Diff, found = secretscan.Redact(change.Diff)
	change.Redacted = change.Redacted || len(found) > 0
	return change, nil
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("mr-review-local", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	provider := flags.String("provider", "local", "provider: local ou gitlab")
	repo := flags.String("repo", ".", "checkout local")
	base := flags.String("base", "", "referência de base (ex.: upstream/main)")
	head := flags.String("head", "", "referência da mudança (ex.: origin/branch-teste)")
	project := flags.String("project", "", "gitlab: ID ou path do projeto")
	mrIID := flags.String("mr-iid", "", "gitlab: IID do merge request")
	configPath := flags.String("config", "", "gitlab: caminho do JSON de configuração")
	fetch := flags.Bool("fetch", false, "atualiza os remotos das referências")
	maxBytes := flags.Int("max-bytes", defaultMaxBytes, "limite de bytes do patch")
	timeout := flags.Duration("timeout", 30*time.Second, "timeout da coleta")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *timeout <= 0 {
		return errors.New("argumentos ou timeout inválidos")
	}
	input := reviewInput{
		Provider: *provider,
		Repo:     *repo,
		Base:     *base,
		Head:     *head,
		Project:  *project,
		MRIID:    *mrIID,
		Fetch:    *fetch,
		MaxBytes: *maxBytes,
	}
	if *provider == "gitlab" {
		if *configPath == "" {
			return errors.New("provider gitlab requer -config")
		}
		data, err := os.ReadFile(*configPath)
		if err != nil {
			return fmt.Errorf("ler -config: %w", err)
		}
		var cfg GitLabConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("parse -config: %w", err)
		}
		input.GitLabConfig = &cfg
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	prov, err := dispatchProvider(input)
	if err != nil {
		return err
	}
	change, err := prov.Fetch(ctx, input, input.MaxBytes)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(change)
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "mr-review-local:", err)
		os.Exit(1)
	}
}
