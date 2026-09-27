# ADR-0021: A velocidade da máquina entra no veredito

**Status:** Proposto
**Data:** 2026-09-26
**Decisores:** Lucas Zanetti

## Contexto

As cotas da sandbox são frações de CPU (o Postgres tem 0,5 CPU). Quanto trabalho cabe nessa fração depende da velocidade de cada núcleo do host.

Em 2026-09-26, o notebook de referência estava na bateria com o perfil `power-saver`, e os núcleos ficaram em ~1 GHz (o máximo é 4,7 GHz). Nessas condições, a referência trigram do C1 deu:

- busca p95 de 230 a 1400 ms e CPU do banco de 0,77 a 1,0;
- FAIL/INCONCLUSIVE tanto no `verify` quanto dentro do jogo.

Na calibração, na tomada, a mesma referência deu 20–75 ms e 0,5–0,6.

A H5 (`mvp.md` §6) pede vereditos estáveis em 3 máquinas, e o Playtest #2 roda nas máquinas das pessoas. Um veredito que inverte por causa da energia do notebook ensina a coisa errada: a pessoa conclui que a solução está errada.

## Decisão

1. **Sonda de velocidade dentro da sandbox** (`stack.DBProbe`, `evaluator.Prober`):
   - antes de cada execução do Production Test, com o banco recém-restaurado e sem carga, o evaluator cronometra uma consulta fixa que só usa CPU (`generate_series` de 1 milhão de linhas com filtro, sem paralelismo);
   - a consulta roda no próprio Postgres da sandbox, sob a cota do jogador, depois de um `CHECKPOINT` síncrono;
   - roda 5 vezes, e vale a **mais rápida**: ruído só soma tempo. A primeira versão usava a mediana de 3, sem checkpoint, e no jogo media 600–900 ms numa máquina de ~300 ms. O reset acabou de clonar o banco e rodar as migrações, e o checkpointer disputava a mesma cota. A CPU do banco durante a carga (0,54) mostrava que a máquina estava na velocidade de referência;
   - o tempo é o `Execution Time` do `EXPLAIN ANALYZE`, sem a partida do `psql`;
   - a sonda mede o que decide o veredito (CPU efetiva do banco sob a cota), inclusive a VM do Docker Desktop se um dia houver suporte a ele.
2. **Piso e fator de lentidão:**
   - o evaluator compara a **mediana** das sondas das execuções com uma referência (`DefaultProbeReference`) e com um piso (`DefaultProbeLimit`);
   - usa a mediana porque uma sonda isolada lenta é ruído: na calibração, uma execução na tomada mediu 842 ms entre outras de ~300 ms;
   - acima do piso, cada FAIL ou INCONCLUSIVE em medida sensível a velocidade (`*_ms`, `db_cpu_*`, `error_rate`) é julgado de novo, com o limiar multiplicado pelo fator de lentidão *k* = sonda ÷ referência;
   - se ainda reprova com esse desconto, continua **FAIL**, com o motivo "reprova mesmo descontando a máquina": código quebrado (busca a 5000 ms) não vira INCONCLUSIVE em máquina lenta;
   - se o desconto explica o erro, vira **INCONCLUSIVE**, com a máquina como motivo (`host.excused`). Só então o aviso de máquina lenta aparece para o jogador: um PASS numa máquina lenta não precisa de aviso;
   - um **PASS continua PASS**, porque máquina mais lenta só piora essas medidas. Corretude, integridade e custo não mudam.

   É o mesmo padrão do gerador limitado (`GeneratorLimited`): uma medida em que não dá para confiar não reprova ninguém.
3. **Aviso antecipado** (`internal/power`): no Linux, se o computador estiver na bateria ou em perfil de economia (sysfs), o Production Test e o `scenario verify` avisam antes de começar. Em outros sistemas, o pacote não sabe e não diz nada. A sonda continua sendo a verificação que vale.
4. **Registro:**
   - a sonda de cada execução vai para o relatório (`runs[].db_probe_ms`, `host`), para a tabela da CLI e para o app Testes;
   - o evento `production_test_run` recebe `db_probe_ms` e `slow_host`. É um número, sem identidade, e serve para a H5 nas máquinas do Playtest #2.

## Calibração

As três referências mais perto de um limiar de velocidade foram escolhidas pelas calibrações anteriores (pior execução ÷ limite):

- C1 `trigram-index`, `db_cpu_p90`: 0,87;
- C3 `redundancy-only`, `error_rate`: 0,82;
- C2 `sized-pools-no-cancel`, `reads_p95_ms`: 0,80.

As três rodaram uma rodada (3 execuções) em cada estado do notebook de referência, em 2026-09-26:

| Estado | Sondas (ms) | Vereditos das três referências |
|---|---|---|
| Tomada, perfil `balanced`, sonda antiga | 305, 304, 318 · 313, 517, 299 · 842, 296, 314 | todos os esperados; trigram com `db_cpu_p90` 0,50–0,53 e busca p95 18–25 ms |
| Tomada, perfil `balanced`, sonda final | 300, 233, 235 · 305, 294, 290 · 267, 243, 290 | todos os esperados; trigram com `db_cpu_p90` 0,54–0,58 |
| Tomada, perfil `power-saver` | 1599, 1501, 1672 · 1917, 1429, 2177 · 1232, 1923, 1593 | trigram e C2 INCONCLUSIVE em velocidade; nenhum FAIL falso; a integridade do C3 continuou FAIL, como esperado |
| Bateria, `power-saver` (~0,7–1 GHz) | 1133, 1426, 1287 | trigram INCONCLUSIVE em latência (311–370 ms) e recursos (0,85–0,93) |
| Transição (carregador ligado no meio) | 1128, 300, 413 · 396, 297, 286 | C2 e C3 como esperado |

- **Referência:** 267 ms. É a mediana das sondas das referências na tomada, com a sonda final (checkpoint + mais rápida de 5): 235, 294 e 267 ms, cada uma a mediana de 3 execuções. Com a sonda antiga (mediana de 3, sem checkpoint), a referência era 305 ms.
- **Piso:** 325 ms, **derivado dos dados** (substitui a estimativa inicial de 400 ms, que supunha que a CPU cresce na proporção da lentidão).
  - Para cada referência que passa na tomada e cada medida de velocidade, o evaluator calcula a menor lentidão *k* que faria até a pior execução medida errar o limite pela margem: *k* = limite × (1 + margem) ÷ pior valor (`evaluator.SlowdownTolerance`).
  - O menor *k* do histórico de calibração na tomada é **1,34**: C3 `redundancy-only`, `error_rate` pior em 0,0041 contra 0,005 × 1,1. O segundo é o trigram do C1, com 1,44.
  - Desconta-se o ruído da própria sonda: as medianas na tomada variaram de 235 a 294 ms, ±10 % em torno de 267, porque o turbo do perfil `balanced` varia a frequência entre 2,0 e 2,6 GHz. 267 × 1,34 ÷ 1,10 ≈ **325 ms**.
  - O `sesim scenario verify` imprime a tolerância e o piso recomendado a cada calibração, então o piso não fica velho quando um cenário muda.
  - Continua a suposição de que a medida piora na proporção da lentidão. Ela é mais fraca para `error_rate`, justamente a medida que define o piso.
- **Ponto intermediário:** o perfil `power-saver` **na tomada** deu sondas de 1500–1670 ms, tão lento quanto na bateria. Não existe um estado de energia intermediário nesta máquina, e nenhuma medição caiu entre 413 e 1128 ms. O piso foi derivado da tolerância das referências, e não de um estado intermediário.
- Um Postgres isolado com a mesma imagem e cota mediu ~690–870 ms em `power-saver` (média de ~1,1 GHz), e a sandbox mediu 1133–1426 ms (média de ~0,7 GHz). As duas medições não isolam a causa da diferença: frequência ou disputa com o resto da sandbox. Por isso a referência e o piso foram medidos na sandbox, que é onde o veredito acontece.

## Alternativas descartadas

- **Escalar a cota pela velocidade medida** (dar mais CPU a uma máquina lenta). Tornaria a física parecida entre máquinas, mas:
  - não corrige a latência de uma única consulta, que depende do núcleo e não da cota;
  - uma máquina lenta pode não ter os núcleos extras;
  - mexe na física de todos os cenários, e cada um teria de ser recalibrado.

  Fica como opção se o Playtest #2 mostrar muitas máquinas abaixo do piso.
- **Micro-benchmark no host** (fora do Docker). Não enxerga a cota nem a VM do Docker Desktop. Além disso, um benchmark que usa instruções especializadas (SHA, AES) mede o conjunto de instruções, não a velocidade.
- **Recusar o teste numa máquina lenta.** Impediria um PASS legítimo e daria à pessoa um bloqueio em vez de informação.

## Consequências

- **Mais fácil:** o veredito não reprova por causa da energia do notebook, e o Playtest #2 mede a velocidade de cada máquina.
- **Mais difícil:**
  - numa máquina abaixo do piso, uma solução que erra por pouco fica INCONCLUSIVE, e não FAIL. A pessoa recebe "máquina lenta" em vez de "a solução não passa". É o preço de não mentir sobre a solução.
  - o desconto supõe que a medida piora na proporção da lentidão. Isso é aproximado para latência e CPU, e mais fraco para `error_rate`, que salta quando o sistema satura.
  - a sonda soma 3 consultas por execução: ~0,3 s cada na tomada, ~1,3 s em `power-saver`.
- **Limite conhecido:** é um piso, não um teto. Uma máquina muito mais rápida que a de referência pode fazer uma solução quebrada passar em velocidade. Não há dado dessa direção ainda.
