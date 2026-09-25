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
```
Rodar `just check` antes de concluir qualquer mudança. Ferramentas Go vêm do go.mod em `tools/` (`go tool -modfile=…`), não do sistema.

## Arquitetura
- `cmd/sesim` → binário único (CLI + servidor local + SPA embutida). ADR-0002/0003.
- `internal/` → um pacote por componente; papéis em `docs/architecture/overview.md` §4.
- `internal/sandbox/docker` → único pacote que pode importar `github.com/moby/moby` (depguard garante).
- `web/` → SPA; só será embutida no Chunk 9.
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
- `golangci-lint` fica em módulo próprio (`tools/lint/go.mod`) para não misturar dependências. Atualize com `go get -tool -modfile=tools/lint/go.mod …@vX`.
- A migração automática do Biome trocou `recommended` por `preset: "none"`, que desliga todas as regras. Revise o `biome.json` depois de `biome migrate`.
- Segredo falso de cenário vai para `.gitleaksignore` com justificativa; nunca contornar o hook.

## Evitar
- Kubernetes, microsserviços, editor web, mentores de IA antes do Playtest #2 (`docs/product/mvp.md` §2).
- Dependência nova sem justificativa; preferir a stdlib.

## Estado
Fonte da verdade = git (`git log --oneline -5`) e o roadmap em `docs/product/mvp.md` §8.
