package skills

import (
	"fmt"
)

// RunCommand despacha subcommands do grupo `skills` (index, lint, new).
func RunCommand(args []string) error {
	if len(args) == 0 {
		return RunIndex(nil)
	}
	switch args[0] {
	case "index":
		return RunIndex(args[1:])
	case "lint":
		return RunLint(args[1:])
	case "new":
		return RunNew(args[1:])
	case "-h", "--help", "help":
		fmt.Println("Uso: agent-sync skills <subcommand>")
		fmt.Println("Subcommands:")
		fmt.Println("  index    Lista skills instaladas cruzando manifest.json × diretório skills/")
		fmt.Println("  lint     Valida SKILL.md: frontmatter YAML, name bate com pasta, description com gatilho")
		fmt.Println("  new      Cria esqueleto de nova skill em skills/<id>/SKILL.md")
		fmt.Println("Flags do index:")
		fmt.Println("  -json       Saída em JSON estruturado")
		fmt.Println("  -orphans    Mostra apenas skills órfãs (pasta sem entrada no manifest)")
		fmt.Println("  -missing    Mostra apenas skills faltantes (manifest sem pasta)")
		fmt.Println("Flags do lint:")
		fmt.Println("  -json       Saída em JSON estruturado")
		fmt.Println("Flags do new:")
		fmt.Println("  --description, -d  Descrição da skill (obrigatória)")
		fmt.Println("  --title, -t        Título legível da skill (opcional)")
		fmt.Println("  --force            Sobrescreve se já existir")
		return nil
	}
	return fmt.Errorf("subcommand skills desconhecido: %q (use: index, lint ou new)", args[0])
}
