# Definição do MVP

## 1. Hipótese que o MVP existe para testar

> H0: Receber um sistema real, vê-lo degradar sob carga, modificá-lo e provar a correção com um teste objetivo e multidimensional é **engajante** (a pessoa termina e quer o próximo) e **educativo** (a pessoa explica *por que* a correção funcionou e cita uma alternativa).

Tudo no MVP serve a H0. O que não serve é non-goal.

## 2. Escopo

### Dentro

| Área | Forma no MVP |
|---|---|
| Distribuição | **Local-first** (ADR-0002). Um binário `sesim` (nome provisório) sobe a sandbox no Docker do jogador e serve a UI em `127.0.0.1`. |
| App demo | Marketplace "Nexora" em **Go** (ADR-0004): auth, produtos, busca, pedidos/checkout. PostgreSQL e Nginx. Dataset seed determinístico (~100k produtos gerados a partir de uma seed). |
| Workspace | Repositório git no disco com o código do app e o `topology.yaml`. O jogador usa o próprio editor. |
| Topologia | Arquivo declarativo e validado que descreve componentes de um catálogo pequeno (nginx, app ×N, postgres, pgbouncer, valkey) e seus limites de recursos. É a fonte da verdade para a sandbox, a tela de arquitetura e o modelo de custo. |
| Terminal | Terminal web (xterm.js) num container *toolbox* da rede da sandbox, com psql, curl, ss, top e afins. Também dá para usar localmente com `sesim shell`. |
| Carga | Perfis de workload (Normal Day, Flash Sale) como scripts k6 versionados, em modelo aberto (arrival rate). |
| Telemetria | Métricas de requisição medidas **no gerador de carga e no Nginx** (fora do app), mais métricas de containers e do Postgres. |
| Evaluator | Objetivos declarativos por cenário. Resultado por dimensão: PASS, FAIL ou INCONCLUSIVE, com valor medido e dispersão. Inclui um oráculo de corretude funcional. |
| Cenários | 3 (abaixo), encadeados como Stage 1. Reset para o início do cenário; opção de adotar o estado canônico. |
| Dicas | Escada autorada de 5 níveis por cenário. |
| Debrief | Antes/depois, gráfico das soluções de referência, notas de conceito. |
| Analytics de produto | Log local de eventos (JSONL), exportação opt-in. Nenhuma telemetria remota por padrão. |
| Docs | Produto, arquitetura, threat model, ADRs, setup local. |

### Non-goals (MVP)

- Sandbox hospedada multi-tenant, contas, autenticação da plataforma.
- Editor de código web.
- Kubernetes, Kafka, multi-região, service mesh.
- Mentores de IA e correção de postmortem por IA.
- *Avaliação* de postmortem ou ADR (templates podem existir; nota não).
- UI do Chaos Lab, modo Incidents, modo Labs.
- Leaderboards, multiplayer, social, marketplace, certificados, cenários da comunidade.
- Economia além de uma tabela estática de preço por tamanho de componente.
- Mais de uma linguagem para o app demo.
- Suporte nativo a Windows (WSL2 + Docker Desktop fica como best-effort).
- Qualquer fluxo de pagamento. A doação é um link externo.

## 3. Os três desafios (refinados)

Regra comum aos três: **o evaluator verifica resultados; o briefing nunca nomeia a correção.**

### C1 — Slow Search

- **Montagem:** `GET /search?q=` usa `ILIKE '%termo%'` sobre `products.name`, sem índice adequado, com `ORDER BY created_at` sem índice. Evento de crescimento ×40.
- **Sintomas:** p95 > 2 s, CPU do banco quase saturada, CPU do app baixa.
- **Soluções de referência** (implementadas como patches em `scenarios/stage1/c1-slow-search/references/`):
  - (a) índice GIN trigram (`pg_trgm`): **PASS**;
  - (b) índice de busca em memória no app: **PASS** por outro caminho. Tira a carga do banco ao custo de memória por instância e de resultados desatualizados em até 30 s;
  - (c) busca por prefixo + B-tree: rápida, mas **falha em corretude**, porque o requisito é "em qualquer posição do nome";
  - (d) full-text search (`tsvector`): rápida, mas **falha em corretude**, porque casa palavras e não substrings (`ette` não acha "Kettle"). *Mudança em relação ao Chunk 1, que a listava como válida.*
  - Um cache ingênuo na frente ficou fora das referências por enquanto. Com a cauda longa dos termos, a expectativa é melhora parcial, mas isso ainda não foi medido.
- **Objetivos** (`scenario.yaml`): corretude (9 casos de oráculo, esperado regenerado da seed), p95 da busca < 300 ms, taxa de erro < 1 %, p90 da CPU do banco < 80 % da quota (era 70 %; mudado pela H5). Margem de 10 % nos três últimos. Production Test: Flash Sale até 60 req/s, 3 execuções.

### C2 — Connection Pressure

- **Montagem:** depois do C1, o tráfego cresce de novo e o app ganha mais instâncias (história: "subimos mais instâncias porque estava lento"). Cada instância tem um `pgxpool` grande demais, e a soma passa de `max_connections`. O checkout segura a conexão durante uma chamada lenta a um pagamento externo emulado.
- **Sintomas:** 5xx intermitentes (`too many clients`) e picos de latência com CPU do banco *moderada*. É o enigma "CPU baixa mas lento".
- **Soluções válidas:**
  - dimensionar os pools de acordo com `max_connections`;
  - PgBouncer em transaction pooling;
  - liberar a conexão antes da chamada externa (a correção mais didática);
  - aumentar `max_connections` e a memória. Funciona, mas pesa na dimensão de custo.
- **Objetivos:** taxa de erro < 0,1 %, p95 < 400 ms, **zero pedido perdido ou pagamento duplicado** (verificado contra registros do gerador de carga e do mock de pagamento, ambos fora do controle do jogador), custo ≤ orçamento.

### C3 — Single Instance Failure

- **Montagem:** o injetor de falhas mata a instância do app no meio do teste.
- **Sintomas:** 100 % de erro até alguém reiniciar manualmente.
- **Soluções válidas:**
  - ≥2 instâncias atrás do upstream do Nginx com `proxy_next_upstream`/health check;
  - restart policy + startup rápido: *pode* passar num alvo frouxo, mas falha no estrito. Ensina a diferença entre *recuperação* e *redundância*.

  Em qualquer caminho, o checkout precisa ser idempotente para não duplicar pagamento no retry.
- **Objetivos:** disponibilidade ≥ 99,5 % num teste com um kill de instância, zero pagamento duplicado, custo ≤ orçamento.

Cada desafio tem uma solução *tecnicamente atraente mas errada* diante das restrições. É o principal recurso didático.

## 4. Escada de dicas (autorada, por cenário)

| Nível | Tipo | Exemplo (C2) |
|---|---|---|
| 1 | Observação | "Observe as conexões do banco em relação a `max_connections` durante o Quick Check." |
| 2 | Direção | "Compare o tempo executando queries com o tempo esperando para obter uma conexão." |
| 3 | Conceito | "Pesquise connection pooling e o tempo de vida de conexões dentro de handlers." |
| 4 | Explicação | Explicação completa do dimensionamento de pool e de segurar conexão durante I/O. |
| 5 | Implementação | Aponta o local no código e esboça a mudança. |

## 5. Modelo inicial de sandbox

Detalhado em `docs/architecture/sandbox.md`. Resumo: containers no Docker local, duas redes `internal` (zona do jogador e zona observadora), limites de CPU, memória e pids em todos os containers, nada privilegiado, sem socket Docker dentro da sandbox, egress bloqueado, medição feita pela zona observadora.

## 6. Hipóteses a validar

| # | Hipótese | Teste | Sinal |
|---|---|---|---|
| H1 | A persona completa o C1 numa sessão de 30–90 min usando dicas até o nível 3. | Playtest com 5–8 pessoas (think-aloud) | conclusão, tempo, nível de dica |
| H2 | O briefing só com sintomas motiva em vez de paralisar, *dado* o onboarding calmo. | Playtest; perguntar o momento de maior frustração | tempo até a primeira ação; desistências |
| H3 | Depois do debrief, o jogador explica por que a correção funcionou e cita uma alternativa. | Entrevista de 3 perguntas | % de explicações corretas |
| H4 | Quick Check ≤ 90 s mantém a iteração fluindo. | Tempos instrumentados | mediana do loop; nº de iterações |
| H5 | O evaluator é estável o bastante para ser confiável. | Rodar cada solução de referência N vezes em 3 máquinas | taxa de inversão PASS/FAIL. **1 de 3 máquinas medida (2026-09-25): 0 % em 45 testes**, com uma célula frágil (`evaluator.md` §4) |
| H6 | O jogador quer o próximo cenário. | "Começaria o C2 agora?" + retorno em 7 dias | % sim; retorno |
| H7 | O setup local é aceitável para a persona. | Tempo do download até o primeiro dashboard | mediana do setup; falhas |

A H5 é **técnica** e deve ser testada *antes* dos playtests com pessoas.

## 7. Métricas de sucesso

As metas abaixo são **limiares propostos para o playtest**, não dados medidos.

| Métrica | Meta proposta |
|---|---|
| Conclusão do C1 (playtest) | ≥ 60 % |
| Mediana do tempo até a primeira hipótese (C1) | ≤ 15 min |
| Mediana do loop Quick Check (edição → resultado) | ≤ 90 s |
| Taxa de inversão do evaluator nas soluções de referência | ≤ 2 % |
| Explicação correta de "por que funcionou" após o debrief | ≥ 70 % |
| "Continuaria para o próximo cenário" | ≥ 70 % |
| Setup (download → dashboard) | ≤ 10 min; ≤ 1 falha em 8 |

Eventos de produto em JSONL local (esquema em `docs/architecture/observability.md`). Nenhum conteúdo de código, nenhum dado pessoal.

## 8. Roadmap (ordem revisada)

| Chunk | Objetivo | Saída demonstrável |
|---|---|---|
| 0 | Product discovery | docs de produto |
| 1 | Arquitetura, threat model v0, ADRs | `docs/architecture`, `docs/security`, `docs/adr` |
| 2 | Fundação do repositório: layout, `justfile`, lint/format/test, CI, gitleaks, `CLAUDE.md` | `just check` verde no CI |
| 3 | App demo Go + seed determinística + compose de desenvolvimento | Nexora responde; busca lenta |
| 4 | Gerador de carga + perfis | Flash Sale imprime p50/p95/p99/erros |
| 5 | Telemetria: Prometheus, exporters, medição fora do app | séries temporais consultáveis de uma execução |
| 6 | Evaluator (CLI) + C1 ponta a ponta + **teste de estabilidade (H5)** | `sesim test` → PASS/FAIL por dimensão, estável |
| 7 | Preparação do Playtest #1: fluxo do jogador no terminal (`sesim play/next/status/check/test/hint`), conteúdo PT-BR, eventos, protocolo | C1 jogável de ponta a ponta; [`playtest-1.md`](playtest-1.md) |
| 7b | **Playtest #1** (C1 só no terminal, 3–5 pessoas), conduzido por você | relatório; go/no-go |
| 8 | Driver de sandbox programático: spec com hardening, ciclo de vida, registro de runs, GC, testes de limpeza | `just sandbox-check`: create/health/exec/destroy/verify-cleanup, kill no meio + gc |
| 8b | Jogo (`play`/`test`/`verify`) sobre o driver; aposentar `internal/devstack` e o compose no caminho do jogador | feito: fluxo do jogador e `verify` (20/20) com o compose desligado |
| 9 | UI web: **NexoraOS**, com desktop simulado, e-mail (briefing, incidente, debrief), Pulse, Terminal, Testes, Notas, Configurações (3 temas + personalização) e Tabby com roteiro fixo; regras do jogo em `internal/game`, compartilhadas com a CLI (ADR-0016) | feito: C1 jogado de ponta a ponta no navegador, de `play` ao PASS e ao debrief |
| 10a | Topologia (`topology.yaml`, catálogo fechado, custo modelado, deploy por reconciliação, nginx no workspace, PgBouncer); apps Arquitetura, Deploy e Docs (instalar = editar a topologia); C1 com dimensão custo (ADR-0017) | feito: escalar o app e instalar o PgBouncer pela UI; C1 verificado com 6 casos |
| 10b | C2 Connection Pressure + app Suporte (reclamações vindas dos erros reais) | C2 jogável |
| 10c | C3 + injetor de falhas + Laboratório de carga | C3 jogável |
| 10d | App Git + campanha do Stage 1 (C1 → C2 → C3, histórico de missões) | Stage 1 completo |
| 11 | **Playtest #2** | decisão sobre Stage 2 e modo hospedado |

## 9. Decisões tomadas (2026-09-24)

| Questão | Decisão | Registro |
|---|---|---|
| Linguagem do app demo | Go | ADR-0004 |
| MVP local-first | Aceito | ADR-0002 |
| Persona primária | Estudante do fim do curso | `vision.md` §4 |
| Idioma dos docs | PT-BR | — |
| Nome | Adiado; deve derivar da empresa fictícia | — |
| Licença | Código fechado, gratuito, com doação | ADR-0009 |
