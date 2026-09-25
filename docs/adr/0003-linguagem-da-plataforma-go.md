# ADR-0003: Linguagem da plataforma — Go

**Status:** Aceito
**Data:** 2026-09-24
**Decisores:** Lucas Zanetti

## Contexto

A plataforma (CLI, servidor local, orquestração, evaluator) é principalmente cola sobre ferramentas de infraestrutura: Docker Engine API, k6, Prometheus e, no futuro, containerd, Firecracker e Kubernetes. Precisa ser distribuída como binário único para Linux, macOS e Windows, com streaming concorrente (PTY via WebSocket, logs, métricas ao vivo).

## Decisão

Go para toda a plataforma. Preferir a stdlib (`net/http` com padrões de rota, `log/slog`, `embed`, `os/exec`, `context`) e justificar cada dependência.

## Opções consideradas

| Dimensão | **Go** | Rust | TypeScript |
|---|---|---|---|
| Ecossistema de infra | Nativo (moby, containerd, Firecracker SDK, client-go, Prometheus client) | Bom (`bollard`, `kube-rs`; Firecracker é Rust) | Fino (`dockerode`) |
| Binário único e cross-compile | Trivial (sem cgo) | Bom, com mais atrito de toolchain | Possível (Bun/Deno compile), pesado |
| Concorrência para streaming | Goroutines, simples | tokio, poderoso e verboso | Event loop, suficiente |
| Velocidade de desenvolvimento solo | Alta | Média | Alta |
| Aprendizado para o autor | Médio | Alto | Baixo |

## Análise de trade-offs

O risco do projeto é **conteúdo e evaluator**, não performance da plataforma. Go minimiza o atrito com as ferramentas usadas e mantém um só idioma com o app demo (ADR-0004). Rust ensinaria mais, mas custaria velocidade onde ela mais importa. Fica reservado para um componente que o justifique (ex.: agente de convidado numa microVM, gerador de carga próprio).

## Consequências

- Mais fácil: integrar Docker e Prometheus; distribuir; contribuir sozinho.
- Mais difícil: tipagem de domínio menos expressiva que Rust. Compensar com testes de propriedade nos limites críticos (spec da sandbox, veredito).
- Proibido cgo no binário principal (cross-compile) → SQLite puro Go (ADR-0012).

## Ações

1. [ ] Fixar a versão do Go no `go.mod` e no CI (Chunk 2).
2. [ ] `golangci-lint` + `govulncheck` no CI.
