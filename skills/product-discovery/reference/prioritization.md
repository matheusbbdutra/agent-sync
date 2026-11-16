# Priorização e direito de decisão

Fonte primária: `deanpeters/Product-Manager-Skills`, `skills/prioritization-advisor/SKILL.md` e
`skills/decision-rights-matrix/SKILL.md` (CC BY-NC-SA 4.0). Reescrito; nenhuma frase copiada.

Priorização é decisão, não planilha. Duas perguntas separadas: **qual framework encaixa no estágio**
e **quem decide**. Escolher o score sem definir o decisor produz número que ninguém segue.

## Escolher o framework pelo estágio

Escolha por estágio, contexto de time, tipo de decisão e dados disponíveis — não por preferência
(`prioritization-advisor#Application`, perguntas 1-4).

| Situação | Framework | Por que |
|---|---|---|
| Há dados de uso e métricas por iniciativa | **RICE** | Exige reach e impacto estimáveis |
| Sem dados, decisão rápida | **ICE** (impact, confidence, ease) | Gut-check estruturado |
| Escolha visual de portfólio | **Value × Effort** (2×2) | Quick wins vs apostas estratégicas |
| Múltiplos critérios já acordados | **Weighted scoring** | Critérios com peso declarado |
| Precisa arrancar escolha dura | **MoSCoW** | Forcing function |
| Prazo e janela importam mais que valor | **Cost of Delay** | Urgência |
| Objetivo → iniciativa | **Impact mapping** | Amarra feature a outcome |
| Jornada do usuário | **Story mapping** | Fluxo narrativo |
| Básico vs delighter | **Kano** | Classe de satisfação |
| Lacuna de satisfação | **Opportunity scoring** | Importância × satisfação |
| Cliente aloca budget | **Buy-a-Feature** | Preferência com restrição |

Regras:

- **Pré-product-market-fit com RICE é erro de encaixe**: não existe reach confiável antes de saber
  quem é o cliente (`prioritization-advisor#Example 2: Bad Framework Match`).
- Trocar de framework a cada ciclo é whiplash: ninguém aprende a ler o score
  (`prioritization-advisor#Pitfall 2: Framework Whiplash`).
- Declare **quando reavaliar**: mudança de estágio, reorganização de time, mudança de stakeholder,
  ou quando o framework começa a atrapalhar (`prioritization-advisor#Reassess When`).

## RICE

```text
score = (Reach × Impact × Confidence) / Effort
```

Escalas verificadas no material (`prioritization-advisor#Step 1: Define Scoring Criteria`):

- **Reach** — quantos usuários por mês/trimestre.
- **Impact** — 1 = mínimo, 2 = alto, 3 = massivo.
- **Confidence** — 50% (dado fraco), 80% (dado bom), 100% (certeza).
- **Effort** — pessoa-mês, incluindo design, engenharia e QA.

Regras de uso:

- Pontue com PM, design e engenharia juntos; PM sozinho pontuando é pitfall explícito
  (`prioritization-advisor#Pitfall 4: Solo PM Scoring`).
- Não infle Confidence para fazer a iniciativa subir — é o campo que mais distorce
  (`prioritization-advisor#Step 2: Score Each Feature`).
- Ordene por score, revise o top 10-20 com stakeholders e ajuste por estratégia; o score não captura
  tudo (`prioritization-advisor#Step 4: Review and Adjust`).
- Score é insumo, não veredito (`prioritization-advisor#Pitfall 3: Treating Scores as Gospel`).

Template mínimo:

| Iniciativa | Reach | Impact | Confidence | Effort | Score | Evidência de reach |
|---|---|---|---|---|---|---|
| A | 1200 | 2 | 80% | 3 | 640 | funil, mês anterior |
| B | 400 | 3 | 50% | 1 | 600 | entrevista (n=6) |

A coluna "evidência de reach" é o que impede a tabela de virar opinião com duas casas decimais.

## Direito de decisão (DACI)

RACI mapeia **trabalho**: responsible (faz), accountable (dono do resultado, uma pessoa),
consulted (input antes), informed (sabe depois). DACI mapeia **uma decisão**: driver (conduz o
processo, normalmente o PM), approver (uma pessoa que decide), contributors (trazem input e
evidência), informed (`decision-rights-matrix#RACI and DACI`).

O que sobrevive a qualquer variante (RAPID, modelo de consentimento): **um decisor**.

Regras:

- Dois accountable = a matriz não decidiu nada. Se a resposta honesta é "os dois", **divida a
  decisão**: "o que construir" é do PM, "como construir" é da engenharia
  (`decision-rights-matrix#One Accountable Person`).
- Não mapeie tudo. Comece pelas 5-8 decisões onde já existe atrito
  (`decision-rights-matrix#Pick the Decisions That Hurt`):

  | Decisão | Conflito típico |
  |---|---|
  | O que entra no roadmap | pedido de vendas vs estratégia vs capacidade |
  | Preço e regra de desconto | vendas vs produto vs finanças |
  | Escopo e requisitos | produto vs engenharia |
  | Go / no-go de lançamento | produto vs vendas vs suporte vs jurídico |
  | Entrada/saída de portfólio (inclui descontinuação) | produto vs finanças vs liderança |
  | Compromisso específico com cliente | vendas vs engenharia |

- Separe três coisas que as pessoas confundem: **quem decide**, **quem precisa ser consultado**
  (input ouvido ≠ concordância) e **quem pode vetar** (raro, e só por motivo declarado, como
  jurídico ou segurança) (`decision-rights-matrix#Decide Vs. Input Vs. Veto`).
- Consulted agindo como approver é o conflito mais comum; approver decidindo sem consultar é o
  segundo (`decision-rights-matrix#Pitfall 2: Consulted Acting as Approver`).
- Nomeie quem escalou e para onde, antes de precisar (`decision-rights-matrix#Step 3: Define Forums and Escalation`).

### Decision log

Uma linha por decisão, no mesmo lugar sempre (`decision-rights-matrix#Step 4: Set the Log`):

```text
data | decisão | approver | consultados | evidência usada | o que reabriria
```

Sem log, a decisão é reaberta por quem não estava na sala — e a próxima reunião recomeça do zero.

## Pitfalls

- **Framework nenhum**: prioridade vira quem grita mais alto (`prioritization-advisor#Pitfall 5: No Framework at All`).
- **Score como verdade**: o número decide sozinho e a estratégia desaparece (`Pitfall 3`).
- **PM pontuando sozinho**: score sem engenharia é estimativa de effort inventada (`Pitfall 4`).
- **HiPPO**: executivo define roadmap e o framework vira teatro
  (`roadmap-planning#Pitfall 2: Prioritizing by HiPPO`).
- **Mapear tudo**: matriz de 40 linhas que ninguém mantém (`Pitfall 4: Mapping Everything`).
- **Designação sem acordo**: papel atribuído no documento e não confirmado com a pessoa
  (`decision-rights-matrix#Pitfall 3: Assignments Nobody Agreed`).
