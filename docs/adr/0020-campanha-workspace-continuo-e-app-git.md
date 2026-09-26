# ADR-0020: Campanha com workspace contínuo, história entre cenários e app Git

**Status:** Proposto
**Data:** 2026-09-26
**Decisores:** Lucas Zanetti

## Contexto

O Stage 1 tem três cenários jogáveis (C1 → C2 → C3). Até o Chunk 10c, cada um começava num workspace novo, e o código que a pessoa escreveu no C1 não chegava ao C2. O `mvp.md` pede um Stage contínuo: a mesma Nexora crescendo, com os problemas de cada fase nascendo das decisões da anterior.

A pessoa também precisa enxergar o que mudou no próprio código (o que o teste avaliou, o que o próximo cenário herda) sem sair do NexoraOS.

## Decisão

1. **O stage é um manifesto** (`scenarios/stage1/stage.yaml`, `internal/campaign`): a ordem dos cenários e, em cada passo, uma **história** que muda a topologia ("subimos instâncias porque estava lento"; "corte de custo: voltamos a uma instância").
   - A história só pode mudar campos de uma lista fechada (`app.replicas`, `app.dbMaxConns`).
   - Ela é aplicada ao `topology.yaml` da pessoa editando o nó YAML (`topology.SetFile`), sem perder comentários nem o resto do arquivo.
2. **O workspace segue de um cenário para o outro.** Avançar (`game.Advance`, app Missões, `sesim advance`) só é possível depois do PASS no Production Test. O avanço:
   1. commita o workspace ("Fim do cenário …");
   2. desliga a sandbox, porque o mundo do próximo cenário (gateway lento, falhas) é outro;
   3. aplica a história;
   4. commita e marca `sesim/scenario-start/<cenário>`;
   5. reinicia a sessão, guardando o resultado do cenário concluído em `History` (tempo, testes, dicas).
3. **Estado canônico como saída de emergência** (`--canonical`): os arquivos do workspace são trocados pelo início oficial do próximo cenário, o mesmo que o `verify` usa. `.git`, `.sesim` e `.gitignore` ficam, então o código da pessoa continua no histórico. Esse caminho é para quando a solução dela não combina com a história (o `topology.yaml` não aceita a edição) ou quando ela quer comparar com o ponto de partida calibrado.
4. **O cenário vem da sessão.** `--scenario` passa a ser opcional: sem ele, o jogo continua o cenário gravado no workspace. Com um cenário diferente do gravado, o jogo recusa, em vez de misturar os dois.
5. **App Git** (`game.Git*`, app Git): mostra status, diff (desde o último commit ou desde o início do cenário), histórico com as tags de cada cenário, e faz commit. Os limites do app:
   - comandos fixos com `exec`, sem shell;
   - hooks e `core.fsmonitor` desligados, diff sem `ext-diff` e identidade fixa, então um commit não depende do `~/.gitconfig` nem dispara os hooks do workspace;
   - o diff aceita só caminhos que o `git status` listou;
   - um link simbólico novo aparece como o alvo, nunca é seguido;
   - o diff é cortado em 256 KiB.

   O app faz commit de tudo (sem área de staging), e não tem branches nem revert: no jogo, o Git serve para ver a própria mudança e guardar pontos de volta. O terminal de verdade da pessoa continua disponível para o resto.
6. **A sessão em disco é a fonte da verdade entre processos.** O `sesim ui` e um comando da CLI (`test`, `advance`, `stop`) podem estar abertos no mesmo workspace.
   - Antes, cada processo guardava uma cópia da sessão e salvava o arquivo inteiro: um processo desatualizado apagava o progresso do outro. Com a campanha, isso também apagaria o histórico ou desfaria um `advance`.
   - Agora cada processo relê `state.json` quando outro processo o substituiu (`os.SameFile`: o `Save` sempre troca o arquivo por rename). Toda alteração é ler, alterar e salvar.
   - Quando o cenário mudou, o pacote de cenário acompanha. Quando o sandbox que o processo controlava saiu da sessão, ele solta o controle.
   - Não há trava entre processos. Duas alterações no mesmo milissegundo ainda podem se sobrepor, algo que não acontece em ações de uma pessoa. Uma trava de arquivo portável (Windows incluso) pediria promover `golang.org/x/sys` a dependência direta.
   - Um sandbox que outro processo subiu não é assumido: a UI mostra a Nexora desligada até ser aberta de novo.

## Consequências

- **Mais fácil:**
  - o C2 e o C3 avaliam o código que a pessoa escreveu, e não uma solução pronta;
  - o histórico do git conta a campanha: cada início e fim de cenário é um commit;
  - novos stages são um manifesto, sem código novo.
- **Mais difícil:**
  - as referências de calibração partem do início canônico. Uma solução válida do C1 diferente das referências (cache no nginx, por exemplo) pode encontrar outra física no C2. Isso é aceito: os objetivos avaliam propriedades (ADR-0010), e o estado canônico existe para quando a história não encaixa.
  - uma história que precise mudar código, e não só a topologia, exigiria patches sobre o código da pessoa, e patches podem não aplicar. Por isso a história é limitada à topologia.
