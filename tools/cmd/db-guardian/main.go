package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Config struct {
	Default  string             `json:"default"`
	Profiles map[string]Profile `json:"profiles"`
}

type Profile struct {
	Driver   string `json:"driver"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	Readonly bool   `json:"readonly"`
}

var (
	destructiveRegex = regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|DROP|ALTER|TRUNCATE|CREATE|REPLACE|GRANT|REVOKE)\b`)
	limitRegex       = regexp.MustCompile(`(?i)\bLIMIT\s+\d+`)
)

// sanitizeQuery aplica os guardrails de leitura: bloqueia mutações, avisa sobre
// SELECT * e injeta um LIMIT defensivo quando ausente. Não executa a query.
func sanitizeQuery(query string, maxLimit int) (string, []string, error) {
	if match := destructiveRegex.FindString(query); match != "" {
		return "", nil, fmt.Errorf("comando '%s' detectado; acesso padrão é estritamente Read-Only", strings.ToUpper(match))
	}

	var notices []string
	if strings.Contains(strings.ToUpper(query), "SELECT *") {
		notices = append(notices, "AVISO: 'SELECT *' detectado. Projete colunas explícitas para economizar tokens.")
	}

	sanitized := query
	if !limitRegex.MatchString(sanitized) {
		sanitized = strings.TrimRight(sanitized, ";")
		sanitized = fmt.Sprintf("%s LIMIT %d;", sanitized, maxLimit)
		notices = append(notices, fmt.Sprintf("LIMIT INJETADO: limite seguro de %d linhas adicionado automaticamente.", maxLimit))
	}
	return sanitized, notices, nil
}

func loadConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	cfgPath := filepath.Join(home, ".config", "db-guardian", "profiles.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("arquivo de perfis não encontrado em %s: %w", cfgPath, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("erro ao ler %s: %w", cfgPath, err)
	}
	// Permite credenciais por indireção de ambiente (ex.: "${DB_PASSWORD}").
	for name, p := range cfg.Profiles {
		p.Password = os.ExpandEnv(p.Password)
		cfg.Profiles[name] = p
	}
	return &cfg, nil
}

func main() {
	queryFlag := flag.String("query", "", "SQL query a ser analisada ou sanitizada")
	profileFlag := flag.String("profile", "", "Nome do perfil de banco (ex.: dev_main, dev_secondary)")
	listProfiles := flag.Bool("list", false, "Lista os perfis de banco disponíveis")
	maxLimit := flag.Int("limit", 20, "Limite defensivo de linhas")
	flag.Parse()

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Aviso: %v\n", err)
	}

	if *listProfiles {
		if cfg == nil || len(cfg.Profiles) == 0 {
			fmt.Println("Nenhum perfil configurado em ~/.config/db-guardian/profiles.json")
			return
		}
		fmt.Println("📋 Perfis de Banco Disponíveis:")
		for name, p := range cfg.Profiles {
			defMarker := ""
			if name == cfg.Default {
				defMarker = " (default)"
			}
			fmt.Printf(" - %s%s: [%s] %s:%d/%s (user: %s, readonly: %t)\n",
				name, defMarker, p.Driver, p.Host, p.Port, p.Database, p.User, p.Readonly)
		}
		return
	}

	query := strings.TrimSpace(*queryFlag)
	if query == "" && len(flag.Args()) > 0 {
		query = strings.TrimSpace(strings.Join(flag.Args(), " "))
	}

	if query == "" {
		fmt.Println("Uso: db-guardian [-profile <nome>] -query \"SELECT id, name FROM users\" [-limit 20]")
		fmt.Println("     db-guardian -list")
		os.Exit(1)
	}

	// Identifica o perfil selecionado
	selectedProfile := ""
	if *profileFlag != "" {
		selectedProfile = *profileFlag
	} else if cfg != nil {
		selectedProfile = cfg.Default
	}

	if selectedProfile != "" && cfg != nil {
		p, exists := cfg.Profiles[selectedProfile]
		if !exists {
			fmt.Fprintf(os.Stderr, "❌ Perfil '%s' não encontrado em ~/.config/db-guardian/profiles.json\n", selectedProfile)
			os.Exit(1)
		}
		fmt.Printf("📋 [PERFIL SELECIONADO]: '%s' (%s - %s:%d/%s)\n", selectedProfile, p.Driver, p.Host, p.Port, p.Database)
	}

	sanitizedQuery, notices, err := sanitizeQuery(query, *maxLimit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ [DB-GUARDIAN BLOQUEIO]: %v.\n", err)
		os.Exit(2)
	}
	for _, notice := range notices {
		fmt.Printf("🛡️  [%s]\n", notice)
	}

	fmt.Println("✅ Query validada (NÃO executada — este binário não abre conexão):")
	fmt.Println(sanitizedQuery)
}
