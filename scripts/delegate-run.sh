#!/usr/bin/env bash
# Envelope fino de delegação entre CLIs (print | session).
# Não decide "qual modelo" — só empacota execução, log, manifesto e tmux.
#
# Uso:
#   delegate-run start --target opencode --mode print  --prompt "..."
#   delegate-run start --target agy      --mode session --prompt "..." [--workspace DIR]
#   delegate-run start --target opencode --mode print  --prompt-file ./p.txt
#   echo "..." | delegate-run start --target opencode --mode print
#   delegate-run status <id>
#   delegate-run result <id> [--raw]
#   delegate-run tail <id> [-f]
#   delegate-run attach <id>
#   delegate-run list
#   delegate-run kill <id>
#
# Dados em ~/.cache/agent-sync/delegates/<id>/{manifest.json,prompt.txt,run.log,exit_code,result.json,wrapper.sh}
set -euo pipefail

DELEGATE_ROOT="${AGENT_SYNC_DELEGATES_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/agent-sync/delegates}"
STALL_SECS="${AGENT_SYNC_DELEGATE_STALL_SECS:-600}"

die() { printf 'delegate-run: %s\n' "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

usage() {
  cat <<'EOF'
Uso:
  delegate-run start --target <opencode|agy|claude|cursor|codex> --mode <print|session> \
      (--prompt TEXT | --prompt-file PATH | stdin) [--workspace DIR] [--name SLUG] [--force-agy-print]
  delegate-run status <id>
  delegate-run result <id> [--raw]
  delegate-run watch <id> [--interval SECS] [--timeout SECS] [--fail-on-stall]
  delegate-run tail <id> [-f]
  delegate-run attach <id>
  delegate-run list
  delegate-run kill <id>

Modos:
  print    non-interactive (bloqueia até terminar; default bom p/ OpenCode)
  session  tmux detached (attach se precisar aprovar; default p/ agy)

Contrato de saída (filho):
  === DELEGATE_RESULT ===
  {"ok":true,"summary":"...","artifacts":[],"notes":""}
  === END_DELEGATE_RESULT ===
  (opcional) store_memory name=delegate-<id>

watch: faz poll de status até done|failed (ou stalled com --fail-on-stall).
  --interval  default 5 (ou AGENT_SYNC_DELEGATE_WATCH_INTERVAL)
  --timeout   default 0 = sem limite (ou AGENT_SYNC_DELEGATE_WATCH_TIMEOUT)

Default sugerido pela skill agent-delegate: target=opencode mode=print.
Stall hint: se log parado por AGENT_SYNC_DELEGATE_STALL_SECS (default 600) e status=running.
EOF
}

ensure_root() { mkdir -p "$DELEGATE_ROOT"; }

iso_now() { date -u +"%Y-%m-%dT%H:%M:%SZ"; }

slugify() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//; s/-{2,}/-/g' | cut -c1-32
}

new_id() {
  local slug="$1" ts
  ts="$(date -u +"%Y%m%dT%H%M%S")"
  if [ -n "$slug" ]; then
    printf '%s-%s' "$ts" "$slug"
  else
    printf '%s-%s' "$ts" "$(head -c 4 /dev/urandom | od -An -tx1 | tr -d ' \n')"
  fi
}

sha256_file() {
  if have sha256sum; then
    sha256sum "$1" | awk '{print $1}'
  elif have shasum; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    printf 'unavailable'
  fi
}

# Prepend contrato DELEGATE_RESULT ao prompt do usuário.
wrap_prompt_with_contract() {
  local id="$1" user_prompt_file="$2" out_file="$3"
  cat >"$out_file" <<EOF
[delegate-run id=${id}]
Ao concluir a tarefa, imprima no stdout (obrigatório) exatamente este bloco — JSON numa linha ou multilinha válido:

=== DELEGATE_RESULT ===
{"ok":true,"summary":"<1-3 frases do que foi feito>","artifacts":[],"notes":""}
=== END_DELEGATE_RESULT ===

Campos: ok (bool), summary (string), artifacts (lista de paths/URLs), notes (string opcional).
Se houver aprendizado que valha persistir, também grave via memory-mcp store_memory com name="delegate-${id}" (scratch:true se for rascunho/teste).
Não omita o bloco DELEGATE_RESULT mesmo se a tarefa falhar (use "ok":false e explique em summary/notes).

---
EOF
  cat "$user_prompt_file" >>"$out_file"
}

# Extrai o último bloco DELEGATE_RESULT do log → result.json; atualiza manifesto.
extract_result() {
  local dir="$1"
  local log="$dir/run.log"
  local out="$dir/result.json"
  [ -f "$log" ] || return 1
  python3 - "$log" "$out" "$dir/manifest.json" <<'PY'
import json, os, re, sys
log_path, out_path, manifest_path = sys.argv[1], sys.argv[2], sys.argv[3]
text = open(log_path, encoding="utf-8", errors="replace").read()
pat = re.compile(
    r"=== DELEGATE_RESULT ===\s*(.*?)\s*=== END_DELEGATE_RESULT ===",
    re.DOTALL,
)
matches = pat.findall(text)
if not matches:
    sys.exit(1)
raw = matches[-1].strip()
try:
    data = json.loads(raw)
except json.JSONDecodeError as e:
    data = {"ok": False, "summary": "DELEGATE_RESULT JSON inválido", "artifacts": [], "notes": str(e), "raw": raw}

with open(out_path, "w", encoding="utf-8") as fh:
    json.dump(data, fh, indent=2, ensure_ascii=False)
    fh.write("\n")

if os.path.exists(manifest_path):
    with open(manifest_path, encoding="utf-8") as fh:
        man = json.load(fh)
else:
    man = {}
man["has_result"] = True
man["result_file"] = out_path
with open(manifest_path, "w", encoding="utf-8") as fh:
    json.dump(man, fh, indent=2, ensure_ascii=False)
    fh.write("\n")
print(out_path)
PY
}

log_age_secs() {
  local log="$1"
  [ -f "$log" ] || { printf '0'; return; }
  python3 - "$log" <<'PY'
import os, sys, time
path = sys.argv[1]
try:
    age = int(time.time() - os.path.getmtime(path))
except OSError:
    age = 0
print(age)
PY
}

write_manifest() {
  # args: dir key=value...
  local dir="$1"
  shift
  python3 - "$dir/manifest.json" "$@" <<'PY'
import json, os, sys
path = sys.argv[1]
data = {}
if os.path.exists(path):
    with open(path) as fh:
        raw = fh.read().strip()
        if raw:
            data = json.loads(raw)
for item in sys.argv[2:]:
    if "=" not in item:
        continue
    k, v = item.split("=", 1)
    if v == "null":
        data[k] = None
    elif v.isdigit() or (v.startswith("-") and v[1:].isdigit()):
        data[k] = int(v)
    elif v in ("true", "false"):
        data[k] = v == "true"
    else:
        data[k] = v
os.makedirs(os.path.dirname(path), exist_ok=True)
with open(path, "w") as fh:
    json.dump(data, fh, indent=2, ensure_ascii=False)
    fh.write("\n")
PY
}

read_manifest_field() {
  local dir="$1" field="$2"
  python3 - "$dir/manifest.json" "$field" <<'PY'
import json, sys
path, field = sys.argv[1], sys.argv[2]
with open(path) as fh:
    data = json.load(fh)
v = data.get(field, "")
print("" if v is None else v)
PY
}

resolve_bin() {
  local target="$1"
  case "$target" in
    opencode) command -v opencode ;;
    agy) command -v agy ;;
    claude) command -v claude ;;
    cursor)
      if have agent; then command -v agent
      elif have cursor-agent; then command -v cursor-agent
      else return 1
      fi
      ;;
    codex) command -v codex ;;
    *) return 1 ;;
  esac
}

# Monta a linha de comando do CLI (prompt já está em $prompt_file).
# Imprime um script bash que deve ser executado no workspace.
# force_agy_print é validado em cmd_start (não aqui: build_runner roda em subshell).
build_runner() {
  local target="$1" mode="$2" prompt_file="$3" _force_agy_print="$4" workspace="$5"
  local bin
  bin="$(resolve_bin "$target")" || die "CLI não encontrado no PATH para target=$target"

  case "$target-$mode" in
    opencode-print)
      # --auto: sem TTY, se o OpenCode pedir aprovação sem essa flag ele
      # trava esperando stdin que nunca chega (o processo fica vivo, mas
      # sem CPU e sem consumir token — parece "rodando" mas está morto).
      # A deny-list continua valendo; --auto só aprova o que não é negado,
      # coerente com o perfil "permissivo" descrito na skill agent-delegate.
      printf '%q run --auto "$(cat %q)"\n' "$bin" "$prompt_file"
      ;;
    opencode-session)
      cat <<EOF
printf '\\n=== delegate-run: prompt em %q ===\\n' $(printf '%q' "$prompt_file")
cat $(printf '%q' "$prompt_file")
printf '\\n=== iniciando opencode TUI (cole/adapte o prompt se precisar) ===\\n'
exec $(printf '%q' "$bin")
EOF
      ;;
    agy-print)
      printf '%q -p "$(cat %q)"\n' "$bin" "$prompt_file"
      ;;
    agy-session)
      printf '%q --prompt-interactive "$(cat %q)"\n' "$bin" "$prompt_file"
      ;;
    claude-print)
      printf '%q -p "$(cat %q)"\n' "$bin" "$prompt_file"
      ;;
    claude-session)
      printf '%q "$(cat %q)"\n' "$bin" "$prompt_file"
      ;;
    cursor-print)
      printf '%q -p --workspace %q "$(cat %q)"\n' "$bin" "$workspace" "$prompt_file"
      ;;
    cursor-session)
      printf '%q --workspace %q "$(cat %q)"\n' "$bin" "$workspace" "$prompt_file"
      ;;
    codex-print)
      # < /dev/null: mesmo recebendo o prompt como argumento, `codex exec`
      # ainda tenta ler um bloco <stdin> extra se detectar stdin "piped"
      # (confirmado no --help: "If stdin is piped ... stdin is appended as
      # a <stdin> block"). Sem TTY, sem fechar o stdin herdado, ele trava
      # pra sempre esperando EOF que nunca chega (mesma classe de bug do
      # opencode sem --auto, mas causa diferente: aqui é leitura de stdin,
      # não aprovação de permissão).
      if "$bin" exec --help >/dev/null 2>&1; then
        printf '%q exec "$(cat %q)" < /dev/null\n' "$bin" "$prompt_file"
      else
        printf '%q "$(cat %q)" < /dev/null\n' "$bin" "$prompt_file"
      fi
      ;;
    codex-session)
      printf '%q "$(cat %q)"\n' "$bin" "$prompt_file"
      ;;
    *)
      die "combinação target/mode inválida: $target / $mode"
      ;;
  esac
}

cmd_start() {
  local target="" mode="" prompt="" prompt_file="" workspace="" name="" force_agy_print=0
  while [ $# -gt 0 ]; do
    case "$1" in
      --target) target="${2:-}"; shift 2 ;;
      --mode) mode="${2:-}"; shift 2 ;;
      --prompt) prompt="${2:-}"; shift 2 ;;
      --prompt-file) prompt_file="${2:-}"; shift 2 ;;
      --workspace) workspace="${2:-}"; shift 2 ;;
      --name) name="${2:-}"; shift 2 ;;
      --force-agy-print) force_agy_print=1; shift ;;
      -h|--help) usage; return 0 ;;
      *) die "flag desconhecida: $1" ;;
    esac
  done

  [ -n "$target" ] || die "falta --target"
  [ -n "$mode" ] || die "falta --mode (print|session)"
  case "$mode" in print|session) ;; *) die "mode inválido: $mode" ;; esac
  case "$target" in opencode|agy|claude|cursor|codex) ;; *) die "target inválido: $target" ;; esac

  resolve_bin "$target" >/dev/null || die "binário do target=$target não está no PATH"

  if [ "$target" = "agy" ] && [ "$mode" = "print" ] && [ "$force_agy_print" != "1" ]; then
    die "agy em mode=print costuma falhar em prompts de permissão. Use --mode session (ou --force-agy-print se a allowlist já cobrir a tarefa)."
  fi

  if [ -z "$workspace" ]; then
    workspace="$(pwd)"
  fi
  workspace="$(cd "$workspace" && pwd)" || die "workspace inválido"

  ensure_root
  local id dir
  id="$(new_id "$(slugify "$name")")"
  dir="$DELEGATE_ROOT/$id"
  mkdir -p "$dir"

  if [ -n "$prompt_file" ]; then
    [ -f "$prompt_file" ] || die "prompt-file não encontrado: $prompt_file"
    cp "$prompt_file" "$dir/prompt.user.txt"
  elif [ -n "$prompt" ]; then
    printf '%s' "$prompt" >"$dir/prompt.user.txt"
  elif [ ! -t 0 ]; then
    cat >"$dir/prompt.user.txt"
  else
    die "forneça --prompt, --prompt-file ou stdin"
  fi
  [ -s "$dir/prompt.user.txt" ] || die "prompt vazio"
  wrap_prompt_with_contract "$id" "$dir/prompt.user.txt" "$dir/prompt.txt"

  local runner_body
  runner_body="$(build_runner "$target" "$mode" "$dir/prompt.txt" "$force_agy_print" "$workspace")"

  # Wrapper: roda o CLI, registra exit, atualiza manifesto.
  cat >"$dir/wrapper.sh" <<EOF
#!/usr/bin/env bash
set -uo pipefail
cd $(printf '%q' "$workspace")
LOG=$(printf '%q' "$dir/run.log")
EXITF=$(printf '%q' "$dir/exit_code")
MAN=$(printf '%q' "$dir/manifest.json")
{
  echo "=== delegate-run start \$(date -u +%Y-%m-%dT%H:%M:%SZ) target=$target mode=$mode ==="
  set +e
$runner_body
  ec=\$?
  set -e
  echo "=== delegate-run end \$(date -u +%Y-%m-%dT%H:%M:%SZ) exit=\$ec ==="
  printf '%s' "\$ec" > "\$EXITF"
  if [ "\$ec" -eq 0 ]; then st=done; else st=failed; fi
  python3 - "\$MAN" "\$st" "\$ec" <<'PY'
import json, sys, datetime
path, st, ec = sys.argv[1], sys.argv[2], int(sys.argv[3])
with open(path) as fh:
    data = json.load(fh)
data["status"] = st
data["exit_code"] = ec
data["finished_at"] = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
with open(path, "w") as fh:
    json.dump(data, fh, indent=2, ensure_ascii=False)
    fh.write("\\n")
PY
  exit "\$ec"
} 2>&1 | tee -a "\$LOG"
exit \${PIPESTATUS[0]}
EOF
  chmod +x "$dir/wrapper.sh"

  local tmux_session=""
  write_manifest "$dir" \
    "id=$id" \
    "target=$target" \
    "mode=$mode" \
    "workspace=$workspace" \
    "started_at=$(iso_now)" \
    "finished_at=null" \
    "status=running" \
    "exit_code=null" \
    "tmux_session=null" \
    "has_result=false" \
    "result_file=$dir/result.json" \
    "log=$dir/run.log" \
    "prompt_file=$dir/prompt.txt" \
    "prompt_sha256=$(sha256_file "$dir/prompt.txt")" \
    "wrapper=$dir/wrapper.sh"

  if [ "$mode" = "print" ]; then
    set +e
    bash "$dir/wrapper.sh"
    local ec=$?
    set -e
    extract_result "$dir" >/dev/null 2>&1 || true
    printf 'id=%s status=%s exit=%s has_result=%s log=%s\n' \
      "$id" "$(read_manifest_field "$dir" status)" "$ec" \
      "$(read_manifest_field "$dir" has_result)" "$dir/run.log"
    return "$ec"
  fi

  have tmux || die "mode=session exige tmux no PATH"
  tmux_session="delegate-$id"
  if tmux has-session -t "$tmux_session" 2>/dev/null; then
    die "já existe sessão tmux: $tmux_session"
  fi
  write_manifest "$dir" "tmux_session=$tmux_session" "status=running"
  tmux new-session -d -s "$tmux_session" -c "$workspace" "bash $(printf '%q' "$dir/wrapper.sh"); exec bash"
  printf 'id=%s status=running mode=session tmux=%s\n' "$id" "$tmux_session"
  printf 'attach: delegate-run attach %s\n' "$id"
  printf '   or: tmux attach -t %s\n' "$tmux_session"
  printf 'tail:   delegate-run tail %s -f\n' "$id"
  printf 'log:    %s\n' "$dir/run.log"
}

refresh_status() {
  local id="$1" dir="$DELEGATE_ROOT/$id"
  [ -d "$dir" ] || die "delegação não encontrada: $id"
  local mode tmux_session status
  mode="$(read_manifest_field "$dir" mode)"
  tmux_session="$(read_manifest_field "$dir" tmux_session)"
  if [ -f "$dir/exit_code" ]; then
    local ec
    ec="$(cat "$dir/exit_code")"
    if [ "$ec" = "0" ]; then status=done; else status=failed; fi
    write_manifest "$dir" "status=$status" "exit_code=$ec"
    if [ -z "$(read_manifest_field "$dir" finished_at)" ]; then
      write_manifest "$dir" "finished_at=$(iso_now)"
    fi
    extract_result "$dir" >/dev/null 2>&1 || true
    return 0
  fi
  if [ "$mode" = "session" ] && [ -n "$tmux_session" ] && [ "$tmux_session" != "null" ]; then
    if tmux has-session -t "$tmux_session" 2>/dev/null; then
      local age
      age="$(log_age_secs "$dir/run.log")"
      if [ "$age" -ge "$STALL_SECS" ]; then
        write_manifest "$dir" "status=stalled" "stalled_hint=true" "log_idle_secs=$age"
      else
        write_manifest "$dir" "status=running" "stalled_hint=false" "log_idle_secs=$age"
      fi
    else
      write_manifest "$dir" "status=failed" "exit_code=130" "finished_at=$(iso_now)"
      printf '130' >"$dir/exit_code"
      extract_result "$dir" >/dev/null 2>&1 || true
    fi
  fi
}

cmd_status() {
  local id="${1:-}"
  [ -n "$id" ] || die "uso: delegate-run status <id>"
  refresh_status "$id"
  python3 - "$DELEGATE_ROOT/$id/manifest.json" <<'PY'
import json, sys
with open(sys.argv[1]) as fh:
    data = json.load(fh)
keys = [
    "id", "target", "mode", "status", "exit_code", "has_result",
    "stalled_hint", "log_idle_secs", "tmux_session", "workspace",
    "started_at", "finished_at", "log", "result_file",
]
for k in keys:
    if k in data and data[k] is not None:
        print(f"{k}={data.get(k)}")
PY
}

cmd_result() {
  local id="${1:-}" raw=0
  shift || true
  [ -n "$id" ] || die "uso: delegate-run result <id> [--raw]"
  if [ "${1:-}" = "--raw" ]; then raw=1; fi
  local dir="$DELEGATE_ROOT/$id"
  [ -d "$dir" ] || die "delegação não encontrada: $id"
  refresh_status "$id"
  if ! extract_result "$dir" >/dev/null 2>&1; then
    printf 'delegate-run: nenhum DELEGATE_RESULT no log de %s\n' "$id" >&2
    printf 'Dica: consulte memory-mcp get_memory/search_memory name=delegate-%s\n' "$id" >&2
    return 1
  fi
  if [ "$raw" -eq 1 ]; then
    cat "$dir/result.json"
  else
    python3 - "$dir/result.json" <<'PY'
import json, sys
data = json.load(open(sys.argv[1], encoding="utf-8"))
print(f"ok={data.get('ok')}")
print(f"summary={data.get('summary', '')}")
arts = data.get("artifacts") or []
print(f"artifacts={json.dumps(arts, ensure_ascii=False)}")
if data.get("notes"):
    print(f"notes={data.get('notes')}")
if "raw" in data:
    print("parse_error=true")
PY
  fi
}

cmd_watch() {
  local id="${1:-}"
  shift || true
  [ -n "$id" ] || die "uso: delegate-run watch <id> [--interval SECS] [--timeout SECS] [--fail-on-stall]"
  local interval="${AGENT_SYNC_DELEGATE_WATCH_INTERVAL:-5}"
  local timeout="${AGENT_SYNC_DELEGATE_WATCH_TIMEOUT:-0}"
  local fail_on_stall=0
  while [ $# -gt 0 ]; do
    case "$1" in
      --interval) interval="${2:-}"; shift 2 ;;
      --timeout) timeout="${2:-}"; shift 2 ;;
      --fail-on-stall) fail_on_stall=1; shift ;;
      *) die "flag desconhecida em watch: $1" ;;
    esac
  done
  [[ "$interval" =~ ^[0-9]+$ ]] && [ "$interval" -ge 1 ] || die "--interval deve ser inteiro >= 1"
  [[ "$timeout" =~ ^[0-9]+$ ]] || die "--timeout deve ser inteiro >= 0"

  local dir="$DELEGATE_ROOT/$id"
  [ -d "$dir" ] || die "delegação não encontrada: $id"

  local started elapsed=0 status last=""
  started="$(date +%s)"
  while true; do
    refresh_status "$id"
    status="$(read_manifest_field "$dir" status)"
    if [ "$status" != "$last" ]; then
      printf 'watch id=%s status=%s log_idle_secs=%s has_result=%s\n' \
        "$id" "$status" \
        "$(read_manifest_field "$dir" log_idle_secs)" \
        "$(read_manifest_field "$dir" has_result)"
      last="$status"
    fi

    case "$status" in
      done)
        extract_result "$dir" >/dev/null 2>&1 || true
        return 0
        ;;
      failed)
        extract_result "$dir" >/dev/null 2>&1 || true
        return 1
        ;;
      stalled)
        if [ "$fail_on_stall" -eq 1 ]; then
          printf 'delegate-run: stalled (log idle >= %ss). attach: delegate-run attach %s\n' "$STALL_SECS" "$id" >&2
          return 2
        fi
        # sem --fail-on-stall: continua polling (pode voltar a running se o log mexer)
        ;;
    esac

    elapsed=$(( $(date +%s) - started ))
    if [ "$timeout" -gt 0 ] && [ "$elapsed" -ge "$timeout" ]; then
      printf 'delegate-run: watch timeout após %ss (status=%s)\n' "$elapsed" "$status" >&2
      return 3
    fi
    sleep "$interval"
  done
}

cmd_tail() {
  local id="${1:-}" follow=0
  shift || true
  [ -n "$id" ] || die "uso: delegate-run tail <id> [-f]"
  if [ "${1:-}" = "-f" ] || [ "${1:-}" = "--follow" ]; then follow=1; fi
  local log="$DELEGATE_ROOT/$id/run.log"
  [ -f "$log" ] || { mkdir -p "$DELEGATE_ROOT/$id"; : >"$log"; }
  if [ "$follow" -eq 1 ]; then
    tail -n 50 -f "$log"
  else
    tail -n 80 "$log"
  fi
}

cmd_attach() {
  local id="${1:-}"
  [ -n "$id" ] || die "uso: delegate-run attach <id>"
  refresh_status "$id"
  local sess
  sess="$(read_manifest_field "$DELEGATE_ROOT/$id" tmux_session)"
  [ -n "$sess" ] && [ "$sess" != "null" ] || die "delegação $id não tem sessão tmux (mode=print?)"
  tmux has-session -t "$sess" 2>/dev/null || die "sessão tmux sumiu: $sess (veja: delegate-run status $id)"
  exec tmux attach -t "$sess"
}

cmd_list() {
  ensure_root
  local d id
  shopt -s nullglob
  for d in "$DELEGATE_ROOT"/*/; do
    id="$(basename "$d")"
    [ -f "$d/manifest.json" ] || continue
    refresh_status "$id" 2>/dev/null || true
    printf '%s\t%s\t%s\t%s\n' \
      "$id" \
      "$(read_manifest_field "$d" target)" \
      "$(read_manifest_field "$d" mode)" \
      "$(read_manifest_field "$d" status)"
  done | sort
}

cmd_kill() {
  local id="${1:-}"
  [ -n "$id" ] || die "uso: delegate-run kill <id>"
  local dir="$DELEGATE_ROOT/$id"
  [ -d "$dir" ] || die "delegação não encontrada: $id"
  local sess
  sess="$(read_manifest_field "$dir" tmux_session)"
  if [ -n "$sess" ] && [ "$sess" != "null" ] && tmux has-session -t "$sess" 2>/dev/null; then
    tmux kill-session -t "$sess"
  fi
  if [ ! -f "$dir/exit_code" ]; then
    printf '137' >"$dir/exit_code"
  fi
  write_manifest "$dir" "status=failed" "exit_code=137" "finished_at=$(iso_now)"
  printf 'killed %s\n' "$id"
}

main() {
  local cmd="${1:-}"
  [ -n "$cmd" ] || { usage; exit 1; }
  shift || true
  case "$cmd" in
    start) cmd_start "$@" ;;
    status) cmd_status "$@" ;;
    result) cmd_result "$@" ;;
    watch) cmd_watch "$@" ;;
    tail) cmd_tail "$@" ;;
    attach) cmd_attach "$@" ;;
    list) cmd_list "$@" ;;
    kill) cmd_kill "$@" ;;
    -h|--help|help) usage ;;
    *) die "comando desconhecido: $cmd (help para uso)" ;;
  esac
}

main "$@"
