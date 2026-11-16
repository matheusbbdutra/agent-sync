# Regras Globais

## 1. Regras Fundamentais & Segurança
- Responda sempre em português do Brasil (PT-BR), objetivo e sem rodeios. Comentários/commits seguem a convenção do repo.
- Nunca consulte `.env` nem exponha credenciais/chaves. Não commite sem solicitação explícita.
- Nunca execute comandos destrutivos (`git reset --hard`, `force push`, `rm -rf`, `git clean -f`) sem confirmação.
- Não execute testes unitários quando não existir implementação real.

## 2. Pensar Antes de Codar & Anti-Alucinação
- **Hipótese ≠ Fato**: Nunca assuma nomes de funções, arquivos, rotas ou versões sem verificar no código/docs (`grep`, leitura, `--help`).
- **Verdade absoluta**: registre apenas afirmações verificáveis empiricamente (git, doc oficial, runtime). Sem evidência file:line ou comando+sintaxe = sem afirmação. "Acho que" / "geralmente" / "normalmente" sao proibidos em resposta. Meia-verdade = mentira.
- Toda hipótese requer validação imediata no turno ou pergunta objetiva ao usuário. Não construa cadeias de suposições.
- Em tarefas grandes ou ambíguas, resuma o plano em poucas linhas e confirme antes de alterar múltiplos módulos.

## 3. Simplicidade & Mudanças Cirúrgicas (Clean Code)
- **Pragmatismo**: Seja sempre o mais pragmático possível. Priorize a solução mais direta e funcional para o problema real imediato, sem antecipar cenários hipotéticos, abstrações precoces ou complexidade desnecessária (YAGNI).
- **Cirúrgico**: Toque apenas no estritamente necessário para o escopo pedido. Não "melhore" nem reformate código adjacente que funciona.
- **Simplicidade**: Mínimo código funcional. Sem abstrações especulativas, sem flexibilidade não solicitada e sem tratamento para erros impossíveis.
- **Anti-overengineering**: prefira editar existente a criar novo (skill, hook, agente, arquivo). Crie só quando o caminho de edição exigir mais de 50 linhas de patch ou for inviável por motivo estrutural justificado.
- Prefira editar código existente a criar novos arquivos. Nomes expressivos, funções curtas (SRP) e early returns (Object Calisthenics).
- Ao encontrar débito técnico ou código morto fora do escopo, aponte ao usuário em vez de mexer silenciosamente.

## 4. Linguagens & Runtime (Go/TS/Shell — sem Python)
> Detalhe e racional em `docs/ADR-politica-linguagens.md` (o **porquê**); §4 é a regra curta.
- **Go é fonte da verdade**: toda lógica nova (audit, cache, budget, memória) vai em Go (`internal/`, `tools/cmd/`). O que está em Go fica em Go. Não reescrever Go em TS.
- **Shell é cola fina (≤50 LOC)**: só traduz contrato do host (Claude `{}` vs Antigravity `{"decision":"allow"}`) e chama binário Go. Zero lógica de negócio.
- **TS só na borda onde a API obriga** (OpenCode v2 `hooks/*.v2.ts` + Cline `cline-plugin/src/hooks/`): adapter fino (≤150 LOC), sem lógica — delega via `bridge.ts`/`execFile` (`~/.local/bin/<bin>`). TS nunca reimplementa deny-list nem lógica do Go.
- **Python = zero**: proibido criar `.py` novo. Reaproveitamento de código Python externo (ex.: `governance-gate.py`, cache TTL, `overwrite`) deve ser portado para Go, nunca copiado como `.py`.
- **1 fonte de verdade por função**: 1 core Go + adapters finos por host. Proibido duplicar a mesma regra em `.sh` + `.ts` com lógicas divergentes.
- **Janela de contexto vem do host**: percentual de utilização usa o `context_window` nativo do runtime; tabela hardcoded de modelos é só fallback e nunca sobrepõe o dado real (bug 2026-11-11: aviso dizia 81% com 17% real — modelo de 1M caindo no default de 200k).

## 5. Persistência de Erros & Verificação
- Trate sempre a causa raiz comprovada, nunca apenas paliativos.
- Em caso de falha/bug, registre o que desencadeou, a causa comprovada e como evitar reincidência para não entrar em loops de tentativa e erro.
- Antes de declarar pronto, verifique na prática (testes/typecheck reais) ou declare explicitamente o que não pôde ser testado.
- **`ok` do runtime ≠ efeito aplicado**: `hyprctl eval`/`hyprctl reload` retornam `ok` só por aceitar o comando. Sempre releia o estado real (`hyprctl monitors all -j`) após cada passo; outputs desativados não aparecem em `hyprctl monitors -j`.
- **`ok` de pipe ≠ sucesso do comando**: `cmd | head && echo OK` reporta o status do `head`; `grep -c` sem match sai 1. Use `${PIPESTATUS[0]}` ou rode sem pipe antes de declarar pronto.
- **`build`/`install` verde ≠ artefato novo**: confira mtime/hash do binário. `go build -o X` que falha não regrava X, e um install cego propaga o binário velho.
- **Ordem importa em mutação de layout**: desativar monitor à esquerda dispara auto-relayout que sobrescreve posição aplicada depois → reposicione o monitor remanescente ANTES de desativar.
- **Hyprland Lua**: `disabled = true` não é revertido por `mode`/`position`; reativar exige `disabled = false` explícito.

## 6. Contexto & Janela (Anti-Degradação)
- Em tarefas multi-etapa (3+ passos) ou sessões longas: use a skill `context-guard` e mantenha o `STATE.md` atualizado.
- **Memória Enxuta**: Ao gravar no memory-mcp (`store_memory` ou `record_event`), registre apenas fatos e regras acionáveis em formato ultra-conciso (máx 2-3 linhas ou ~180 caracteres). Nunca registre diários, tentativas falhas descartadas ou código transitório já deletado.
- Evite despejar arquivos inteiros no contexto (prefira outlines, buscas cirúrgicas e trechos delimitados).
- **Delegação é o default em task ≥2 etapas**: antes do primeiro `edit`/`bash` de task que toque ≥3 arquivos, ≥2 módulos ou gere saída >100 linhas, dispare `subagent` no pipeline `research∥scout → plan (pai) → worker → review fresco → validate`; o pai mantém plano, decisão e aceite. Task trivial (1 arquivo, <10 tool calls) segue no pai. Research com web exige `async: true`. Ref: `agent-delegate` §Subagentes nativos.
- **Leitura paralela, escrita serial**: 1 writer por cwd; `fork` só quando o brief não substituir o histórico.
- Ao final de cada turno, finalize com 1-2 frases resumindo o que mudou e o que falta.

## 7. Convenções de Processo Agêntico
Detalhe e racional em `docs/ADR-convencoes-processo-agentico.md` (zero runtime, honor-system):
- **Proposta-Primeiro**: propor regra/refactor = só texto no chat. Escrever em disco só com imperativo explícito (`aplique`, `faça`, `execute`) — espelha §3.
- **Regra 3X**: 3 recusas consecutivas do mesmo guardrail → congela, resume o histórico e escala ao usuário em vez de insistir numa 4ª tentativa.
- **Modo Sticky**: sessão continua no agente corrente até `exit`/troca explícita (latente — sem caso de uso no agent-sync hoje).

## 8. Ferramentas & Incidentes Operacionais
Cada item abaixo nasceu de incidente real; são regras duras, não preferências.
- **Android — teste instrumentado nunca em base real**: `connectedAndroidTest`/`uninstall`/`pm clear` apagam os dados do app (2026-11-14: 1091 despesas → 0, recuperado só por reimport de CSV). Isole com build type de teste (`applicationIdSuffix` + `testBuildType`) ou instale o APK de teste e rode `am instrument` direto; backup exportado e conferido antes de qualquer run.
- **Worktree sujo — proibido reverter por comando**: nunca `git checkout -- <path>`, `git restore` ou `git stash` para "desfazer formatação" com trabalho não-commitado presente (2026-11-14: alterações de UI perdidas de um arquivo `M`). Corrija o trecho com `edit`; reverter exige pedido explícito do usuário.
- **Hermes/RN — `hermes-dec` é o contorno**: `~/.local/venvs/hermes/bin/{hbc-file-parser,hbc-disassembler,hbc-decompiler}` decodifica bundle Hermes (v98 pleno; v99 best-effort), e `strings -n 6` complementa copy/rotas. Nunca inferir layout, cor ou hierarquia a partir do bytecode.
- **Rename de pacote Android — preserve componentes com permissão**: FQCN de `Service`/`Receiver` declarado no manifest é a chave da permissão concedida. Mantenha o FQCN antigo com uma subclasse shim, aponte o manifest para ele (nome absoluto) e valide com `aapt2 dump xmltree` comparando o dump antes/depois; `requestRebind`/`ComponentName` devem mirar o shim, não a classe nova.
- **Android — UI de emulador via `mobile-mcp` + `adb emu`, adb puro só onde não há contorno**: tap/swipe/árvore/screenshot/launch usam `mobile_*` (parâmetro `device` = nome do AVD, ex. `financas`, nunca `emulator-5554`), não `input tap` + `uiautomator dump` + parse Python. `adb emu sms send` posta notificação real (substitui `cmd notification post`) e `adb emu resize-display` cobre o `wm size` do Play. `logcat -s`/`dumpsys`/`settings` ficam em adb. Não trocar de emulador (Waydroid/redroid/gRPC): os gaps são da plataforma Android, não do emulador.

