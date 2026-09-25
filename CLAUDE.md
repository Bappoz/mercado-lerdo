# sesim — simulador de engenharia de software em sistemas reais (nome provisório)

## Stack
Go 1.27 (plataforma e app demo) · React + Vite + TS em `web/` (Node 22) · Docker (sandbox) · k6 · Prometheus · SQLite (`modernc.org/sqlite`).
Docs em PT-BR; código, comentários, logs e commits em inglês.

## Comandos
```bash
just setup      # npm ci + go mod download
just check      # gate completo: fmt-check → lint → typecheck → test → build → licenses → vuln → secrets
just fmt        # formata Go e web
just build-all  # cross-compile para dist/
just doctor     # toolchain + Docker
just dev-up | dev-psql | dev-reset          # stack de dev (compose); reset apaga o banco
just dev-load normal-day                    # carga k6 (normal-day | flash-sale); RATE/DURATION/RAMP/SEED via env
just dev-smoke | dev-telemetry-check        # aceite dos Chunks 3 e 5
just eval | verify --only trigram-index --runs 1   # Production Test do C1 | harness das referências
```
Prometheus de dev em `127.0.0.1:9090`. As stats de containers só chegam enquanto `sesim dev container-stats` roda (o `dev-load` liga sozinho; `just dev-stats` para rodar direto).
Rodar `just check` antes de concluir qualquer mudança. Ferramentas Go são fixadas em `tools/*.go.mod` e compiladas em `bin/tools/` por `just tools`; nunca use as do sistema.

## Arquitetura
- `cmd/sesim` → binário único (CLI + servidor local + SPA embutida). ADR-0002/0003.
- `internal/` → um pacote por componente; papéis em `docs/architecture/overview.md` §4.
- `internal/sandbox/docker` → único pacote que pode importar `github.com/moby/moby` (depguard garante).
- `web/` → SPA; só será embutida no Chunk 9.
- `nexora/` → app demo, **módulo Go separado** (`module nexora`) com `vendor/` commitado; é o workspace que o jogador edita. Contrato em `nexora/API.md`.
- `scenarios/stage1/c1-slow-search/` → pacote do C1 (manifesto, oráculo, referências como patches contra `nexora/`). `internal/scenario` lê; `internal/evaluator` julga; `internal/devstack` conduz o compose (temporário até o SandboxDriver).
- `internal/seed` + `cmd/nexora-seed` → dataset determinístico; `internal/observer` + `cmd/observer-svc` → gateway de pagamento emulado (zona observadora).
- `deploy/dev/compose.yml` → stack de dev que espelha redes/limites da sandbox; não é a sandbox do jogo.
- `docs/` → produto, arquitetura, threat model e ADRs (`docs/adr/README.md`). Decisão nova ou alterada = ADR novo.

## Convenções (não-negociáveis)
- Evaluator nunca usa métricas expostas pelo app avaliado (ADR-0013).
- Objetivos de cenário avaliam propriedades, nunca a presença de tecnologia (ADR-0010).
- Nada AGPL/GPL linkado no binário; k6 e Grafana só como container separado (ADR-0009). `just licenses` aplica a política.
- Sem cgo no binário (`CGO_ENABLED=0`). `github.com/docker/docker` é proibido (deprecado; ADR-0005).
- Qualquer servidor local escuta só em `127.0.0.1` (threat model T4).
- Commits: Conventional Commits.

## Armadilhas
- Node do sistema pode ser outra versão: use o do mise (`~/Work/.mise.toml` fixa a 22) ou `mise exec node@22 -- just …`.
- `golangci-lint` fica em módulo próprio (`tools/lint/go.mod`) para não misturar dependências. Atualize com `go get -tool -modfile=tools/lint/go.mod …@vX`. Não troque por `go tool -modfile` dentro de `nexora/`: o `vendor/` de lá quebra a resolução.
- Mudou `nexora/go.mod`? `just nexora-vendor`, senão o build offline quebra.
- O código do `nexora/` não comenta os problemas plantados pelos cenários (busca lenta, transação aberta durante o pagamento, idempotência ignorada). É proposital; não "conserte" nem documente no código.
- A migração automática do Biome trocou `recommended` por `preset: "none"`, que desliga todas as regras. Revise o `biome.json` depois de `biome migrate`.
- k6: `handleSummary` recebe `setup_data` com tokens (o `traffic.js` apaga; não reverta); `discardResponseBodies` também vale no `setup()` (use `responseType: 'text'`).
- Redes do dev: `play`, `ingress`, `telemetry` (ADR-0014). Nunca coloque nginx/app/postgres na `telemetry`: o jogador controla a configuração do nginx.
- k6 remote write envia duração em **segundos**, como histograma nativo (`K6_FEATURES=native-histograms`): p95 = `histogram_quantile(0.95, sum by (name) (rate(k6_http_req_duration_seconds[..])))`. Não use `k6_http_req_duration_p95`: é cumulativo desde o início do run. O texto das queries do `pg_stat_statements` fica em `pg_stat_statements_query_id` (join por `queryid`).
- O reset do evaluator clona `nexora_seed`, que é criado uma vez com as migrações do **repo**. Mudou o schema base (`nexora/migrations/0001…`) ou a seed? `DROP DATABASE nexora_seed` (com `ALTER DATABASE … IS_TEMPLATE false` antes), senão os testes medem o dataset antigo.
- Não rode `just check`/builds pesados durante `just verify`: disputa de CPU no host vira ruído na medição.
- `docker compose exec -T` consome o stdin do chamador: em scripts, sempre `</dev/null`.
- Segredo falso de cenário vai para `.gitleaksignore` com justificativa; nunca contornar o hook.

## Evitar
- Kubernetes, microsserviços, editor web, mentores de IA antes do Playtest #2 (`docs/product/mvp.md` §2).
- Dependência nova sem justificativa; preferir a stdlib.

## Estado
Fonte da verdade = git (`git log --oneline -5`) e o roadmap em `docs/product/mvp.md` §8.
