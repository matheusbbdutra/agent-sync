package skills

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// SkillStatus é o resultado do cruzamento entre manifest e diretório.
type SkillStatus string

const (
	StatusInstalled SkillStatus = "installed"
	StatusMissing   SkillStatus = "missing"
	StatusOrphan    SkillStatus = "orphan"
)

// SkillIndexEntry é a linha tabular retornada pelo index.
type SkillIndexEntry struct {
	ID        string      `json:"id"`
	Domain    string      `json:"domain,omitempty"`
	BundleRef []string    `json:"bundle_ref,omitempty"`
	Status    SkillStatus `json:"status"`
	Note      string      `json:"note,omitempty"`
}

// SkillIndexResult é o agregado retornado pelo subcommand.
type SkillIndexResult struct {
	Skills       []SkillIndexEntry `json:"skills"`
	Total        int               `json:"total"`
	Installed    int               `json:"installed"`
	Missing      int               `json:"missing"`
	Orphan       int               `json:"orphan"`
	ManifestPath string            `json:"manifest_path"`
	SkillsDir    string            `json:"skills_dir"`
}

// BundleRefToSlice normaliza bundleRef (string ou []string) em []string.
func BundleRefToSlice(v any) []string {
	switch x := v.(type) {
	case string:
		if x == "" {
			return nil
		}
		return []string{x}
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// RunIndex implementa `agent-sync skills index`.
func RunIndex(args []string) error {
	return RunIndexWithBase(args, "")
}

// RunIndexWithBase é a versão injetável — se baseDir != "", ignora resolveBaseDir.
func RunIndexWithBase(args []string, baseDirInjected string) error {
	fs := flag.NewFlagSet("skills index", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Saída em JSON estruturado em vez de tabela texto")
	showOrphans := fs.Bool("orphans", false, "Mostrar apenas skills órfãs (pasta sem entrada no manifest)")
	showMissing := fs.Bool("missing", false, "Mostrar apenas skills faltantes (manifest sem pasta)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	baseDir := baseDirInjected
	if baseDir == "" {
		exePath, _ := os.Executable()
		cwd, _ := os.Getwd()
		var err error
		baseDir, err = pathutil.ResolveBaseDir(exePath, cwd, os.Getenv("AGENT_SYNC_HOME"))
		if err != nil {
			return err
		}
	}

	result, err := CollectIndex(baseDir)
	if err != nil {
		return err
	}

	switch {
	case *showOrphans:
		filtered := result.Skills[:0]
		for _, s := range result.Skills {
			if s.Status == StatusOrphan {
				filtered = append(filtered, s)
			}
		}
		result.Skills = filtered
		result.Total = len(filtered)
		result.Installed, result.Missing, result.Orphan = 0, 0, 0
		for _, s := range filtered {
			switch s.Status {
			case StatusInstalled:
				result.Installed++
			case StatusMissing:
				result.Missing++
			case StatusOrphan:
				result.Orphan++
			}
		}
	case *showMissing:
		filtered := result.Skills[:0]
		for _, s := range result.Skills {
			if s.Status == StatusMissing {
				filtered = append(filtered, s)
			}
		}
		result.Skills = filtered
		result.Total = len(filtered)
		result.Installed, result.Missing, result.Orphan = 0, 0, 0
		for _, s := range filtered {
			switch s.Status {
			case StatusInstalled:
				result.Installed++
			case StatusMissing:
				result.Missing++
			case StatusOrphan:
				result.Orphan++
			}
		}
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	printSkillsIndexTable(result)
	return nil
}

// CollectIndex cruza o manifest com o diretório de skills e devolve o resultado estruturado.
func CollectIndex(baseDir string) (SkillIndexResult, error) {
	manifestPath := filepath.Join(baseDir, "skills", "manifest.json")
	skillsDir := filepath.Join(baseDir, "skills")

	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return SkillIndexResult{}, fmt.Errorf("falha ao carregar manifest: %w", err)
	}

	manifestByID := make(map[string]SkillEntry, len(manifest.Skills))
	for _, s := range manifest.Skills {
		manifestByID[s.ID] = s
	}

	diskByID := map[string]bool{}
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return SkillIndexResult{}, fmt.Errorf("falha ao listar %s: %w", skillsDir, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		diskByID[e.Name()] = true
	}

	allIDs := make(map[string]struct{}, len(manifestByID)+len(diskByID))
	for id := range manifestByID {
		allIDs[id] = struct{}{}
	}
	for id := range diskByID {
		allIDs[id] = struct{}{}
	}
	ids := make([]string, 0, len(allIDs))
	for id := range allIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	result := SkillIndexResult{
		ManifestPath: manifestPath,
		SkillsDir:    skillsDir,
	}
	for _, id := range ids {
		entry, inManifest := manifestByID[id]
		_, inDisk := diskByID[id]

		var status SkillStatus
		var note string
		switch {
		case inManifest && inDisk:
			status = StatusInstalled
		case inManifest && !inDisk:
			status = StatusMissing
			note = "declared in manifest, folder missing on disk"
		default:
			status = StatusOrphan
			note = "folder exists but not declared in manifest"
		}

		bundleRef := decodeBundleRef(manifestPath, id)

		row := SkillIndexEntry{
			ID:        id,
			BundleRef: bundleRef,
			Status:    status,
			Note:      note,
		}
		if inManifest {
			row.Domain = entry.Domain
		}
		result.Skills = append(result.Skills, row)
		switch status {
		case StatusInstalled:
			result.Installed++
		case StatusMissing:
			result.Missing++
		case StatusOrphan:
			result.Orphan++
		}
	}
	result.Total = len(result.Skills)
	return result, nil
}

func decodeBundleRef(manifestPath, id string) []string {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil
	}
	var raw struct {
		Skills []map[string]any `json:"skills"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	for _, item := range raw.Skills {
		if item["id"] == id {
			return BundleRefToSlice(item["bundleRef"])
		}
	}
	return nil
}

func printSkillsIndexTable(r SkillIndexResult) {
	fmt.Printf("Skills index — %d total (installed=%d, missing=%d, orphan=%d)\n",
		r.Total, r.Installed, r.Missing, r.Orphan)
	fmt.Printf("Manifest: %s\n", r.ManifestPath)
	fmt.Printf("Skills:   %s\n\n", r.SkillsDir)

	const idW, statusW, domainW, bundleW = 36, 10, 18, 24
	fmt.Printf("%-*s  %-*s  %-*s  %-*s  %s\n",
		idW, "ID", statusW, "STATUS", domainW, "DOMAIN", bundleW, "BUNDLE", "NOTE")
	fmt.Println(strings.Repeat("-", idW+statusW+domainW+bundleW+12))
	for _, s := range r.Skills {
		bundle := strings.Join(s.BundleRef, ",")
		if bundle == "" {
			bundle = "-"
		}
		note := s.Note
		if note == "" {
			note = "-"
		}
		fmt.Printf("%-*s  %-*s  %-*s  %-*s  %s\n",
			idW, truncate(s.ID, idW),
			statusW, string(s.Status),
			domainW, truncate(s.Domain, domainW),
			bundleW, truncate(bundle, bundleW),
			note)
	}
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}
