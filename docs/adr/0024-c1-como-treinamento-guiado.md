# ADR-0024: O C1 é um treinamento guiado pela Tabby, com duas perguntas para pensar

**Status:** Proposto
**Data:** 2026-09-27
**Decisores:** Lucas Zanetti

## Contexto

No playtest do autor, o C1 mostrou que a primeira missão ensina coisa demais ao mesmo tempo:
- o LerdOS;
- o terminal (que abre o `bash`, não o `psql`);
- as estatísticas do Postgres, o plano de execução e as migrações;
- a diferença entre o Quick Check e o Production Test.

Com a escada de dicas, quem trava no primeiro dia paga (do salário) para descobrir coisas que são do jogo, não do problema. A decisão do autor foi: o C1 vira um tutorial. A Tabby ajuda de graça do começo ao fim, e só em alguns pontos deixa o jogador pensar.

## Decisão

1. **Declarado no pacote** (`player.tutorial` → `player/tutorial.yaml`, `scenario.TutorialStep`). Cada passo tem:
   - fase: `calm` ou `incident`;
   - tipo: `guia` (diz o que fazer) ou `pergunta` (pergunta, e só explica se o jogador pedir ajuda);
   - o texto;
   - um objetivo de um conjunto fechado;
   - a fala de quando o passo termina.

   Os objetivos:
   - abrir uma tela: `view_pulse`, `view_support`, `open_shell`;
   - consultas do próprio jogador no `psql`: `query_stats` (consultou `pg_stat_statements`) e `explain`;
   - resultados: `add_migration`, `quick_check`, `deploy`, `next`, `test_pass`;
   - respostas à Tabby: `continue` e `answer` (escolha entre alternativas, com uma resposta certa e uma fala para a errada).

   `ValidateTutorial` recusa:
   - passo calmo depois de passo do incidente;
   - pergunta sem ajuda;
   - `answer` sem duas alternativas ou sem a certa;
   - alternativas em outro objetivo.
2. **Avaliado na leitura, como a história** (ADR-0022). A função pura `evalTutorial(passos, sessão, contadores, agora)`:
   - começa o passo atual e o fecha quando o objetivo foi cumprido;
   - começa o seguinte na mesma chamada;
   - descarta os passos calmos que o incidente atropelou (quem roda `next` cedo não fica preso).

   O progresso fica na sessão (`Tutorial`: início, fim, contador-base, respostas, ajuda pedida).
3. **"Rode agora", não "rodou um dia"**: `query_stats`, `explain` e `quick_check` são contadores. O passo só fecha quando o contador cresce depois de começar. O banco recriado (Production Test, loja religada) zera o `pg_stat_statements`, e o contador-base desce junto.
   - `add_migration` conta desde o início do cenário (tag `sesim/scenario-start/…`), então escrever a migração antes do passo vale.
   - **Consultas do jogador:** as que o papel `nexora` rodou no `pg_stat_statements`. O app nunca consulta a visão nem roda `EXPLAIN`; o exporter (`sesim_observer`) e a plataforma (`postgres`) têm papéis próprios.
   - **Sem sondar durante um teste:** a UI não pergunta ao banco enquanto um job roda, para não somar ruído a um Production Test (ADR-0021).
4. **A ajuda do treinamento é de graça** (ADR-0023):
   - num cenário com treinamento, toda dica custa R$ 0;
   - usar dica não tira o bônus "sem dicas";
   - a escada de dicas continua existindo, mas o balão da Tabby a esconde enquanto conduz: a ajuda do passo faz esse papel.
5. **O C1 tem 12 passos**:
   - no dia calmo: conhecer o normal no Pulse, abrir o terminal, a primeira consulta, ler o código da busca, começar o expediente;
   - no incidente: quem está no teto de CPU (pergunta), qual consulta pesa, o plano com `Seq Scan`, por que um índice comum não serve (pergunta), a migração com `pg_trgm`, o Quick Check e o Production Test.

   O texto de cada passo nomeia as ferramentas por placeholder, como todo texto de cenário.
6. **Onde aparece**:
   - **UI:** o passo ocupa o balão da Tabby antes dos prazos da história (a contagem regressiva continua no topo), com "Continuar", "Me ajuda" e as alternativas como botões. Um job que acabou de falhar ainda fala primeiro.
   - **CLI:** o passo novo aparece no fim de qualquer comando, e `sesim tabby [ok | ajuda | responder <letra>]` mostra o passo e responde a ela.
   - **Eventos de produto:** `tutorial_step` (começou, terminou, pulado), `tutorial_help` e `tutorial_answer` (certa ou errada), para o Playtest #2.

## Consequências

- **Mais fácil:**
  - o primeiro contato ensina o jogo e o método (medir, achar a consulta, entender o plano, mudar pelo workspace, provar com o teste) sem custo e sem travar;
  - outros cenários podem ganhar treinamento só com um `tutorial.yaml`.
- **Mais difícil:**
  - o C1 deixa de medir se o jogador descobre sozinho. A H1 (dicas até o PASS) passa a ser medida a partir do C2, e o Playtest #2 precisa registrar isso;
  - os textos do treinamento dão a solução do C1: o debrief continua sendo onde as alternativas aparecem.
- **Limites conhecidos:**
  - a UI registra "viu o Pulse" ao abrir o app. Por isso, o passo do incidente que depende do Pulse é uma pergunta, não um "abra o Pulse";
  - as consultas contam por papel, não por sessão do terminal: um `EXPLAIN` rodado por outro cliente com o papel `nexora` contaria. Na sandbox, só o terminal do jogador usa esse papel para isso.
