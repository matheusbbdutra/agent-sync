package opencode

import (
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// VersionOverride permite forçar a linha em testes e em ambientes
// com múltiplas instalações: AGENT_SYNC_OPENCODE_VERSION=1|2.
func VersionOverride() int {
	switch strings.TrimSpace(os.Getenv("AGENT_SYNC_OPENCODE_VERSION")) {
	case "1":
		return 1
	case "2":
		return 2
	}
	return 0
}

var opencodeVersionRe = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

// ParseMajor extrai o major de saídas como "opencode v2.0.11",
// "2.0.11" ou "opencode version 1.18.27". Retorna 0 quando não parseável.
func ParseMajor(out string) int {
	m := opencodeVersionRe.FindStringSubmatch(out)
	if m == nil {
		return 0
	}
	major, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return major
}

// MajorVersion detecta a linha instalada do OpenCode.
// Ordem: override de env > `opencode --version` > fallback 1.
// O fallback 1 preserva o comportamento histórico (plugins v1) quando o
// binário está ausente ou a saída não é parseável — nunca quebra o -apply.
func MajorVersion() int {
	if v := VersionOverride(); v != 0 {
		return v
	}
	out, err := exec.Command("opencode", "--version").Output()
	if err != nil {
		return 1
	}
	if v := ParseMajor(string(out)); v != 0 {
		return v
	}
	return 1
}

// PluginSource escolhe o arquivo-fonte no repo conforme a versão:
// v1 -> hooks/<base>.opencode.ts (formato @opencode-ai/plugin)
// v2 -> hooks/<base>.v2.ts (formato @opencode/plugin, Plugin.define + setup).
// Retorna "" quando o arquivo esperado não existe (caller decide se é
// opcional ou erro — plugins legados são obrigatórios, novos só-v2 são
// silenciosos em v1).
func PluginSource(baseDir, base string, major int) string {
	if major >= 2 {
		return baseDir + "/hooks/" + base + ".v2.ts"
	}
	return baseDir + "/hooks/" + base + ".opencode.ts"
}
