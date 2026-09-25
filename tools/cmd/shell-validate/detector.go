// Static pre-execution shell validator — pattern from arXiv 2609.11957
// ("Look Before You Leap"). Detects shell commands that will almost
// certainly fail because they are missing a required positional argument.
// This is a deliberately small heuristic: a full command validator
// (covering flags, paths, syntax) is out of scope and would produce too
// many false positives for advisory use.
//
// Each rule is "this binary requires at least one non-flag argument of
// kind X". Anything more elaborate (semantic path checks, flag
// compatibility, shell syntax) belongs in a dedicated lint, not here.
package main

import "regexp"

// rule: when the command line starts with this binary and the args list
// has no token that satisfies `argRequired`, the validator flags it.
type rule struct {
	bin         string
	argRequired func([]string) bool
}

// argIsFlag matches tokens starting with `-` (so they are skipped when
// looking for the required positional).
var argIsFlag = regexp.MustCompile(`^-`)

// requiresPositional: rejects the call if there is no non-flag token.
func requiresPositional(args []string) bool {
	for _, a := range args {
		if !argIsFlag.MatchString(a) {
			return true
		}
	}
	return false
}

// rules covers a small set of binaries the agent routinely uses in this
// repo (confirmed against go.mod / Makefile). Each entry is a binary
// that is invalid when invoked without a positional argument.
//
// Intentionally NOT covered: `cd` (variable positional rules), shells
// (would require parsing the rest of the string), mkdir/rm/cp with
// optional paths, anything that accepts stdin-only.
var rules = []rule{
	{bin: "go", argRequired: requiresPositional}, // `go` alone lists help; `go test`, `go build` need args
	{bin: "cargo", argRequired: requiresPositional},
	{bin: "rustc", argRequired: requiresPositional},
	{bin: "gcc", argRequired: requiresPositional},
	{bin: "gofmt", argRequired: requiresPositional},
	{bin: "gofmt-lint", argRequired: requiresPositional},
	{bin: "python", argRequired: requiresPositional}, // not `python3 -c` (handled by requiresNonFlagToken)
	{bin: "python3", argRequired: requiresPositional},
	{bin: "node", argRequired: requiresPositional},
	{bin: "deno", argRequired: requiresPositional},
	{bin: "bun", argRequired: requiresPositional},
	{bin: "ruby", argRequired: requiresPositional},
	{bin: "npm", argRequired: requiresPositional},
	{bin: "yarn", argRequired: requiresPositional},
	{bin: "pnpm", argRequired: requiresPositional},
	{bin: "pip", argRequired: requiresPositional},
	{bin: "pip3", argRequired: requiresPositional},
	{bin: "pytest", argRequired: requiresPositional},
	{bin: "make", argRequired: requiresPositional}, // not just `make` to print default
	{bin: "kubectl", argRequired: requiresPositional},
	{bin: "docker", argRequired: requiresPositional},
	{bin: "podman", argRequired: requiresPositional},
}

// Verdict is the result of validating one shell command.
type Verdict struct {
	Valid  bool
	Reason string
}

// Classify validates a single shell command line. The line is taken
// as-is — we do not parse shell metacharacters, quotes, or pipes.
// Each rule operates on the *head* of the command (the binary), so this
// is only meaningful for top-level commands, not piped fragments.
func Classify(command string) Verdict {
	binary, args := splitHead(command)
	if binary == "" {
		return Verdict{Valid: true}
	}
	for _, r := range rules {
		if r.bin == binary && !r.argRequired(args) {
			return Verdict{
				Valid:  false,
				Reason: binary + " foi invocado sem argumento posicional (provavelmente vai falhar ou não fazer nada útil)",
			}
		}
	}
	return Verdict{Valid: true}
}

// splitHead devolve o nome do binário e os argumentos restantes. Não é um
// shell parser; lida só com o caso comum (espaços, sem aspas compostas)
// que o agent emite. Para comandos com pipes, o argumento `b` da regra
// será o que vem antes do `|` — pode dar classificação incorreta, mas
// o custo de errar é só um aviso adicional falso positivo, não um
// bloqueio.
func splitHead(command string) (string, []string) {
	tokens := tokenize(command)
	if len(tokens) == 0 {
		return "", nil
	}
	return tokens[0], tokens[1:]
}

// tokenize quebra por espaço, sem honrar aspas. Bom o bastante para o
// caso onde o agent emite comandos simples; um parser de shell real é
// desnecessário para validar "faltou argumento".
func tokenize(command string) []string {
	var out []string
	cur := ""
	for _, r := range command {
		if r == ' ' || r == '\t' || r == '\n' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
