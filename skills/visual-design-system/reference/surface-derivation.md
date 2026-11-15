# Derivação de superfície (direção → número)

Complementa §3 (tokens) e §5 (espaço/densidade). Não é catálogo de estilos — é a conversão de
"como isso deve parecer" nos quatro números que o agente erra por omissão: **raio, sombra, borda e
delta entre superfícies**.

Sofisticação não é mais efeito. É menos elementos com mais precisão: uma escala de raio curta, uma
sombra por elevação, uma linguagem de borda só. Isso aparece medido abaixo.

## 1. Medição de referência (não é gosto, é CSS de produção)

Fonte: bundle CSS público de cada produto, busca de `border-radius`, `box-shadow`, `border`,
`font-size` e hex. Reproduzível com `curl` + regex; nenhum valor abaixo foi estimado.

| | GOV.UK | IBM Carbon | Shopify Polaris | Radix Themes |
| --- | --- | --- | --- | --- |
| Perfil | contido/editorial | denso/técnico | consumo/comércio | expressivo/consumo |
| `border-radius` | **1 declaração no CSS inteiro** (≈ zero) | 2px×10, 4px×8, 8px×3, 12px×2, 16px×4 | tokens 0/2/4/6/8/12px | 6 níveis 3/4/6/8/12/16px, todos ×`--radius-factor` |
| Sombra | 18 declarações, **todas** com `#fd0` (anel de foco); zero sombra de card | 80 declarações, elevação resolvida por borda 1px | 3 tiers: `--p-shadow-100/200/300` | 6 tiers; o tier 2 tem **5 camadas** (ring + 4 offsets) |
| Borda | estrutura por `border-bottom`, 41 declarações | 408 declarações, 50 em `1px solid` | 121 declarações, todas por token de largura | 7 no total; o ring vive dentro do `box-shadow` |
| Famílias | 1 (GDS Transport) | 2 (Plex Sans + Mono) | 2 (sans + mono) | 1 + mono |
| Escala de fonte | 12–48px, base **19px**, 12 degraus | 12–60px, 13 degraus | por token, 1 valor literal (15px) | por token |

Três consequências que contrariam o instinto:

1. **Produto mais sofisticado não tem mais raio.** GOV.UK tem zero e é referência de craft. Carbon
   usa 2–4px e reserva 8–16 para overlay. Raio grande é sinal de consumo, não de qualidade.
2. **Sombra é o eixo mais elástico** (0 → 3 → 6 tiers) e o mais fácil de errar: Radix compensa o
   exagero de raio com sombras de 5 camadas; Carbon dispensa sombra e usa borda.
3. **Nível de expressão aparece na quantidade de camadas, não no blur.** `0 10px 30px` com 4% de
   alfa é uma escolha, não uma regra — o medido vai de 1px/6% a 5 camadas.

**Caso real medido (APK `minhas-financas`, via `apktool`, 1036 cores, 40k arquivos).** Serve como
evidência de deriva, não como modelo: 16 valores de raio em uso (0/2/3/4/5/8/10/12/16/18/20/22/24/
28/36/56) contra o limite de 3; elevação 142×`0dp`, 7×`1dp`, 2×`4dp`; borda 18×`1dp`; três tokens
de fundo concorrentes (`#ffffff`, `#dedede`, `#e3e3e3`) com card `#f8fafd`; 20 `textSize` distintos em
uso; `colorPrimary #1e88e5` (3.68:1 sobre branco) e `colorAccent #0f9d58` (3.51:1); nenhum
`values-night/colors.xml`, ou seja dark sem token próprio. Resultado: raio de perfil consumo,
elevação de perfil contido, escala inexistente — a combinação que o §2 abaixo proíbe.

## 2. Eixos → valores

Escolha dois eixos antes de escrever a primeira classe. Não invente um quinto nível.

**Os eixos são independentes, não uma escada.** Dá para ter raio de consumo (16–24px) com elevação
de denso (0) — é o padrão mais comum em app Android de terceiro. Nesse caso a separação tem de vir
do delta entre superfícies (≥ 8%) ou de borda 1px; raio alto + sombra zero + delta abaixo de 5% é
a “caixa de wireframe arredondada”, que junta o pior dos dois perfis.

| Eixo | Contido | Denso | Consumo | Expressivo |
| --- | --- | --- | --- | --- |
| Raio (base / overlay) | 0 / 2 | 2 / 4 (8–16 só overlay) | 8 / 12–16 | 12 / 16–24, themeable |
| Níveis de raio no produto | 1–2 | 2 | 3 | até 6, via variável |
| Sombra | nenhuma; elevação por borda ou fundo | 1 tier quase invisível (`0 1px 2px`, α ≤ .06) | 2–3 tiers difusos, α ≤ .08 | 3 tiers, multicamada |
| Borda | 1px `--line` como estrutura | 1px em tudo que separa | só em input e divisória | quase nenhuma; ring no shadow |
| Delta canvas→surface | ≤ 3% de luminância | 4–6% | 8–12% | 10%+ com cor, não cinza |
| Escala tipográfica | 1.25, base 18–19px | 1.2, base 14–16px | 1.25, base 16px | 1.333, display tracking −0.03em |
| Pesos por tela | 2 | 2 | 2 | 2–3 |

Regras que saem daí:

- **No máximo 3 valores de raio por produto** (base, pequeno, overlay). Quatro é vazamento de
  escala; seis exige que todos venham de uma variável (o caso Radix).
- **Uma sombra por elevação, e escolha um mecanismo**: ou borda, ou sombra. Os dois juntos com peso
  igual são o "card soup" do §2.
- **Dark mode troca sombra por borda** (§3); portanto a sombra nunca pode ser a única portadora da
  hierarquia — o contraste de superfície precisa carregar sozinho.

## 3. Dois níveis de variação (não confunda)

- **Entre produtos** varia o sistema: paleta, raio, sombra, densidade, escala.
- **Entre telas do mesmo produto** varia a composição e a âncora (ver
  [product-archetypes.md](product-archetypes.md)); **nunca** paleta, raio ou sombra. Aba com sombra
  diferente da outra é produto com quatro estéticas.

## 4. Três exemplos trabalhados

**Ferramenta interna densa.** Raio 4px em tudo, overlay 8px. Uma sombra `0 1px 2px rgba(0,0,0,.06)`.
Borda 1px `--line` em toda separação. Canvas `#f7f7f7`, surface `#ffffff` (delta ~4%). Escala 1.2,
base 14px, Mono só em id e valor técnico. Densidade de lista: 40–44px de altura de linha.

**App de consumo acolhedor.** Raio 16px em card, 20px em hero, 999px em pílula (3 níveis). Sombra
`0 10px 30px rgba(16,38,26,.045)` só no hero; cards sem sombra, separados por fundo. Canvas e
surface com delta 8–10%, ambos tintados na matiz da marca. Escala 1.25, base 16px, dois pesos.

**Editorial calmo.** Raio 0, zero sombra de card, elevação por fundo alternado. Borda 1px só em
divisória horizontal. Canvas `#f3f2f1`, surface `#ffffff`. Escala 1.25, base 19px, medida de 65ch.

Nos três, antes de entregar: medir os pares de texto sobre canvas e sobre surface (§3) e conferir
que a sombra escolhida não é o único sinal de hierarquia.

## 5. Proibido

- Entregar a paleta desta referência como tokens reutilizáveis. Os valores acima são **faixas** e
  servem para calibrar; hex de marca vindo daqui é o mesmo erro que "escolher a visão 7" num catálogo.
- Reaproveitar os números de outro produto só porque o segmento é parecido. Carbon é denso porque é
  console de nuvem; o produto denso do outro projeto pode ser denso por outro motivo.
