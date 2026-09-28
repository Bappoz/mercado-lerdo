# ADR-0025: Menu de título com até três saves e uma abertura da empresa

**Status:** Proposto
**Data:** 2026-09-28
**Decisores:** Lucas Zanetti

## Contexto

O LerdOS abria direto na mesa de trabalho de um workspace só (`sesim ui --workspace`). Recomeçar pedia apagar a pasta à mão (`rm -rf workspace`), e o jogador não via a história da empresa antes do primeiro e-mail. O autor pediu:
- um menu divertido, onde se continua uma partida ou se começa outra, com no máximo três;
- uma abertura (cutscene) contando a história do Mercado Lerdo;
- a logo do jogo, de forma sutil.

## Decisão

1. **Três saves, três workspaces** (`ui.SaveSlots`): o `--workspace` de sempre (padrão `workspace`) é o save 1; os outros são `<workspace>-2` e `<workspace>-3`, ao lado dele.
   - A partida de quem já jogava vira o save 1 sem migração nenhuma.
   - A CLI joga qualquer save com `--workspace workspace-2`.
2. **O menu lê o save sem abrir o jogo** (`game.Summarize`): só o arquivo de sessão. Ele mostra:
   - cenário e fase;
   - missões concluídas;
   - nível e saldo (o mesmo cálculo do perfil: `profileOf`, ADR-0023);
   - quando foi jogado.
3. **Um save por vez no servidor.** O `ui.Server` guarda o jogo atual atrás de um lock e troca de jogo ao escolher outro save (`POST /api/saves/select`). Ao trocar:
   - a loja do save anterior é desligada (a sandbox destruída; o código e o progresso ficam). Um notebook não roda duas lojas com folga;
   - trocar enquanto um job roda é recusado.
4. **Nova partida** (`POST /api/saves/new`) escolhe o primeiro save vazio; com os três ocupados, responde 409 e o menu desabilita o botão ("apague um para começar outro").
5. **Apagar** (`POST /api/saves/delete`) pede dois cliques no menu. O servidor:
   - desliga a loja daquele save e remove a pasta;
   - só apaga uma pasta que tenha o `.sesim/` do jogo. O caminho vem do número do save, calculado no servidor, nunca do navegador.
6. **O fluxo da UI:**
   - o LerdOS sempre abre no menu;
   - "Continuar" abre o save jogado por último;
   - um save vazio passa pela abertura antes da mesa (e o treinamento do primeiro dia vem depois, como antes);
   - "Ver a abertura" toca a cena e volta ao menu;
   - a logo na barra do topo volta ao menu.
7. **A abertura** tem 8 cenas, uma frase cada, conforme `docs/product/universe.md` §1:
   - a Dona Lurdes e o crochê;
   - o Álvaro vendendo pela internet;
   - as taxas dos grandes;
   - o apelido Lerdo;
   - o Mercado Lerdo;
   - o site do Kleber;
   - o coworking e a Tabby;
   - "você".

   A arte é pixel art em 16×16 nas cores da logo, mais a própria logo. Enter, clique e seta avançam; Esc pula. Com as animações desligadas, o texto aparece inteiro e parado.
8. **A logo:**
   - a marca (a preguiça, sem o nome) é o favicon e o botão da barra do topo;
   - a logo inteira fica no menu, na abertura e no topo do README.

   Os arquivos ficam em `web/public/` (embutidos no binário) e em `docs/assets/`.

## Consequências

- **Mais fácil:** recomeçar e comparar abordagens em saves diferentes, sem mexer em pastas; o jogador conhece a empresa antes do primeiro e-mail.
- **Mais difícil:**
  - o servidor passou a ter um jogo trocável; cada handler lê o jogo atual (`s.gm()`) em vez de guardar um;
  - as preferências (tema, e-mails lidos, treinamento visto) continuam globais, não por save.
- **Limites conhecidos:**
  - apagar o save aberto o deixa selecionado e vazio;
  - uma requisição em andamento no momento da troca ainda fala com o jogo anterior.
