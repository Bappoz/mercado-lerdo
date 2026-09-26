# ADRs

Formato e regras: [ADR-0001](0001-registrar-decisoes-com-adrs.md).

| # | Decisão | Status |
|---|---|---|
| [0001](0001-registrar-decisoes-com-adrs.md) | Registrar decisões com ADRs | Aceito |
| [0002](0002-mvp-local-first.md) | MVP local-first | Aceito |
| [0003](0003-linguagem-da-plataforma-go.md) | Plataforma em Go | Aceito |
| [0004](0004-linguagem-do-app-demo-go.md) | App demo em Go (`net/http` + `pgx`, sem ORM) | Aceito |
| [0005](0005-driver-de-sandbox-docker-engine-api.md) | Driver de sandbox: Docker Engine API (`moby/moby/client`) atrás de `SandboxDriver` | Proposto |
| [0006](0006-gerador-de-carga-k6.md) | Carga: k6 em container separado (AGPL) | Proposto |
| [0007](0007-stack-de-metricas-prometheus.md) | Métricas: Prometheus + exporters; dashboard próprio | Proposto |
| [0008](0008-terminal-xterm-websocket-exec.md) | Terminal: xterm.js + WebSocket + `docker exec` | Proposto |
| [0009](0009-licenciamento-e-distribuicao.md) | Gratuito, código fechado, doação; política de licenças | Aceito / política Proposta |
| [0010](0010-formato-de-cenario-yaml-json-schema-cel.md) | Cenário: YAML + JSON Schema + CEL | Proposto |
| [0011](0011-frontend-react-vite-spa-embutida.md) | Frontend: React + Vite, SPA embutida | Proposto (bibliotecas revistas pelo 0016) |
| [0012](0012-persistencia-local-sqlite.md) | Persistência: SQLite puro Go + JSONL | Proposto |
| [0013](0013-zonas-de-rede-e-medicao-externa.md) | Zonas play/obs e medição externa | Proposto (redes refinadas pelo 0014) |
| [0014](0014-topologia-de-telemetria.md) | Telemetria: agente de coleta, redes `ingress`/`telemetry`, stats de containers pelo host via OTLP | Proposto |
| [0015](0015-objetivos-estruturados-no-v0.md) | Objetivos estruturados no v0; CEL adiado | Proposto |
| [0016](0016-nexoraos-desktop-simulado-e-temas-por-tokens.md) | NexoraOS: desktop simulado; temas como conjuntos de tokens; bibliotecas do 0011 adiadas | Proposto |
| [0017](0017-topologia-catalogo-fechado-e-deploy-por-reconciliacao.md) | Topologia: `topology.yaml` com catálogo fechado, custo modelado, deploy por reconciliação | Proposto |
