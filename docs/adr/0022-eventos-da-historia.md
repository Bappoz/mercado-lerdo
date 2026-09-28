# ADR-0022: Eventos da história declarados no cenário, avaliados na leitura

**Status:** Proposto
**Data:** 2026-09-27
**Decisores:** Lucas Zanetti

## Contexto

O universo do Mercado Lerdo (`docs/product/universe.md` §6) pede que o chefe e o time falem com o jogador durante a missão: pedidos, broncas, emergências com prazo, elogios e notícias. As mensagens precisam:

- sair do conteúdo do cenário, e não do código: são dezenas de missões planejadas;
- reagir ao que o jogador faz, como um teste que reprova, a loja que volta ou um deploy;
- funcionar igual na UI e no terminal, com o jogo parado ou não;
- nunca bloquear a missão. O jogo ensina, e um prazo perdido custa bônus (etapa 4), nunca o PASS.

## Decisão

1. **Declarados no pacote** (`player.events` → `player/events.yaml`, `scenario.StoryEvent`). Cada evento tem:
   - `kind`: pedido, bronca, emergencia, elogio ou noticia;
   - `from`: uma chave do elenco (`scenario.Cast`);
   - um **gatilho**, exatamente um:
     - `at: {phase, after}`: tempo dentro de uma fase;
     - `on: test_failed | test_passed | store_down`: um resultado do jogador ou o momento em que a loja cai;
   - opcionalmente, um **objetivo** de um conjunto fechado (`test_pass`, `store_up`, `deploy`, `view_pulse`, `view_support`), com `within` (prazo) e as respostas `met` e `missed`.

   `ValidateStory` recusa gatilho duplo ou ausente, objetivo desconhecido, prazo sem resposta, emergência sem prazo e elogio ou notícia que peçam algo.

   Os textos aceitam `{{baseline}}`, que vira os números da produção na primeira olhada do dia calmo com tráfego (`Session.Baseline`, gravado por `status` e pelo Pulse), ou some quando não há. Acrescentado depois de um playtest do autor: o Álvaro pedia para guardar os números sem mostrar nenhum.
2. **Avaliado na leitura, sem relógio próprio** (`game.Story`):
   - a UI chama a cada leitura do estado, e a CLI no fim de `next`, `status`, `check`, `test`, `deploy`, `hint` e `support`, e no novo `sesim inbox`;
   - a função pura `evalStory(eventos, sessão, agora, loja no ar?)` dispara o que venceu e fecha objetivos cumpridos ou prazos passados;
   - o resultado fica na sessão (`Story`: `fired_at`, `met_at`, `missed_at`) e vai para o histórico da campanha (`Finished.Story`), de onde a etapa 4 tira os bônus.
3. **Janelas de fase:** um evento `at` dispara no horário certo mesmo que ninguém tenha olhado naquela hora, com a data no passado. Nunca dispara depois de a fase acabar: um pedido do dia calmo não aparece no meio do incidente.
4. **O objetivo conta só depois do pedido:** uma olhada no Pulse antes de o Álvaro pedir não cumpre o pedido. `store_up` exige que a loja tenha caído (`CrashedAt`) e esteja respondendo agora. Achado no teste ponta a ponta: sem isso, a emergência "A loja caiu!" do C3 se cumpria sozinha, porque a queda vem alguns segundos depois de o incidente começar.
5. **Onde aparece:**
   - UI: mensagens na Caixa de entrada com etiqueta do tipo; contagem regressiva na barra do topo para o prazo mais urgente; a Tabby preocupada e apontando onde olhar;
   - CLI: `sesim inbox` e a linha do prazo depois de cada comando;
   - evento de produto `story_event` (disparado, cumprido, perdido) para o Playtest.

## Alternativas descartadas

- **Um relógio em segundo plano no servidor:** exigiria um processo vivo para o jogador só de terminal, e dois relógios quando UI e CLI estão abertas. A avaliação na leitura é determinística e testável como função pura.
- **Eventos com código por cenário:** cada missão nova viraria código. O conjunto fechado de gatilhos e objetivos cobre o arco planejado, e crescer é acrescentar um valor a `Goals` ou `Triggers`.
- **Prazo que reprova a missão:** contradiz o tom do jogo (`universe.md` §3).

## Consequências

- **Mais fácil:** cada missão nova ganha história só com YAML. O resultado de cada evento já está guardado para a carreira.
- **Mais difícil:**
  - `store_up` faz uma requisição a `/healthz` a cada leitura enquanto está pendente;
  - um jogador que nunca abre a UI nem roda comandos vê as mensagens só quando voltar, porém com os horários certos.
- **Limite conhecido:** a pílula "Loja no ar" da barra do topo mostra a sandbox, não a saúde do app. Durante a emergência do C3 ela continua verde ao lado de "Emergência · 04:59".
