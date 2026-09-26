# ADR-0019: Injeção de falhas por crash, clientes que tentam de novo e laboratório de carga

**Status:** Proposto
**Data:** 2026-09-26
**Decisores:** Lucas Zanetti

## Contexto

O C3 (Single Instance Failure) pede que o evaluator derrube uma instância do app no meio do teste e distinga **recuperação** (voltar rápido) de **redundância** (não parar), e que um retry não cobre duas vezes (`mvp.md` §3). O jogador também pediu um lugar para testar carga e falhas antes de publicar.

## Decisão

1. **Falha = crash do processo, via `SIGABRT`** (`stack.Crash`: `docker exec … kill -ABRT 1`).
   - `docker kill` conta como parada manual, e o Docker **não** aplica a política de reinício depois dela (medido: `restarts=0` com `on-failure`). Com isso, a solução "reinício automático" seria impossível de testar.
   - O runtime do Go aborta com `SIGABRT` mesmo como PID 1, e o runtime do container vê um processo que morreu sozinho, como num panic ou num OOM.
2. **Falhas declaradas no cenário:**
   - `faults: [{after, crash}]` roda em toda execução do Production Test, com o tempo contado a partir do início da carga.
   - `player.growth.crash` derruba a instância quando o incidente começa, no jogo.
   - Depois das execuções, o evaluator restaura a sandbox antes do oráculo: corretude é propriedade do código, e a disponibilidade já foi medida.
3. **Réplica do app caída faz parte do jogo.** A sandbox só é considerada morta se algo além das réplicas do app parou; senão, o próximo comando recriaria a sandbox e apagaria o incidente.
4. **`app.autoRestart`** na topologia (política `on-failure`). Uma parada feita pela plataforma continua parada.
5. **Clientes que tentam de novo** (perfil `crash-day`):
   - um checkout que falha é reenviado um segundo depois, com o mesmo `order_ref` e a mesma `Idempotency-Key`, como faz uma pessoa;
   - o retry é opcional no `traffic.js` e só esse perfil o liga, então a calibração do C1 e do C2 não muda.
6. **O gateway emulado honra `Idempotency-Key`**, como os reais: a primeira requisição com a chave cobra; as seguintes, inclusive uma concorrente, recebem a cobrança original com `200`. Documentado em `nexora/API.md`. O bug plantado continua no app, que não envia a chave e trata cada checkout como pedido novo.
7. **Laboratório de carga** (`game.Lab`, app Lab, `sesim lab`):
   - carga escolhida pelo jogador contra a produção atual, sem reset e sem nota;
   - pode derrubar a instância 1 no meio;
   - mostra os números por página e a conciliação de pagamentos daquele experimento;
   - perfis e limites fechados: taxa de 1 a 150, duração de 20 a 120 s.

## Consequências

- **Mais fácil:** qualquer cenário futuro declara falhas sem código novo. O laboratório dá ao jogador o mesmo instrumento do teste, sem a nota.
- **Mais difícil:**
  - o crash depende de o app ser um processo Go (`SIGABRT`);
  - outras linguagens de app precisariam de outro sinal.
- **Calibração:** um app Go reinicia em menos de 1 s. Com uma queda por execução, "reinício automático" ficava a 0,67 % de erro contra um limiar de 0,5 %, perto demais para ser estável. O C3 derruba a instância duas vezes por execução (crash loop).
