package skills

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

var kebabCaseRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

const defaultSkillTemplate = `---
name: {{id}}
description: "{{description}}"
---

# {{title}}

## Use this skill when

- Descreva os cenários em que esta skill deve ser ativada.
- Quando a tarefa envolver {{title}}.

## Do not use this skill when

- Tarefas diretas que não necessitam deste procedimento específico.

## Instructions

1. Descreva o procedimento operacional padrão passo a passo.
2. Mantenha os passos objetivos, determinísticos e verificáveis.
`

func toTitleCase(kebab string) string {
	parts := strings.Split(kebab, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
		}
	}
	return strings.Join(parts, " ")
}

// RunNew implementa `agent-sync skills new`.
func RunNew(args []string) error {
	return RunNewWithBase(args, "")
}

// RunNewWithBase aceita baseDir injetável para testes.
func RunNewWithBase(args []string, baseDirInjected string) error {
	fs := flag.NewFlagSet("skills new", flag.ContinueOnError)
	var description, title, rootFlag string
	var force bool

	fs.StringVar(&description, "description", "", "Descrição da skill (obrigatória, deve conter gatilhos de uso)")
	fs.StringVar(&description, "d", "", "Alias para --description")
	fs.StringVar(&title, "title", "", "Título legível da skill (opcional, padrão: derivado do id)")
	fs.StringVar(&title, "t", "", "Alias para --title")
	fs.StringVar(&rootFlag, "root", "", "Diretório raiz do projeto (opcional)")
	fs.BoolVar(&force, "force", false, "Sobrescreve a skill caso o diretório já exista")

	var positionalID string
	var flagArgs []string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		positionalID = args[0]
		flagArgs = args[1:]
	} else {
		flagArgs = args
	}

	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	id := positionalID
	if id == "" {
		remaining := fs.Args()
		if len(remaining) > 0 {
			id = remaining[0]
		}
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("id da skill é obrigatório: uso `agent-sync skills new <id> --description '...'`")
	}

	if !kebabCaseRegex.MatchString(id) {
		return fmt.Errorf("id inválido %q: deve seguir o padrão kebab-case (letras minúsculas, números e hífens, ex: 'meu-componente')", id)
	}

	description = strings.TrimSpace(description)
	if description == "" {
		return fmt.Errorf("a flag --description (ou -d) é obrigatória e não pode ser vazia")
	}

	baseDir := baseDirInjected
	if baseDir == "" {
		if rootFlag != "" {
			baseDir = rootFlag
		} else {
			exePath, _ := os.Executable()
			cwd, _ := os.Getwd()
			var err error
			baseDir, err = pathutil.ResolveBaseDir(exePath, cwd, os.Getenv("AGENT_SYNC_HOME"))
			if err != nil {
				return err
			}
		}
	}

	skillsDir := filepath.Join(baseDir, "skills")
	targetDir := filepath.Join(skillsDir, id)
	targetFile := filepath.Join(targetDir, "SKILL.md")

	if fi, err := os.Stat(targetDir); err == nil && fi.IsDir() && !force {
		return fmt.Errorf("a skill %q já existe em %s; use --force para sobrescrever", id, targetDir)
	}

	if title == "" {
		title = toTitleCase(id)
	}

	tplContent := defaultSkillTemplate
	tplPath := filepath.Join(baseDir, "templates", "skill.md.tpl")
	if data, err := os.ReadFile(tplPath); err == nil && len(data) > 0 {
		tplContent = string(data)
	}

	rendered := strings.ReplaceAll(tplContent, "{{id}}", id)
	rendered = strings.ReplaceAll(rendered, "{{title}}", title)
	rendered = strings.ReplaceAll(rendered, "{{description}}", description)

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("falha ao criar diretório %s: %w", targetDir, err)
	}

	if err := os.WriteFile(targetFile, []byte(rendered), 0o644); err != nil {
		return fmt.Errorf("falha ao criar arquivo %s: %w", targetFile, err)
	}

	fmt.Printf("✅ Skill criada com sucesso: %s\n", targetFile)
	fmt.Println("Execute 'agent-sync -apply' para sincronizar com as CLIs configuradas.")
	return nil
}
