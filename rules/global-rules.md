# Regras Globais

## 1. Regras Fundamentais & Segurança
- Responda sempre em português do Brasil (PT-BR), objetivo e sem rodeios. Comentários/commits seguem a convenção do repo.
- Nunca consulte `.env` nem exponha credenciais/chaves. Não commite sem solicitação explícita.
- Nunca execute comandos destrutivos (`git reset --hard`, `force push`, `rm -rf`, `git clean -f`) sem confirmação.
- Não execute testes unitários quando não existir implementação real.

## 2. Pensar Antes de Codar & Anti-Alucinação
- **Hipótese ≠ Fato**: Nunca assuma nomes de funções, arquivos, rotas ou versões sem verificar no código/docs (`grep`, leitura, `--help`).
- Toda hipótese requer validação imediata no turno ou pergunta objetiva ao usuário. Não construa cadeias de suposições.
- Em tarefas grandes ou ambíguas, resuma o plano em poucas linhas e confirme antes de alterar múltiplos módulos.

## 3. Simplicidade & Mudanças Cirúrgicas (Clean Code)
- **Cirúrgico**: Toque apenas no estritamente necessário para o escopo pedido. Não "melhore" nem reformate código adjacente que funciona.
- **Simplicidade**: Mínimo código funcional. Sem abstrações especulativas, sem flexibilidade não solicitada e sem tratamento para erros impossíveis.
- Prefira editar código existente a criar novos arquivos. Nomes expressivos, funções curtas (SRP) e early returns (Object Calisthenics).
- Ao encontrar débito técnico ou código morto fora do escopo, aponte ao usuário em vez de mexer silenciosamente.

## 4. Persistência de Erros & Verificação
- Trate sempre a causa raiz comprovada, nunca apenas paliativos.
- Em caso de falha/bug, registre o que desencadeou, a causa comprovada e como evitar reincidência para não entrar em loops de tentativa e erro.
- Antes de declarar pronto, verifique na prática (testes/typecheck reais) ou declare explicitamente o que não pôde ser testado.

## 5. Contexto & Janela (Anti-Degradação)
- Em tarefas multi-etapa (3+ passos) ou sessões longas: use a skill `context-guard` e mantenha o `STATE.md` atualizado.
- Evite despejar arquivos inteiros no contexto (prefira outlines, buscas cirúrgicas e trechos delimitados).
- Ao final de cada turno, finalize com 1-2 frases resumindo o que mudou e o que falta.
