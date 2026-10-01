# ADR-0026: Leituras recomendadas, relógio da história pausável e passos do treinamento revisíveis

**Status:** Proposto
**Data:** 2026-10-01
**Decisores:** Lucas Zanetti

## Contexto

Jogando o C1, o autor anotou quatro problemas:
- escreveu a migração antes do passo que a pede. O passo fechou no mesmo instante em que começou (`add_migration` conta desde o início do cenário, ADR-0024), e o texto com o código nunca apareceu;
- o balão da Tabby sumia ao clicar fora dele, justamente quando o jogador ia digitar no Terminal o que ela tinha acabado de dizer;
- faltava apontar onde estudar o assunto de cada missão. Quem para para ler perde o prazo da história (ADR-0022);
- os papéis de parede eram três, e nenhum tinha a marca.

## Decisão

1. **Passos que ficaram para trás podem ser relidos.** `TutorialView.Past` lista, em ordem, cada passo fechado ou pulado, com o texto e a fala de conclusão.
   - **UI:** o balão ganha "‹ Passo anterior", e a navegação "Anterior / Próximo / Voltar para agora".
   - **CLI:** `sesim tabby passo` lista os passos; `sesim tabby passo <n>` mostra um.
   - Reler não muda o progresso: é só leitura do que ela já disse.
2. **O balão só fecha pela Tabby.** Clicar nela esconde e mostra o balão; "Ok" também esconde. Clicar numa janela não fecha mais.
3. **Leituras declaradas no pacote** (`player.reading` → `player/reading.yaml`, `scenario.Reading`): título, quem publica, link, idioma, tempo estimado e o porquê, na voz da Tabby.
   - `ValidateReading` recusa id repetido, campos vazios, link que não seja `https`, idioma fora de `pt`/`en` e tempo menor que 1 minuto.
   - **A leitura nomeia um assunto, nunca o diagnóstico.** O diagnóstico continua na escada de dicas, que custa salário (ADR-0023). O limite é o do briefing: a leitura pode aprofundar o que o e-mail de boas-vindas já cita, não o que só a dica revela.
   - São links para a documentação de verdade (PostgreSQL, Go, nginx, Google SRE Book…): ler a fonte é parte do ofício. O jogo não hospeda o texto.
4. **Onde aparecem:**
   - **UI:** o app Docs ganha a seção "Leituras da missão", antes do catálogo. A Tabby cita as leituras na fala do dia calmo e mostra o botão "Leituras da missão" no balão durante a fase calma. No C1, o passo "começar o expediente" aponta para elas (`{{reading}}`).
   - **CLI:** `sesim reading`.
   - **Evento de produto:** `reading_opened` (id, relógio parado ou não).
5. **O relógio da história pode parar** (`Session.Pauses`, `game.PauseClock`/`ResumeClock`). Uma pausa é um trecho de tempo que a história não conta:
   - os eventos por tempo (`at: {phase, after}`) e os prazos (`within`) contam só o tempo sem pausa (`clockAdd`). Com o relógio parado, nada dispara e nenhum prazo vence;
   - **só a história espera.** A loja continua recebendo o tráfego do dia: a produção é de verdade e não tem pausa;
   - o que acontece em produção retoma o relógio sozinho: começar o expediente, deploy, Quick Check, Production Test e laboratório;
   - **UI:** botão na página de cada leitura; com o relógio parado, uma pílula no topo ("Relógio parado · retomar") e a contagem regressiva congelada. **CLI:** `sesim clock [pause | resume]`;
   - **eventos de produto:** `clock_paused` (fase) e `clock_resumed` (segundos parado).
6. **Papéis de parede** (`web/src/wallpaper.ts`): além da cena do tema, do liso e da grade, sete no estilo de um Windows antigo, com a marca e as cores da logo: logo ao centro, lado a lado, nuvens, colina, instalação, crochê e pied-de-poule.
   - Cada um é uma pilha de fundos CSS; os padrões são SVG gerado no cliente (`img-src data:` já era permitido pela CSP), sem arquivo novo no binário.
   - A miniatura em Configurações é a própria mesa, quatro vezes menor.

## Consequências

- **Mais fácil:**
  - quem se adianta no treinamento não perde o que a Tabby ia dizer;
  - dá para seguir uma instrução da Tabby com o balão aberto;
  - o jogador tem por onde estudar, sem pagar com o prazo;
  - outro cenário ganha leituras só com um `reading.yaml`.
- **Mais difícil:**
  - a história deixou de contar o relógio de parede: todo cálculo de tempo dela passa por `clockAdd`;
  - os links são de terceiros e podem mudar. Não há teste que os abra (os testes não usam rede).
- **Limites conhecidos:**
  - a pausa não tem teto: dá para parar o relógio e investigar em vez de ler. O prazo perdido só custa um bônus (ADR-0022), e o Playtest #2 registra as pausas para mostrar se isso acontece;
  - as leituras dão, de graça, parte do que a dica de nível "Conceito" manda pesquisar. A H1 (dicas até o PASS) passa a ser lida junto com `reading_opened`;
  - o balão aberto cobre o canto da tela até o jogador escondê-lo;
  - o tempo de cada leitura é uma estimativa do autor, não uma medida.
