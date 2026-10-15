# ADR — Reuso seletivo do `nickarab/bash-sudo-agents` (overwrite+cache em Go; conceitos como ADR; resto rejeitado)

- **Status**: Aceito
- **Data**: 2026-10-15
- **Decisor**: agente + usuário (sessão de consolidação pós-aposentadoria do Cursor)
- **Fonte**: análise do repo externo `nickarab/bash-sudo-agents` (v4.2, 16 commits, 0 stars) +
  `rules/global-rules.md §4` (política de linguagens, `c5e2e09`) +
  `docs/ADR-linguagem-hooks-por-host.md` (Aceito)
- **Tags**: hooks, governança, cache, tokens, go, cross-cli, policy

## Contexto (verificável)

1. O repo externo `nickarab/bash-sudo-agents` implementa "Antigravity Linux Governance v4.2":
   um motor Python stdlib (`plugin/hooks/governance-gate.py`) wirado em
   `PreToolUse matcher="*"` (`plugin/hooks.json`), mais uma constituição declarativa
   (`security/policies.json` RULE-01..15, `security/meta_policies.json` META-01..04),
   dois SKILLs (`agente-bash` operador, `agente-sudo-policy` legislador) e um
   `install.sh` que faz `cp` + `chmod 555/444`.
2. O motor deles faz 4 estágios em ordem: `deny` (regex destrutiva/privilegiada),
   `force_ask` (sudo/yay/Hyprland/escrita em governança), **`overwrite`**
   (cache TTL hit → `cat <cache>`; miss → `( cmd ) | tee <cache>`; compressor →
   `journalctl --no-pager -n 35`, `dmesg | tail -n 35`, `git log -n 15`) e `allow` default.
   Fail-open global via `try/except` → `allow`.
3. O `agent-sync` hoje **não tem** estágio `overwrite`: seus hooks só permitem, negam,
   redigem ou injetam contexto. A economia de tokens por reescrita do comando na fonte
   é a peça ausente.
4. `rules/global-rules.md §4` (travada em `c5e2e09`) determina: **Go é fonte da verdade**,
   shell é cola fina ≤50 LOC, TS só na borda (Cline/OpenCode v2), **Python = zero**,
   e "reaproveitamento externo deve ser portado para Go, nunca copiado como `.py`".
5. Auditoria do motor deles: apenas ~5 das 15 regras declaradas têm regex implementada
   (`RULE-01/02/06/11/12/15` parcial); `PRIV_PATTERN` não cobre `run0|mount|dd`;
   `DESTRUCTIVE` não cobre `rm -rf /home` ou `rm -rf $VAR`; `chmod 444` é revertido pelo
   dono (sem `chattr +i`/hash/assinatura); cache sem `chmod 700`, sem lock, sem filtro de

## Decisão

### §1. Portar `overwrite` + cache TTL — em **Go**, não em Python

O único reaproveitamento de **código** é o padrão de otimização de comando. Ele entra como
**lógica Go** (fonte da verdade), exposta a todos os hosts pelo mesmo caminho dos outros
hooks: adapter shell/TS fino → binário Go.

- **Onde**: dentro de `tools/cmd/shell-validate` (já wirado e já dono de análise de comando
  shell) ou novo fluxo em `bash-guardian`, decidido na implementação — nunca um 2º gate
  independente (`governance-gate.py` **não** é portado como gate paralelo).
- **Contrato**: o binário emite o mesmo shape que os adapters já sabem traduzir
  (`hookSpecificOutput.additionalContext` para Claude/Codex, `decision`/`injectSteps`
  para Antigravity, `{}` fallback) — o adapter **não** ganha lógica nova.
- **Correções obrigatórias** em relação ao original (todos os 6 defeitos do Contexto §5/§6):
  1. `chmod 700` no diretório de cache + nunca cachear stdout que casa a deny-list de
     segredo (`secret-scan` já é a fonte de verdade para isso).
  2. Hash de chave inclui `uid` (não só `cwd:cmd`) e há cap de tamanho por entrada.
  3. Detecção de redirect distingue `>` de `&&` (o original confunde e perde o compressor).
  4. Allowlist de comandos cacheáveis é **cross-platform/read-only determinístico**
     (`lscpu`, `uname`, `cat /etc/os-release`, `git log`) — descartar `yay/paru -Ss`
     (Arch-only) e `journalctl/dmesg -n 35` como default cego.
  5. TTL obrigatório + expiração por mtime; sem cache para comando com `|`/`>`.
  6. Compressor não reescreve comando do usuário sem allowlist explícita (fail-open).

### §2. Conceitos viram ADR markdown — zero runtime

Não portamos `policies.json`, `meta_policies.json` nem `agents_manifest.json`. Os três
conceitos de processo são adotados como **convenção documentada** (0 LOC):

- **Rito Proposta-Primeiro** (RULE-14 deles) — proposta de regra = só texto no chat; escrita
  em disco só com imperativo explícito do usuário. Já é o comportamento praticado
  (confirmado pela política de não criar agent/hook sem pedido — `global-rules.md §3`).
- **Regra 3X** (RULE-10 deles) — 3 recusas consecutivas do mesmo guardrail → congela e
  escala com histórico, em vez de insistir.
- **Modo Sticky** (RULE-13 deles) — sessão continua no agente corrente até saída explícita
  ou invocação de outro. Relevante só se/quando o `agent-sync` operar agente de sessão.

### §3. Rejeitado explicitamente

| Item do repo externo | Motivo da rejeição |
|---|---|
| `governance-gate.py` como 2º gate | Viola §4 (Python-zero) e duplicaria a deny-list do `secret-scan` |
| `policies.json` RULE-01..15 | Só 5/15 têm código; o resto é falso senso de segurança declarativo |
| `chmod 444` como "imutabilidade" | Revertível pelo dono; sem hash/assinatura/`chattr +i` |
| `install.sh` (cp + overwrite de `hooks.json`) | Clobbera hooks de outros plugins; aqui `apply_table.go` faz merge idempotente |
| `META-03` (esconder veto) | Contradiz o nudge explícito do `agent-sync`; mascara prompt injection |
| Regex Hyprland/Arch (`monitors.lua`, `yay -S`) | Específico do host deles; `agent-sync` é cross-CLI dev |
| Cache sem higiene (`env`/dumps com segredo) | Vaza segredo para disco legível — corrigido no §1.1 |

   segredo; compressor trata `&&` como redirect (`re.search(r'[>|&]')`) e desiste.
6. `META-03` do repo deles **proíbe mostrar o veto ao usuário** (remodela em silêncio).
   O `agent-sync` faz o oposto: nudge explícito (padrão consolidado em `secret-guard`/
   `bash-rm-guardian`).

## Consequências

### Positivas

- Ganho de tokens real (compressor + cache TTL) sem nova linguagem, sem novo gate e sem
  nova fonte de verdade: a lógica fica em Go e os adapters continuam finos.
- Deny-list continua sendo a do `secret-scan` (30+ padrões, paridade 1:1 testada) —
  não a regex incompleta de 8 padrões do original.
- Os 3 conceitos de processo ficam registrados sem custo de runtime.

### Negativas / limitações

- `overwrite` reescreve comando do usuário — exige allowlist conservadora para não surpreender;
  sem isso, risco de comportamento inesperado em shell script.
- Cache em disco é superfície nova: exige higiene (§1.1) e pode desatualizar em repo ativo
  (mitigado por TTL curto para estado de repo).

## Reversibilidade

- §1: remover a flag/estágio do binário Go restaura o comportamento atual (hooks só
  permitem/negam/injetam). Cache é artefato descartável em `~/.cache/agent-sync/`.
- §2: as convenções são markdown — reverter é remover a seção, sem impacto de código.

## Não escopo

- Copiar qualquer `.py` do repo externo (proibido por `global-rules.md §4`).
- Reescrever o `install.sh` deles ou adotar `policies.json`/`meta_policies.json`.
- Adotar `force_ask` como novo contrato — o `agent-sync` já cobre aviso/warn sem bloqueio.

## Refs

- Repo externo: `nickarab/bash-sudo-agents` — `plugin/hooks/governance-gate.py`,
  `plugin/hooks.json`, `security/policies.json`, `install.sh` (v4.2)
- `rules/global-rules.md §4` — política de linguagens (Go-fonte, TS-borda, shell-cola, Python-zero)
- `docs/ADR-linguagem-hooks-por-host.md` (Aceito) — linguagem por host
- `docs/ADR-secret-guard-cross-cli.md` (Aceito) — deny-list como fonte de verdade
- `tools/cmd/secret-scan/` — fonte única da deny-list + redação
- `hooks/bash-rm-guardian.sh`, `hooks/docs-cache.sh` — cores com detecção de contrato (fold A/B)

