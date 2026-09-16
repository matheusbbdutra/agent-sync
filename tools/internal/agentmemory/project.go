package agentmemory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Origin struct{ PC, ProjectPath, ProjectID string }

func ResolveOrigin(path string) (Origin, error) {
	pc, err := os.Hostname()
	if err != nil {
		return Origin{}, fmt.Errorf("identificar PC: %w", err)
	}
	explicit := path != ""
	if !explicit {
		path, err = os.Getwd()
		if err != nil {
			return Origin{PC: pc}, nil
		}
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return Origin{}, err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return Origin{}, err
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return Origin{}, fmt.Errorf("caminho do projeto inválido")
	}
	root := gitValue(path, "rev-parse", "--show-toplevel")
	if root != "" {
		path = root
	}
	origin := Origin{PC: pc, ProjectPath: path}
	remote := gitValue(path, "config", "--get", "remote.upstream.url")
	if remote == "" {
		remote = gitValue(path, "config", "--get", "remote.origin.url")
	}
	if normalized := normalizedRemote(remote); normalized != "" {
		sum := sha256.Sum256([]byte(normalized))
		origin.ProjectID = "git:" + hex.EncodeToString(sum[:12])
		return origin, nil
	}
	id, err := configuredProjectID(path)
	if err != nil {
		return Origin{}, err
	}
	if id == "" {
		if !explicit {
			return Origin{PC: pc}, nil
		}
		return Origin{}, fmt.Errorf("projeto sem remoto Git: configure projects[%q] em config.json", path)
	}
	origin.ProjectID = id
	return origin, nil
}

func gitValue(path string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", path}, args...)...)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func normalizedRemote(remote string) string {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return ""
	}
	var host, path string
	if strings.Contains(remote, "://") {
		parsed, err := url.Parse(remote)
		if err != nil || parsed.Hostname() == "" {
			return ""
		}
		host, path = parsed.Hostname(), parsed.Path
	} else if at := strings.LastIndex(remote, "@"); at >= 0 {
		parts := strings.SplitN(remote[at+1:], ":", 2)
		if len(parts) != 2 {
			return ""
		}
		host, path = parts[0], parts[1]
	} else {
		return ""
	}
	host = strings.ToLower(host)
	path = strings.TrimSuffix(strings.Trim(strings.TrimSpace(path), "/"), ".git")
	if host == "" || path == "" {
		return ""
	}
	return host + "/" + path
}

func configuredProjectID(path string) (string, error) {
	configPath, err := EnsureConfig()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return "", err
	}
	id := strings.TrimSpace(config.Projects[path])
	if len(id) > 256 {
		return "", fmt.Errorf("ID do projeto excede 256 caracteres")
	}
	return id, nil
}
