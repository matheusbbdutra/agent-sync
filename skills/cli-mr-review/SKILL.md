---
name: cli-mr-review
description: "Use when reviewing or collecting MR/PR via glab (GitLab) or gh (GitHub). Coleta e revisão de MR/PR via glab (GitLab) ou gh (GitHub) com detecção automática de host self-hosted."
---

# Skill: revisão de MR/PR via glab/gh

## Quando preferir este caminho ao `mr-reviewer` padrão

- O usuário **já tem** `glab` ou `gh` autenticado e quer evitar config JSON extra.
- O host é self-hosted e foi configurado em `~/.config/glab-cli/config.yml` ou `gh auth login`.
- Compatibilidade com **GitLab 12 self-hosted antigo não é requisito** — `glab` segue a API v4 moderna.

Quando **NÃO** usar: GitLab 12, ambientes sem `glab`/`gh` no PATH, ou quando o usuário pede explicitamente `mr-review-local` (HTTP puro, zero deps).

## Workflow resumido

1. Detectar provider: pergunta direta ao usuário OU infere do remote (`gitlab.com`/`github.com` no host).
2. Resolver `repo` (path do checkout local) e `mr-iid` (IID/número).
3. Executar `mr-collect-cli -provider <gitlab|github> -repo <path> -mr-iid <id> [-host <url>]`.
4. Consumir o JSON `reviewChange` da mesma forma que `mr-review-local` produz.

## Troubleshooting

| Sintoma | Causa provável | Ação |
|---|---|---|
| `glab: command not found` | `glab` não instalado | Instalar via `brew install glab` / `apt install glab` / [releases](https://gitlab.com/gitlab-org/cli/-/releases). Sem fallback automático — usuário precisa instalar. |
| `401 Unauthorized` | Token ausente/expirado | `glab auth login --hostname <url>` ou `gh auth login`. Não pedir para colar token no chat — flag `--token` não existe no fluxo seguro. |
| `mr not found` (GitLab) | IID errado OU projeto errado | Confirmar via `glab mr list --source-branch <branch>` para listar MRs abertas do branch. |
| `Could not resolve host` | `-host` typo OU self-hosted não configurado | Verificar `~/.config/glab-cli/config.yml` tem a entrada `[host "https://gitlab.empresa.com"]`. |
| Patch vem vazio | Diff muito grande OU MR é apenas rename | `mr-collect-cli -max-bytes` (default 256KB). Se ainda vazio, consultar UI web. |
| Erro `subcomando glab X não está na allowlist` | Bug ou tentativa de execução de merge/approve | Read-only POR CONSTRUÇÃO: o binário rejeita. Reportar como finding se vier do agente, nunca do usuário legítimo. |

## Self-hosted: como o host é detectado

`mr-collect-cli` lê `git -C <repo> remote get-url origin` e parseia com duas regex:

- `git@gitlab.empresa.com:group/proj.git` → host `gitlab.empresa.com`
- `https://gitlab.empresa.com/group/proj.git` → host `gitlab.empresa.com`
- `http://gitlab.empresa.com:8080/group/proj.git` → host `gitlab.empresa.com:8080`

Para forçar host: `-host https://gitlab.empresa.com` (override do parse).

## Segurança

- Token vem do credential helper / env var configurado pelo próprio `glab auth login` / `gh auth login`. O binário **não lê** nem **persiste** tokens.
- Allowlist hardcoded de subcomandos: `view`, `diff`. Qualquer outro (`merge`, `approve`, `close`, etc.) falha antes do `exec.Command`.
- Redaction via `internal/secretscan` no patch antes de emitir JSON (mesma proteção que `mr-review-local`).

## Verificação rápida

```bash
# Binário disponível?
which mr-collect-cli || echo "rode: make install"

# glab autenticado?
glab auth status

# gh autenticado?
gh auth status

# Coleta funciona (GitLab)?
mr-collect-cli -provider gitlab -repo /path/to/checkout -mr-iid 42

# Coleta funciona (GitHub)?
mr-collect-cli -provider github -repo /path/to/checkout -mr-iid 42
```
