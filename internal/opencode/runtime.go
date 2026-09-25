package opencode

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// RuntimeIssue descreve um pre-requisito faltante.
type RuntimeIssue struct {
	Kind   string // "node" | "opencode" | "plugin"
	Detail string // mensagem curta exibida ao usuario
	Hint   string // comando/setup corretivo
}

// CheckRuntime verifica os 3 pre-requisitos de runtime TS.
// Retorna slice vazio quando tudo OK. NAO bloqueia — caller decide
// se emite warning ou aborta (decisao de D-26: wirar e best-effort,
// setup e responsabilidade do usuario).
func CheckRuntime(opencodePluginDir string) []RuntimeIssue {
	var issues []RuntimeIssue

	// 1. Node >= 20.11
	nodeVer := DetectNodeVersion()
	if nodeVer == "" {
		issues = append(issues, RuntimeIssue{
			Kind:   "node",
			Detail: "binario 'node' nao encontrado no PATH",
			Hint:   "instale Node >= 20.11 (mise/npm/asdf) — plugins usam import.meta.dirname",
		})
	} else if !NodeVersionAtLeast(nodeVer, 20, 11) {
		issues = append(issues, RuntimeIssue{
			Kind:   "node",
			Detail: fmt.Sprintf("Node %s e anterior ao minimo 20.11", nodeVer),
			Hint:   "atualize para Node >= 20.11 (mise/npm/asdf)",
		})
	}

	// 2. Binario opencode >= 2.0.0
	ocVer := DetectVersionFromBin()
	if ocVer == "" {
		issues = append(issues, RuntimeIssue{
			Kind:   "opencode",
			Detail: "binario 'opencode' nao encontrado no PATH",
			Hint:   "instale OpenCode >= v2.0.0 (mise/npm) — plugins v2 dependem do runtime",
		})
	} else if !VersionAtLeast(ocVer, 2, 0, 0) {
		issues = append(issues, RuntimeIssue{
			Kind:   "opencode",
			Detail: fmt.Sprintf("OpenCode %s e anterior ao minimo v2.0.0 (plugins novos sao so-v2)", ocVer),
			Hint:   "atualize OpenCode para >= v2.0.0, ou defina AGENT_SYNC_OPENCODE_VERSION=1 para fallback v1",
		})
	}

	// 3. @opencode/plugin resolvivel em algum node_modules do loader
	if path, ok := LocatePluginPackage(opencodePluginDir); !ok {
		issues = append(issues, RuntimeIssue{
			Kind:   "plugin",
			Detail: "pacote '@opencode/plugin' nao encontrado em nenhum node_modules pesquisado",
			Hint:   "rode scripts/setup-opencode.sh para instalar, ou: npm i -g @opencode/plugin",
		})
	} else {
		_ = path // OK — caminho encontrado, ignorado no log para nao vazar estrutura
	}

	return issues
}

func DetectNodeVersion() string {
	out, err := exec.Command("node", "--version").Output()
	if err != nil {
		return ""
	}
	v := strings.TrimSpace(string(out))
	return strings.TrimPrefix(v, "v")
}

var nodeMinorRe = regexp.MustCompile(`^(\d+)\.(\d+)(?:\.(\d+))?`)

func NodeVersionAtLeast(ver string, minMajor, minMinor int) bool {
	ver = strings.TrimPrefix(ver, "v")
	m := nodeMinorRe.FindStringSubmatch(ver)
	if m == nil {
		return false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	if major > minMajor {
		return true
	}
	if major < minMajor {
		return false
	}
	return minor >= minMinor
}

func DetectVersionFromBin() string {
	out, err := exec.Command("opencode", "--version").Output()
	if err != nil {
		return ""
	}
	m := opencodeVersionRe.FindStringSubmatch(string(out))
	if m == nil {
		return ""
	}
	return m[0]
}

func VersionAtLeast(ver string, minMajor, minMinor, minPatch int) bool {
	parts := strings.Split(ver, ".")
	if len(parts) < 3 {
		return false
	}
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	patch, _ := strconv.Atoi(parts[2])
	if major != minMajor {
		return major > minMajor
	}
	if minor != minMinor {
		return minor > minMinor
	}
	return patch >= minPatch
}

func LocatePluginPackage(opencodePluginDir string) (string, bool) {
	configDir := filepath.Dir(opencodePluginDir)
	candidates := []string{
		filepath.Join(configDir, "node_modules", "@opencode", "plugin", "package.json"),
		filepath.Join(os.Getenv("HOME"), ".opencode", "node_modules", "@opencode", "plugin", "package.json"),
	}
	if out, err := exec.Command("npm", "root", "-g").Output(); err == nil {
		globalRoot := strings.TrimSpace(string(out))
		if globalRoot != "" {
			candidates = append(candidates,
				filepath.Join(globalRoot, "@opencode", "plugin", "package.json"))
		}
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}
