# Observabilidade

> Status: Chunk 1, proposta. Stack de métricas: ADR-0007.

Há **três** coisas diferentes chamadas "observabilidade" neste produto. Misturá-las é o erro mais fácil de cometer.

| Camada | Para quem | O que é |
|---|---|---|
| 1. Telemetria do sistema do jogador | Jogador (é conteúdo do jogo) e evaluator | Métricas, logs e traces da Nexora e da infra da sandbox |
| 2. Observabilidade da plataforma | Nós, ao depurar o sesim | Logs estruturados do sesim, artefatos de execução |
| 3. Analytics de produto | Nós, para validar hipóteses | Eventos de uso, locais, opt-in para exportar |

## 1. Telemetria do sistema do jogador

### Sinais no MVP

| Sinal | Fonte | Coleta | Uso |
|---|---|---|---|
| Taxa, erros e latência por rota (vistos pelo cliente) | k6 | resumo JSON + séries (saída Prometheus remote write do k6; nome exato a confirmar na versão fixada) | dashboard + evaluator |
| Taxa, status e upstream time no edge | nginx (log estruturado → exporter, ou `stub_status`) | scrape | dashboard; investigação de C3 |
| CPU, memória, rede e restarts por container | stats do Docker (via sesim) ou cAdvisor | scrape | dashboard + evaluator (`resources`) |
| Conexões, locks, `pg_stat_statements`, cache hit, tuplas lidas | postgres_exporter (papel `sesim_observer`) | scrape | dashboard; C1 e C2 |
| Métricas do app (pool pgx: `acquire_count`, `acquire_duration`, conexões em uso/ociosas) | endpoint `/metrics` do app | scrape | **só dashboard**, nunca evaluator |
| Logs | Docker logs API | streaming pelo sesim | painel pesquisável |
| Traces | OpenTelemetry SDK no app → Collector → Jaeger | — | **fora do MVP**; candidato forte para o Stage 2 ("onde o tempo é gasto?") |

- **Problema em aberto (Chunk 5):** o app expõe `/metrics` na porta 9091, dentro da rede `play`. O Prometheus fica só na `obs` (ADR-0013). Ligar o Prometheus à rede `play` quebraria o T12. Candidatos: o sesim faz o scrape e repassa (remote write) ou um agente de scrape dedicado nas duas redes, sem porta de escrita exposta à `play`.
- **Intervalo de scrape:** 1–2 s durante testes, 5 s no restante (a medir o custo). Retenção = duração da run; o que o relatório precisa é exportado para `runs/<test_id>/`.
- **Por que o pool do app aparece mesmo sem entrar no evaluator:** o C2 depende de o jogador *ver* o tempo esperando conexão. É instrumentação que o jogador pode ler e também quebrar, e isso é realista.
- **O dashboard é nosso (uPlot), não o Grafana.** O Prometheus fica acessível no toolbox (`curl prometheus:9090/api/v1/query…`) para quem quiser aprender PromQL. Grafana como painel opcional fica para depois (AGPLv3; rodar a imagem oficial sem modificar é aceitável pela política do ADR-0009).

### Painéis iniciais (Stage 1)

1. **Golden signals do edge:** RPS, taxa de erro, p50/p95/p99.
2. **Saturação:** CPU/mem por container em relação à quota (USE).
3. **Banco:** conexões ativas/ociosas/esperando vs. `max_connections`, top queries por tempo total, cache hit.
4. **App:** pool (em uso, espera por conexão), goroutines, heap.
5. **Linha do tempo:** eventos da timeline, deploys (checkpoints) e falhas injetadas, sobrepostos a todos os gráficos.

## 2. Observabilidade da plataforma

- `log/slog` em JSON, com `run_id`, `attempt_id` e `test_id` em todo log relevante; nível configurável (`SESIM_LOG_LEVEL`).
- Arquivo: `$XDG_STATE_HOME/sesim/logs/sesim.log` com rotação por tamanho.
- `sesim doctor --bundle` gera um zip para bug report: versões, `docker info`, calibração, últimos logs, relatório do último teste. **Não inclui** o workspace nem o conteúdo do banco.
- Métricas internas (duração de build, de reset, do loop Quick Check) viram eventos de produto, porque são a H4.

## 3. Saúde da sandbox para o jogador

Um painel pequeno e sempre visível responde "é o jogo ou sou eu?":

| Estado | Significado |
|---|---|
| Plataforma OK | sesim, Docker e observadores saudáveis |
| Seu sistema | health de nginx/app/postgres (pode estar ruim por causa do jogador, e isso é jogo) |
| Gerador de carga | sustentando a taxa (`dropped_iterations == 0`) |

## 4. Analytics de produto

Local por padrão (`data-model.md` §3). Exportação só com `sesim telemetry export`, que gera um arquivo para o jogador enviar se quiser (playtests).

| Evento | Props | Serve para |
|---|---|---|
| `install_first_run` | os, arch, docker_version, cores, mem | H7 |
| `doctor_failed` | check | H7 |
| `scenario_started` | scenario, seed | funil |
| `onboarding_completed` | duração | H2 |
| `quick_check_run` | build_ms, restart_ms, total_ms | H4 |
| `production_test_run` | duração, runs | — |
| `test_passed` / `test_failed` / `test_inconclusive` | dimensões | H1, H5 |
| `hint_requested` | nível | H1 |
| `hypothesis_recorded` | nº de evidências | `time_to_first_hypothesis` |
| `scenario_reset` | nível (dados/completo/hard) | fricção |
| `canonical_state_adopted` | scenario | campanha |
| `scenario_completed` | duração, nº de testes, maior nível de dica | H1, H6 |
| `debrief_viewed` | tempo na tela | H3 (proxy fraco; a entrevista é o dado real) |
