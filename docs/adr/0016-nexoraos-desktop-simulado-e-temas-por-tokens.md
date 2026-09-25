# ADR-0016: NexoraOS — a UI é um computador simulado; temas são conjuntos de tokens

**Status:** Proposto
**Data:** 2026-09-25
**Decisores:** Lucas Zanetti

## Contexto

O Chunk 9 leva o C1 para o navegador. A direção de produto (2026-09-25) é que o jogador esteja **no computador da empresa**: um desktop com apps (e-mail do chefe, dashboard, terminal, testes, notas, configurações) e uma mascote, a Tabby. O visual é pixelado, mas suave.

Um estudo de design explorou três direções de arte (Nexora 98, Plantão noturno, Estúdio pastel) e uma quarta tela em que o tema muda ao vivo pelas Configurações. Pedido do usuário: o jogador escolhe entre as três e personaliza o visual.

O ADR-0011 listou bibliotecas-alvo (uPlot, React Flow, TanStack Query, Zustand, Tailwind + shadcn, renderizador de Markdown). O projeto é de aprendizado, e cada dependência precisa de justificativa.

## Decisão

1. **Metáfora de desktop.** Cada verbo do jogo é um app:

   | App | Verbo | Antes (CLI) |
   |---|---|---|
   | E-mail | briefing, incidente, debrief | `play` / `next` |
   | Pulse | produção agora | `status` |
   | Terminal | shell na rede da Nexora | `shell` |
   | Testes | Quick Check e Production Test | `check` / `test` |
   | Tabby | dicas, uma por vez | `hint` |

   Notas e Configurações não têm equivalente na CLI.

2. **Tema = conjunto de tokens.**
   - Cada tema (`web/src/theme.ts`) é um mapa de variáveis CSS (cores, espessura de borda, raio, sombras, fontes), mais duas funções: a barra de título por app e as cores da Tabby.
   - Os componentes leem só `var(--…)`, nunca o nome do tema. O CSS conhece o tema apenas nas decorações de cena.
   - Personalizar é sobrepor tokens ao tema escolhido: cor de destaque (4 por tema, com contraste conferido), papel de parede, moldura, tamanho do texto, animações e pelagem da Tabby.
   - Um tema novo é um objeto novo; nenhuma tela muda.

3. **Preferências no servidor**, em `~/.config/sesim/ui.json` (`GET/PUT /api/prefs`). A porta do `sesim ui` muda a cada execução, e com ela a origem do navegador, então `localStorage` perderia tudo. A UI normaliza o JSON: o que ela não reconhece volta ao padrão.

4. **Fontes empacotadas** (`@fontsource/*`, só o subset latin, OFL-1.1):
   - a CSP `default-src 'self'` (T5) proíbe o Google Fonts;
   - o jogo precisa funcionar offline;
   - a política de licenças da web passa a aceitar OFL-1.1, que permite empacotar a fonte com software, desde que ela não seja vendida sozinha.

5. **O que o ADR-0011 previa e fica de fora por enquanto**, com o que se ganha e o que se perde:

   | Previsto | Em vez disso | Ganho | Custo |
   |---|---|---|---|
   | uPlot | Gráfico em SVG, com linha em degraus (`Chart.tsx`) | ~120 linhas; combina com o pixel; nada a aprender de API | Sem zoom nem tooltip; não aguenta milhares de pontos (hoje são ≤ 360 por série) |
   | Renderizador de Markdown com sanitização | Subconjunto de Markdown transformado direto em elementos React (`markdown.tsx`), com testes | Seguro por construção (nunca monta HTML); sem dependência | Só o subconjunto que os cenários usam |
   | TanStack Query + SSE | Polling de `/api/state` (1,2 s com job rodando; 4 s sem) e de `/api/pulse` (5 s) | Um único caminho de dados, fácil de depurar | Mais requisições (locais) |
   | Zustand, Tailwind, shadcn | `useState` + CSS com tokens | O tema é o próprio sistema de design | Primitivos acessíveis escritos à mão |
   | React Flow | — | — | Volta com a topologia (Chunk 10) |

6. **A Tabby tem roteiro fixo.**
   - As falas derivam do estado do jogo (`web/src/tabby.ts`) e nunca da solução.
   - Dicas só a pedido, e só durante o incidente.
   - Nada de IA antes do Playtest #2 (`mvp.md` §2).

## Opções consideradas

| Dimensão | **Desktop + tokens** | Layout de painéis fixo (dashboard) | Temas como folhas CSS separadas |
|---|---|---|---|
| Identidade ("estou no computador da empresa") | Forte | Fraca | Forte |
| Personalização combinável | Sim: tokens se sobrepõem | Não se aplica | Explosão de combinações |
| Custo de um tema novo | Um objeto | — | Uma folha inteira |
| Risco | Janelas sobrepostas escondem conteúdo; a Tabby cobre cliques | Pouco espaço para novos apps | Divergência entre temas |

## Consequências

- **Mais fácil:**
  - um app novo no Chunk 10 (Git, Deploy, Laboratório de carga, Suporte) herda os três temas sem trabalho extra;
  - a CLI e a UI compartilham `internal/game`: um bug de regra de jogo se corrige uma vez;
  - os textos do cenário usam `{{status}}`, `{{check}}` e similares, e cada interface diz o nome da sua ferramenta.
- **Mais difícil:**
  - um contraste precisa ser conferido em cada tema × destaque;
  - o tamanho do texto usa `zoom` só nas janelas de leitura (terminal e gráficos ficam fixos para não desalinhar);
  - o sprite da Tabby é provisório (16×16) até existir arte própria.

## Referências

- Estudo de design (quatro telas, a última com o tema ao vivo): artifact privado "NexoraOS — estudo de design".
- Threat model T1–T5 (implementados no Chunk 9): [`../security/threat-model.md`](../security/threat-model.md).
