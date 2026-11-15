# Consumer Fintech — app de finanças pessoais

Leia este reference quando a tela for de finanças pessoais: carteira, cartões, orçamento,
parcelamentos, fluxo de caixa, faturas. Vale para web (protótipo HTML) e nativo
(Compose/XML/iOS) — declare qual antes de escrever código, porque navegação e insets diferem (§9).

Não repete §2 (deny-list), §3 (tokens), §4 (tipografia) nem §8 (acessibilidade): eles valem
inteiros aqui. O que este arquivo acrescenta é o **modelo de dinheiro**, a **formatação** e a
**dataviz** do domínio.

## 1. Modelo de dinheiro (não negociável)

Quatro estados, e todo número na tela declara a qual pertence:

| Estado | Significado |
| --- | --- |
| **Efetivado** | já liquidou — caiu ou saiu da conta |
| **Previsto** | inclui agendados e futuros do ciclo; não liquidado |
| **Reservado** | dinheiro carimbado (metas, cofrinhos, caixa de emergência) |
| **Saldo livre** | `Total − Reservado − Faturas abertas` — o que dá para gastar hoje |

- A palavra "saldo" sozinha é proibida. É `saldo efetivado`, `previsto` ou `livre`.
- A fórmula do saldo livre é visível ao usuário (card explicativo), não só internalizada no código.
- Toda derivação que a UI afirma precisa existir como função testável, uma fonte de verdade só,
  com a mesma regra na Home, na tela de contas e no extrato. Três implementações divergentes é o
  bug mais caro deste domínio.
- Casos de borda exigem resposta explícita antes de codar: saldo livre negativo, fatura fechando
  hoje, parcela que cai depois do corte, reserva em outra moeda, estorno que chega no ciclo seguinte.
  Sem resposta definida, mostre o valor com rótulo honesto ("pode variar até o fechamento").

## 2. Microcopy por tipo

| Campo | Despesa | Receita | Transferência |
| --- | --- | --- | --- |
| Data | Vencimento | Recebimento | Data |
| Status | Pago / Pendente | Recebido / A receber | Concluída / Agendada |
| Ação primária | Pagar | Confirmar recebimento | Concluir |

- Cartão não tem "vencimento": tem **fechamento** (corte) e **vencimento** (pagamento). Não misture.
- Parcelamento fala em **nº de parcelas restantes** e **valor liberado a partir de \<mês\>**, não em "total".
- Sem vocabulário de marketing: "Economize sem esforço" não é microcopy de app financeiro.

**Dicionário (frieza → humano).** O rótulo do código e da acessibilidade continua técnico; a
camada humana é só o texto que aparece na tela:

| Em vez de | Na tela |
| --- | --- |
| Saldo livre / Safe to spend | "Para gastar com tranquilidade" |
| Despesas por categoria | "Onde seu dinheiro foi gasto" |
| Projeção de fluxo de caixa | "Seu dinheiro nos próximos meses" |
| Pendências / Passivos | "Próximos compromissos" |

- Uma tela, um nome. `Saldo livre` é o nome canônico (código, fórmula, `sr-only`); a frase humana
  substitui o texto visível, nunca o rótulo de acessibilidade nem o identificador no código.
- Não crie sinônimo para o mesmo número em telas diferentes: se a Home diz "Para gastar com
  tranquilidade" e a tela de Contas diz "Livre para usar", o usuário acha que são dois valores.

## 3. Formatação de valores

- `font-variant-numeric: tabular-nums` em **todo** valor monetário, percentual e data de lista.
  Sem isso a coluna de números dança entre linhas.
- Moeda via `Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' })`. Nunca concatene `"R$ "`.
- Agregado/KPI: no máximo 2 casas; lista de lançamentos: 2 casas fixas (nunca 1 em uma linha e 2 na outra).
- Negativo com sinal de menos explícito **e** cor/ícone — nunca entre parênteses contábeis.
- **Data não é timestamp.** Lançamento guarda o instante com timezone e exibe no fuso do usuário;
  campos de calendário guardam data pura. `toISOString().slice(0,10)` desloca o dia para quem está
  a oeste de UTC — bug clássico de "lançamento no dia anterior".
- **Modo privacidade:** botão de olho troca o valor por `••••••` mantendo a largura do bloco
  (mesma contagem de caracteres ou `min-width`, jamais `display:none`), sem reflow nos cards.
  O valor oculto não pode ir para o DOM de leitor de tela nem para `aria-label`: marque o visível
  com `aria-hidden` e anuncie "valor oculto" num `sr-only`.
- **Acento vivo (lima, coral, âmbar)** só existe como **fundo** com tinta escura por cima
  (`--accent` + `--accent-ink`, ~10:1). Nunca como cor de texto, valor, borda ou anel de foco:
  lima sobre branco fica em 1.2:1 e reprova o mínimo de 3:1 de SC 1.4.11. Acento claro em texto
  é o erro de paleta mais comum em UI gerada.

## 4. Dataviz financeira

- **Eixo zero real.** Em projeção de saldo/fluxo, colunas positivas sobem acima da linha zero e
  negativas descem abaixo, em `--out`. Barra horizontal cheia para a direita representando saldo
  negativo está errada — é o erro mais comum do domínio.
- **Ritmo de gastos (spending pace):** barra do gasto atual contra o teto do mês + marcador
  vertical de "Hoje" posicionado por `dia_atual / dias_do_mês`. Diga em texto o que a barra mostra
  ("gastou 68% do mês em 55% do tempo"); marcador sem legenda é decoração.
- **Gráfico clicável** atualiza um painel de contexto com a causa do número, não um tooltip de valor.
- **Cor nunca sozinha** (WCAG 2.1 SC 1.4.1): entrada/saída carregam sinal, ícone ou rótulo além de
  verde/vermelho. Vale para déficit projetado e para categoria.
- Rosca/donut com no máximo 5 fatias + "Outros"; ordem por valor; legenda com valor, não só cor.
- Percentual de economia sempre com o denominador explícito ("12% da receita do mês").

## 5. Navegação e densidade mobile

- App financeiro é **Operate**: escaneabilidade, consistência e expectativa da plataforma acima
  de expressão. Gestos estéticos ficam para a marca, não para o fluxo de lançamento.
- 3 a 5 destinos principais com rótulo curto. **Piso único de 11.5px** para rótulo de barra e
  metadado — 9px e 10.5px não existem no celular. Um número só, não uma faixa "11–11.5".
- Ícone de categoria e de instituição é SVG ou monograma de 2 letras sobre fundo suave. Emoji do
  sistema (🛒, ☕, 🏍) e glifo Unicode (⌂, ⇄) nunca: variam por plataforma e quebram o alinhamento óptico.
- Conteúdo nunca atrás da barra inferior: `padding-bottom` equivalente à altura da barra + inset,
  ou safe area nativa.
- Ação primária de criação ("novo lançamento"): em nativo use FAB/`extended FAB` do Material 3 ou
  barra do iOS — recriar a barra para um "+" central elevado viola §9. Em protótipo web é permitido,
  desde que não seja o mesmo dock em todas as telas.
- Cada tela tem sua âncora visual (hero de carteira, plásticos de cartão, donut + metas). Repetir o
  mesmo cabeçalho overline + número gigante + hairline em todas as abas é a síndrome de PDF
  editorial que o §2 bane — a âncora muda, o esqueleto de navegação não.

## 6. Precedência com outras skills

- `ui-ux-pro-max` recomenda Glassmorphism + Dark Mode OLED para "Personal Finance Tracker" e
  "Fintech/Crypto" (data/products.csv). É recomendação genérica de catálogo e **cede** a este
  reference e ao §2: app de finanças pessoais não é cripto nem landing de banco.
- Nomeie o modo do `impeccable` como **Operate** antes de desenhar.

## 7. Aceite específico (além do §11)

- [ ] Todo número na tela declara o estado (efetivado / previsto / reservado / livre).
- [ ] A fórmula do saldo livre existe em um único lugar no código, e um teste cobre ela.
- [ ] `tabular-nums` em valores, percentuais e datas de lista.
- [ ] Negativo em gráfico aponta para baixo de um eixo zero real.
- [ ] Verde/vermelho nunca é o único portador do significado.
- [ ] Modo privacidade não desloca layout e não vaza o valor no DOM.
- [ ] Data de lançamento testada em fuso diferente de UTC.
- [ ] Dataviz financeira conferida em light e dark mode com os tokens reais.
