# Arquétipos de tela (catálogo opcional)

Catálogo para decidir **a hierarquia de cada aba**, não para montar o app inteiro a partir dele.
Os cinco arquétipos abaixo correspondem a modelos mentais diferentes: visão geral, passado,
inventário, comparação e criação. Escolha pelo que a aba responde, não pelo que já existe no código.

**Sintoma de uso errado:** todas as abas com o mesmo esqueleto (ora overline em caixa alta, ora
número gigante, ora hairline de 1px, ora todos os três). Isso é a síndrome de PDF editorial do §2 —
o §5 do SKILL.md já exige ritmo variado; aqui a variação é estrutural.

| Arquétipo | Estrutura | Cuidado |
| --- | --- | --- |
| **Visão geral** | Âncora de alto contraste (hero) com alternador de escopo + indicador de ritmo/progresso + 2–4 métricas em grid assimétrico + gráfico interativo + lista curta do que vem | Virar dashboard de KPIs iguais. A âncora deve responder "como estou", não "quanto tem" |
| **Feed / histórico** | Resumo compacto do período no topo + busca + filtros em pílula + lista agrupada por data | Lista infinita sem estado vazio nem agrupamento é tabela disfarçada |
| **Gestão de objetos** | Abas segmentadas no topo + cards que representam o objeto real + barra de progresso/limite + ação direta no card | Card sem representação visual do objeto (só texto e ícone genérico) perde o arquétipo |
| **Análises** | Proporção (donut/barra comparativa) ao lado da métrica-chave + lista comparando realizado vs. meta | Nunca só cor: proporção precisa de rótulo e valor; donut acima de 5 fatias vira ruído |
| **Criação / formulário** | Seletor de tipo no topo + palco central para o dado principal + campos em blocos táteis com rótulo que muda por tipo | Rótulo fixo em formulário polimórfico é o erro clássico ("Data" quando deveria ser "Recebimento") |

**Detalhe não é aba.** Detalhe de item é camada (bottom sheet, drawer, rota filha) aberta a partir
de feed, objeto ou gráfico. Uma aba de detalhe sem objeto não tem âncora e quebra o modelo mental.

## Regras de aplicação

- **Uma aba, um arquétipo.** Duas abas com o mesmo arquétipo são candidatas a fusão.
- **A âncora muda, o esqueleto de navegação não:** barra, cabeçalho e tokens permanecem; o que varia
  é a composição do conteúdo. Consistência de shell e variação de conteúdo convivem.
- **Cada arquétipo herda o ciclo completo do §6** (loading, empty, error, success) quando a aba
  depende de dado. Arquétipo sem estados é maquete.
- **Descreva hierarquia, não componente.** "Hero com o saldo do período" deixa espaço para a marca;
  "card `rounded-2xl shadow-lg` com número 40px" é o template que o §2 bane.
- **Mobile:** um arquétipo por viewport. Escada de informação vertical, nunca duas colunas de
  conteúdo no celular.
