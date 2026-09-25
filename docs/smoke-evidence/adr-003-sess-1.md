---
session: 1
date_utc: 2026-09-19T19:46:50Z
duration_min: 3
branch: n/a (/tmp/smoke-adr-003 isolado)
head: n/a
events_total: 26
by_kind:
    action: 6
    blocker: 1
    decision: 7
    open_question: 1
    state_render: 11
by_actor:
    agent-sync: 26
criteria:
  # Nomes do script collect-smoke-evidence.sh ainda refletem v1;
  # mapeamento D-23 / SMOKE-TEST-Q v2 abaixo.
  C1_cenarios: pass          # ≥2: C1 (mix kinds) + C3 (compactação); C2 parcial (status/no-op)
  C2_eventos: pass           # 26 ≥ 20
  C3_mix_kinds: pass         # 5 chaves ≥ 3
  C4_sobrevive_compact: pass # sha256 idêntico antes/depois; event read OK; state render falha
  C5_state_render: pass      # 11 ≥ 3
  C6_reorientacao: pass      # ver notes
compact_test:
  triggered: yes
  trigger_time_min: ~3
  recovered_via: agent-sync event read -last 50 -root /tmp/smoke-adr-003
  before_sha256: 2b3c526dd8409d938da54655b5900ffcf95ebc8e1ccd6dcac6455d36cadc71b5
  after_sha256: 2b3c526dd8409d938da54655b5900ffcf95ebc8e1ccd6dcac6455d36cadc71b5
notes: |
  Cenários: C1 completo + C3 completo (+ C2 parcial: A-2 pending→done→cancelled→no-op).
  Combinação recomendada do PACOTE (C1+C3).

  C1: 3 state writes → 10 eventos, 5 kinds (decision/action/blocker/open_question/state_render).
  Gap no PACOTE: open_questions no schema real é []string, não objetos {id,question}.
  Payload do step3 do PACOTE falhou com "cannot unmarshal object into ... string";
  corrigido no smoke usando string pura; event open_question ainda emite ref Q-N.

  C2 parcial: mudança de status emite details.status; no-op (mesmo conteúdo) emite
  só state_render (+1), sem per-item — conforme esperado.

  C3: rm session-state.json; log intacto (26 linhas, sha match); state render/next-action
  falham; event read/stats funcionam.

  Re-orientação (C6):
  - Consegui re-orientar sem ler STATE? sim
  - Informações perdidas? rationale completo (só rationale_len no event);
    blocking_action_ids do blocker; status "atual" das actions exige ler última
    ocorrência por ref (A-2 passou por pending/done/cancelled/pending).
  - 1ª ação a retomar: A-2 "escrever testes" (status final pending) após D-3
    "usar porta 8081" (B-1 histórico; bloqueio resolvido no STATE antes do compact).

  Validação rápida:
  - total=26, kinds=5, wc -l session-event.jsonl=26
  - log em /tmp/smoke-adr-003/.agent-sync/session-event.jsonl (STATE ausente pós-compact)
