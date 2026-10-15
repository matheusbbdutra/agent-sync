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
// Ordem: override de env > `opencode --version` > fallback 2.
// v1 foi aposentado (hooks/*.opencode.ts deletados): o fallback agora é 2,
// e quem tem v1 real instalado recebe erro explícito no sync (fail-fast)
// em vez de wirar plugin legado silenciosamente.
func MajorVersion() int {
	if v := VersionOverride(); v != 0 {
		return v
	}
	out, err := exec.Command("opencode", "--version").Output()
	if err != nil {
		return 2
	}
	if v := ParseMajor(string(out)); v != 0 {
		return v
	}
	return 2
}
