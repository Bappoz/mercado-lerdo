# Setup local

## Requisitos

| Ferramenta | Versão | Observação |
|---|---|---|
| Go | 1.27.x (ver `go.mod`) | `golangci-lint`, `govulncheck` e `go-licenses` vêm do `tools/`, sem instalação global |
| Node | 22 (ver `.node-version`) | npm incluso |
| just | ≥ 1.x | executor canônico de comandos |
| gitleaks | 8.30.x | usado pelo hook `pre-commit` e por `just secrets` |
| Docker | Engine 29+ com Compose v2 | usuário precisa de acesso ao socket (grupo `docker`, ver abaixo) |
| gcc | qualquer | necessário para `go test -race` (o binário final não usa cgo) |

## Primeira vez

```bash
git clone git@github.com:Bappoz/mercado-lerdo.git
cd mercado-lerdo
just doctor
just setup
just check
```

O hook `pre-commit` com gitleaks entra pelo `init.templateDir` global. Num clone novo sem o template, instale manualmente: copie `~/.config/git/template/hooks/pre-commit` para `.git/hooks/`.

### Acesso ao Docker

O grupo `docker` equivale a root no host (threat model, premissas). Para adicionar:

```bash
sudo usermod -aG docker "$USER"   # depois: novo login (ou reiniciar a sessão gráfica)
```

## Stack de desenvolvimento do Nexora (compose; só para desenvolver o app)

```bash
just dev-up      # build das imagens, Postgres, migrações + seed, app, nginx; espera tudo healthy
curl -s '127.0.0.1:8080/search?q=lamp&limit=3' | jq
just dev-psql    # psql como role nexora (dono do banco, não superuser)
just dev-hash    # hash do dataset: mesma seed → mesmo hash
just dev-reset   # apaga o banco; o próximo dev-up refaz o seed
just dev-load normal-day            # carga k6; tabela por endpoint + JSON bruto em .runs/
RATE=80 DURATION=60s just dev-load flash-sale
```

Perfis em `workloads/nexora/`: modelo aberto (arrival rate), sequência de intenções determinística por `SEED`, timeout de cliente de 5 s. Variáveis: `RATE` (pico, req/s), `DURATION`, `RAMP` (só Flash Sale), `SEED`. A CPU de um container pode ser mudada ao vivo: `docker update --cpus 1.0 nexora-dev-postgres-1`.

| Variável | Padrão | Efeito |
|---|---|---|
| `PG_CPUS` | `0.5` | CPU do Postgres (física escalada; calibrado no Chunk 4, ver `docs/architecture/sandbox.md` §7) |
| `PG_MAX_CONNECTIONS` | `50` | `max_connections` |
| `APP_CPUS` | `0.5` | CPU do app |
| `NEXORA_DB_MAX_CONNS` | `10` | tamanho do pool do app |
| `SEED` | `4242` | seed do dataset (só vale num banco vazio) |
| `NEXORA_DEV_PORT` | `8080` | porta em `127.0.0.1` |

Usuários do seed: `user00001@nexora.test` … `user05000@nexora.test`, todos com a senha `nexora-demo` (credencial de demonstração, só existe no banco local).

## Jogar (C1, terminal)

```bash
just images && just build   # imagens locais + bin/sesim
bin/sesim play              # cria workspace/, sobe a loja (sandbox pela Docker API), briefing e tráfego
bin/sesim help              # status, next, check, test, hint, shell, stop
bin/sesim stop --down       # desliga a loja; workspace e progresso ficam
```

O jogo não usa o compose. Os endereços da API e do Prometheus aparecem no briefing: são IPs dos containers, alcançáveis só no Linux (Docker Desktop ainda não suportado).

O workspace do jogador é `workspace/` (fora do git do projeto; é um repositório git próprio, com a tag do início do cenário). O progresso fica em `workspace/.sesim/state.json` e os eventos em `workspace/.sesim/events.jsonl`. Para recomeçar do zero: `bin/sesim stop && rm -rf workspace`. Protocolo do playtest: [`../product/playtest-1.md`](../product/playtest-1.md).

## Avaliação (evaluator)

```bash
just eval                                   # Production Test do C1 contra nexora/ (≈3–4 min)
just eval --workspace /caminho/outro-app    # outra cópia do app
just verify --rounds 1 --runs 1             # matriz: quebrado + 4 referências (≈5 min)
just verify --rounds 3                      # estabilidade (H5): ≈45 min; não use a máquina para trabalho pesado enquanto roda
```

Os relatórios JSON ficam em `.runs/`. O exit code de `sesim test` é 3 quando alguma dimensão não passa; o de `sesim scenario verify` é 3 quando há veredito inesperado ou instável.

## Sandbox pela Docker API (sem compose)

```bash
just images                 # imagens locais sesim/* (as de terceiros são baixadas por digest sob demanda)
bin/sesim sandbox up        # imprime o run id e as URLs (IP do container; só Linux)
bin/sesim sandbox ls
bin/sesim sandbox exec <run> toolbox -- psql -c 'select count(*) from products'
bin/sesim sandbox down <run>
bin/sesim sandbox gc        # coleta sandboxes órfãos (--all: todos)
just sandbox-check          # aceite: ciclo de vida, isolamento, limpeza, kill no meio + gc
```

## Rotina

| Quero… | Comando |
|---|---|
| Rodar a CLI | `go run ./cmd/sesim version` |
| Jogar no navegador | `just images && just build && bin/sesim ui` (abre o LerdOS; `--no-open` só imprime o endereço) |
| UI em modo dev | `bin/sesim ui --addr 127.0.0.1:7777 --no-open` e, em outro terminal, `npm --prefix web run dev` (o vite faz proxy de `/api` e `/ws` para a 7777) |
| Testes da UI | `npm --prefix web test` (vitest; também roda no `just test`) |
| Formatar tudo | `just fmt` |
| Gate completo | `just check` |
| Binários de todas as plataformas | `just build-all` → `dist/` |

## Estrutura de ferramentas

- `tools/go.mod`: `govulncheck` e `go-licenses` via diretiva `tool`.
- `tools/lint/go.mod`: `golangci-lint` isolado, como recomenda a própria documentação, para não compartilhar dependências.
- `just tools` compila os três em `bin/tools/` (o cache do Go torna isso rápido); todas as receitas usam esses binários, inclusive dentro de `nexora/`.
- Política de licenças (ADR-0009): allowlist em `justfile` (Go) e `web/scripts/check-licenses.mjs` (npm). **Mantenha as duas iguais**, com uma exceção: OFL-1.1 só na web, porque se aplica a fontes (ADR-0016).

## CI

`.github/workflows/ci.yml` roda `just setup && just check` e depois `just build-all`. O CI usa as mesmas receitas do ambiente local.
