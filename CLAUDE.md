# sesim — simulador de engenharia de software em sistemas reais (nome provisório)

## Stack
Go 1.27 (plataforma e app demo) · React + Vite + TS em `web/` (Node 22), embutida no binário · Docker (sandbox) · k6 · Prometheus · SQLite (`modernc.org/sqlite`).
Docs em PT-BR; código, comentários, logs e commits em inglês.

## Comandos
```bash
just setup | fmt | doctor | build-all   # npm ci + go mod download | formata Go e web | toolchain + Docker | cross-compile
just check      # gate completo: fmt-check → lint → typecheck → test → build → licenses → vuln → secrets
just images && just build && bin/sesim ui   # o jogo no navegador (NexoraOS); `bin/sesim play` = terminal (deploy/topology/docs/install também)
bin/sesim ui --addr 127.0.0.1:7777 --no-open & npm --prefix web run dev   # UI com hot reload (vite faz proxy da API)
just dev-load normal-day | dev-smoke | dev-telemetry-check   # carga k6 (RATE/DURATION/RAMP/SEED via env); aceites 3 e 5
just eval | verify --only trigram-index --runs 1   # Production Test do C1 | harness das referências
just images && just sandbox-check           # sandbox pela Docker API (sem compose): aceite do Chunk 8
```
Rodar `just check` antes de concluir qualquer mudança. Ferramentas Go fixadas em `tools/*.go.mod`, compiladas em `bin/tools/` (`just tools`); nunca as do sistema. Prometheus de dev em `127.0.0.1:9090`; stats de containers só enquanto `sesim dev container-stats` roda (o `dev-load` liga sozinho).

## Arquitetura
- `cmd/sesim` → binário único (CLI + servidor local + SPA embutida), ADR-0002/0003. `internal/` → um pacote por componente (`docs/architecture/overview.md` §4).
- `internal/sandbox/docker` → único pacote que pode importar `github.com/moby/moby` (depguard garante).
- `internal/topology` → `topology.yaml`: catálogo fechado, tamanhos, preços, custo, `Install`. `stack.Plan/Apply` reconcilia a sandbox com ele (ADR-0017).
- `internal/game` → regras de uma partida (sessão, sandbox, check/test/dica); a CLI (`internal/cli`) e a UI (`internal/ui`) só apresentam.
- `internal/ui` → servidor local do NexoraOS: API JSON com token, terminal por WebSocket, SPA de `web/dist` (`web/embed.go`). Temas = tokens (ADR-0016).
- `nexora/` → app demo, **módulo Go separado** (`module nexora`) com `vendor/` commitado; é o workspace que o jogador edita. Contrato em `nexora/API.md`.
- `scenarios/stage1/c1-slow-search/` → pacote do C1. `internal/scenario` lê; `internal/evaluator` julga; `internal/stack` roda a Nexora no driver `internal/sandbox/docker` (spec + hardening em `internal/sandbox`) para `play`/`test`/`verify`.
- `internal/seed` + `cmd/nexora-seed` → dataset determinístico; `internal/observer` + `cmd/observer-svc` → gateway emulado (zona observadora).
- `deploy/dev/compose.yml` → só ferramenta de dev do app; o jogo não usa. Os configs em `deploy/dev/{prometheus,initdb}` e `nexora/nginx/default.conf` (início do cenário; o jogo usa o do workspace) são montados pelos dois.
- `docs/` → produto, arquitetura, threat model e ADRs (`docs/adr/README.md`). Decisão nova ou alterada = ADR novo.

## Convenções (não-negociáveis)
- Evaluator nunca usa métricas expostas pelo app avaliado (ADR-0013).
- Objetivos de cenário avaliam propriedades, nunca a presença de tecnologia (ADR-0010).
- Nada AGPL/GPL linkado no binário; k6 e Grafana só como container separado (ADR-0009). `just licenses` aplica a política.
- Sem cgo no binário (`CGO_ENABLED=0`). `github.com/docker/docker` é proibido (deprecado; ADR-0005).
- Qualquer servidor local escuta só em `127.0.0.1` (threat model T4).
- Commits: Conventional Commits. Segredo falso de cenário vai para `.gitleaksignore` com justificativa; nunca contornar o hook.

## Armadilhas
- Node do sistema pode ser outra versão: use o do mise (`~/Work/.mise.toml` fixa a 22) ou `mise exec node@22 -- just …`.
- `golangci-lint` fica em módulo próprio (`tools/lint/go.mod`) para não misturar dependências. Atualize com `go get -tool -modfile=tools/lint/go.mod …@vX`. Não troque por `go tool -modfile` dentro de `nexora/`: o `vendor/` de lá quebra a resolução.
- Mudou `nexora/go.mod`? `just nexora-vendor`, senão o build offline quebra.
- O código do `nexora/` não comenta os problemas plantados pelos cenários (busca lenta, transação aberta durante o pagamento, idempotência ignorada). É proposital; não "conserte" nem documente no código.
- `web/dist` é embutido no binário: mudou a UI, `just build` (npm antes do Go). `web/dist/.gitkeep` fica commitado para o `go build` funcionar antes do npm.
- Textos do cenário usam `{{status}}`, `{{check}}`, `{{test}}`, `{{shell}}`, `{{hint}}`, `{{next}}`: a CLI e a UI trocam pelo nome da ferramenta de cada uma (`play.CLIWords`, `ui.Words`). Não escreva `sesim …` direto no conteúdo.
- A migração automática do Biome trocou `recommended` por `preset: "none"`, que desliga todas as regras. Revise o `biome.json` depois de `biome migrate`.
- k6: `handleSummary` recebe `setup_data` com tokens (o `traffic.js` apaga; não reverta); `discardResponseBodies` também vale no `setup()` (use `responseType: 'text'`).
- Redes do dev: `play`, `ingress`, `telemetry` (ADR-0014). Nunca coloque nginx/app/postgres na `telemetry`: o jogador controla a configuração do nginx.
- k6 remote write envia duração em **segundos**, como histograma nativo (`K6_FEATURES=native-histograms`): p95 = `histogram_quantile(0.95, sum by (name) (rate(k6_http_req_duration_seconds[..])))`. Não use `k6_http_req_duration_p95`: é cumulativo desde o início do run. O texto das queries do `pg_stat_statements` fica em `pg_stat_statements_query_id` (join por `queryid`).
- O reset do evaluator clona `nexora_seed`, que é criado uma vez com as migrações do **repo**. Mudou o schema base (`nexora/migrations/0001…`) ou a seed? `DROP DATABASE nexora_seed` (com `ALTER DATABASE … IS_TEMPLATE false` antes), senão os testes medem o dataset antigo.
- Não rode `just check`/builds pesados durante `just verify` ou um Production Test: disputa de CPU vira ruído na medição.
- O Postgres da sandbox roda com `synchronous_commit=off` (ADR-0018): a latência de fsync do disco do host não pode decidir veredito. Mudou a física do Postgres? Reverifique **todos** os cenários.
- Postgres: health check por TCP (`pg_isready -h 127.0.0.1`; pelo socket ele fica "pronto" no servidor temporário do init). `docker compose exec -T` consome o stdin: em scripts, `</dev/null`.

## Evitar
- Kubernetes, microsserviços, editor web, mentores de IA antes do Playtest #2 (`docs/product/mvp.md` §2).
- Dependência nova sem justificativa; preferir a stdlib.

## Estado
Fonte da verdade = git (`git log --oneline -5`) e o roadmap em `docs/product/mvp.md` §8.
