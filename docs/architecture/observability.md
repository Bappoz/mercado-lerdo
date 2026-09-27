# Observabilidade

> Status: Chunk 1, proposta; coleta implementada no Chunk 5 e eventos de produto no Chunk 7 (`internal/events`, `workspace/.sesim/events.jsonl`; a tabela do §4 lista o que o código emite hoje). Stack de métricas: ADR-0007.

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
| CPU (inclusive throttling do CFS), memória, pids e rede por container | stats do Docker lidas pelo sesim no host | push OTLP (`sesim_container_*`) | dashboard + evaluator (`resources`) |
| Conexões, locks, `pg_stat_statements`, cache hit, tuplas lidas | postgres_exporter (papel `sesim_observer`) | scrape | dashboard; C1 e C2 |
| Métricas do app (pool pgx: `acquire_count`, `acquire_duration`, conexões em uso/ociosas) | endpoint `/metrics` do app | scrape | **só dashboard**, nunca evaluator |
| Logs | Docker logs API | streaming pelo sesim | painel pesquisável |
| Traces | OpenTelemetry SDK no app → Collector → Jaeger | — | **fora do MVP**; candidato forte para o Stage 2 ("onde o tempo é gasto?") |

- **Coleta (resolvido no Chunk 5, ADR-0014):** um agente Prometheus nas redes `play` e `telemetry` faz o scrape do app, do postgres_exporter e do nginx_exporter e envia por remote write. O k6 envia por remote write. As stats de containers saem do host (sesim, Docker API) via OTLP JSON. Validação: `just dev-telemetry-check`.
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
| `scenario_started` / `scenario_resumed` | seed / fase | funil |
| `growth_event` | calm_seconds (implementado no lugar de `onboarding_completed`) | H2 |
| `status_viewed`, `shell_opened` | fase | como a pessoa investiga |
| `quick_check_run` | build_ms, migrate_ms, restart_ms, total_ms | H4 |
| `quick_check_failed` | stage (build/migrate/restart) | fricção |
| `production_test_run` | duração, vereditos por dimensão, `db_probe_ms` e `slow_host` (velocidade da máquina, ADR-0021) | H5 entre máquinas |
| `test_passed` / `test_failed` / `test_inconclusive` | dimensões | H1, H5 |
| `hint_requested` | nível | H1 |
| `hypothesis_recorded` | nº de evidências | `time_to_first_hypothesis` |
| `scenario_reset` | nível (dados/completo/hard) | fricção |
| `scenario_advanced` | from, canonical (ADR-0020) | campanha, H6 |
| `lab_run` | perfil, taxa, duração, queda, segundos | como a pessoa ensaia |
| `git_commit` | nº de arquivos | como a pessoa versiona |
| `story_event` | id, outcome (fired, met, missed) (ADR-0022) | se a história engaja; prazos cumpridos |
| `scenario_completed` | total_minutes, incident_minutes, tests, checks, hints | H1, H6 |
| `session_paused` | fase | abandono |
| `debrief_viewed` | tempo na tela | H3 (proxy fraco; a entrevista é o dado real) |
