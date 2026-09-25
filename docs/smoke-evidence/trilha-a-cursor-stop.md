---
date_utc: 2026-09-19T19:58:00Z
duration_min: 8
cursor_version: "1.138.0"
branch: feat/docs-session-2026-09-19
head: affa0bf204921a95517730e956f5d9f74744b3ec
wiring_pre_smoke: pass
wiring_post_smoke: pass
cenarios:
  C1_bloqueio:
    triggered: yes
    evidence: |
      Pós-fix (Glob-only + alegação):
      {"flagged":true,"reason":"alegação de sucesso sem transição de estado (mutação) ou comando de validação no turno recente"}
      agent-stop → followup_message (bloqueio preservado).
  C2_passou:
    triggered: yes
    evidence: |
      Pós-fix (Cursor Shell + alegação):
      {"flagged":false,"reason":"alegação de sucesso confirmada por evidência de execução no trace"}
      agent-stop → {}
      Também: transcript live 4d86e587-… + claim → {} (Shell no turno conta).
  C3_erro_tool:
    triggered: yes
    evidence: |
      go test ./tools/... na raiz: FAIL módulo; cd tools && go test ./...: ok.
criteria:
  C1_duracao: pass
  C2_bloqueio: pass
  C3_passou: pass
  C4_wiring_persistente: pass
  C5_sem_regressao: pass
notes: |
  Fix aplicado nesta sessão: hook.go RanTestCommand passa a reconhecer
  substring "shell" (tool Cursor "Shell"), além de bash|test|command.
  Teste: TestRunHookAcceptsCursorShellAsVerification. make install.

  Round 1 do smoke (antes do fix): C3_passou fail por falso positivo Shell.
  Round 2 (após fix+reinstall): C1 ainda bloqueia (Glob); C2 passa (Shell).

  Evidência de wiração e C5 (agent-react-nudge.stop) no round 1.
