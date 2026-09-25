package skills

import (
	"encoding/json"
	"fmt"
	"os"
)

// SkillEntry descreve uma entrada de skill no manifest.json.
type SkillEntry struct {
	ID     string `json:"id"`
	Domain string `json:"domain"`
}

// ManifestSource descreve os metadados de origem do manifest de skills.
type ManifestSource struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	License    string `json:"license"`
	SkillsDir  string `json:"skillsDir"`
}

// Manifest representa a estrutura do arquivo skills/manifest.json.
type Manifest struct {
	Source ManifestSource `json:"source"`
	Skills []SkillEntry   `json:"skills"`
}

// LoadManifest carrega e desserializa o arquivo manifest.json.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("erro ao ler %s: %w", path, err)
	}
	if len(m.Skills) == 0 {
		return nil, fmt.Errorf("%s não define skills", path)
	}
	return &m, nil
}
