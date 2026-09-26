# ADR-0018: Integridade por conciliação, mundo e início do cenário, commit sem fsync na sandbox

**Status:** Proposto
**Data:** 2026-09-25
**Decisores:** Lucas Zanetti

## Contexto

O C2 (Connection Pressure) avalia "zero pedido perdido ou pagamento duplicado" (`mvp.md` §3). O evaluator não confia no app (ADR-0013), então a prova precisa vir de fora dele.

O C2 também começa onde o C1 terminou (com o índice trigram) e depende de algo que o jogador não controla: um gateway de pagamento lento.

Na calibração, a primeira versão da referência que libera a conexão ficou **pior** que o código quebrado. A causa era a espera do fsync do WAL a cada commit, que depende do disco da máquina.

## Decisão

1. **Integridade = conciliação de duas fontes** (`evaluator.Reconcile`):
   - o registro de cobranças do gateway emulado (zona observadora, fora do alcance do jogador);
   - os pedidos `paid` no banco, lidos como superusuário.

   Cada `order_ref` do gerador de carga carrega o id da execução. A medida `payment_mismatches` soma:
   - pedidos perdidos (cobrado, sem pedido pago);
   - pedidos fantasmas (pago, sem cobrança);
   - cobranças duplicadas;
   - pedidos duplicados.

   A conciliação espera `IntegritySettle` (12 s, acima do timeout de pagamento do app), porque um checkout que o cliente abandonou pode ainda estar rodando no servidor.

   O banco é o registro do sistema e pode ser lido. Um app que forjasse pedidos pagos seria pego pelo gateway (fantasmas).
2. **O cenário declara o mundo e o início.**
   - `world.paymentLatency` ("mediana,p99"): o gateway responde com latência lognormal, derivada de forma determinística do `order_ref`, para que uma execução repetida veja os mesmos atrasos. O C1 não declara latência e continua instantâneo, com a calibração intacta.
   - `start`: patches aplicados ao workspace-base, em ordem, antes da tag de início do cenário. As referências são patches sobre esse início. O `verify` e o `sesim play` usam o mesmo início.
3. **`synchronous_commit=off` no Postgres da sandbox.** Um commit não espera o fsync do WAL. A sandbox é descartável, e a latência de fsync é do disco do host (SSD, HDD, Docker Desktop): ela não pode decidir um veredito. Com a espera ligada, a solução correta do C2 (duas transações curtas) perdia para o código quebrado nesta máquina, por esperar dois fsyncs por checkout.
4. **Contagens de eventos raros são julgadas pelo total do teste.**
   - `payment_mismatches` é somado sobre as execuções e comparado uma única vez: a propriedade é "nenhum pedido perdido no Production Test".
   - Julgar execução por execução com a regra de margem (PASS só se todas passam) transformava uma execução com 0 perdidos, entre outras com 5 e 6, em INCONCLUSIVE: medido na calibração do C2.
   - As medidas contínuas (latência, erros, CPU) continuam com a regra por execução.
5. **O catálogo limita conexões por tamanho de Postgres** (S 50, M 150, L 400). Mais conexões exigem mais memória, e portanto um banco maior e mais caro. Sem isso, "aumente o `max_connections`" seria uma solução de graça.
6. **Suporte.** O app de reclamações deriva os tickets dos mesmos sinais:
   - erros e p95 do tráfego do dia (Prometheus);
   - conciliação do tráfego de fundo *atual*, porque um Production Test recria o banco, mas o gateway guarda tudo.

   Um sistema saudável não tem tickets. Nada é roteirizado por cenário.

## Consequências

- **Mais fácil:**
  - integridade vale para o C3 também (cobrança duplicada em retry);
  - cenários encadeados reaproveitam o início do anterior.
- **Mais difícil:**
  - o evaluator executa SQL no banco do jogador;
  - o gateway guarda cobranças em memória durante toda a vida da sandbox.
- **Recalibrar:** mudar `synchronous_commit` muda a física de todos os cenários. O C1 foi reverificado depois da mudança (ver PR do Chunk 10b).
