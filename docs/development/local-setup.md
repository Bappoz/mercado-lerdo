# Setup local

## Requisitos

| Ferramenta | Versão | Observação |
|---|---|---|
| Go | 1.27.x (ver `go.mod`) | `golangci-lint`, `govulncheck` e `go-licenses` vêm do `tools/`, sem instalação global |
| Node | 22 (ver `.node-version`) | npm incluso |
| just | ≥ 1.x | executor canônico de comandos |
| gitleaks | 8.30.x | usado pelo hook `pre-commit` e por `just secrets` |
| Docker | Engine 29+ | só a partir do Chunk 3; `just doctor` avisa se faltar |
| gcc | qualquer | necessário para `go test -race` (o binário final não usa cgo) |

## Primeira vez

```bash
git clone git@github.com:Bappoz/software-engineering-simulator.git
cd software-engineering-simulator
just doctor
just setup
just check
```

O hook `pre-commit` com gitleaks entra pelo `init.templateDir` global. Num clone novo sem o template, instale manualmente: copie `~/.config/git/template/hooks/pre-commit` para `.git/hooks/`.

## Rotina

| Quero… | Comando |
|---|---|
| Rodar a CLI | `go run ./cmd/sesim version` |
| UI em modo dev | `npm --prefix web run dev` (escuta só em 127.0.0.1) |
| Formatar tudo | `just fmt` |
| Gate completo | `just check` |
| Binários de todas as plataformas | `just build-all` → `dist/` |

## Estrutura de ferramentas

- `tools/go.mod`: `govulncheck` e `go-licenses` via diretiva `tool`.
- `tools/lint/go.mod`: `golangci-lint` isolado, como recomenda a própria documentação, para não compartilhar dependências.
- Política de licenças (ADR-0009): allowlist em `justfile` (Go) e `web/scripts/check-licenses.mjs` (npm). **Mantenha as duas iguais.**

## CI

`.github/workflows/ci.yml` roda `just setup && just check` e depois `just build-all`. O CI usa as mesmas receitas do ambiente local.
