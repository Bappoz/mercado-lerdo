# ADR-0012: Persistência local — SQLite (puro Go) + JSONL + arquivos

**Status:** Proposto
**Data:** 2026-09-24
**Decisores:** Lucas Zanetti

## Contexto

O estado de campanha, testes, ledger e dicas é relacional e pequeno. Tudo roda na máquina do jogador. O binário não usa cgo (ADR-0003).

## Decisão

- **SQLite** via `modernc.org/sqlite` (tradução pura Go do SQLite, sem cgo), com `database/sql`, WAL e migrações embutidas.
- Eventos de produto em JSONL; artefatos de teste em arquivos (`docs/architecture/data-model.md`).
- Camada `store` fina, com SQL explícito, para trocar por Postgres no modo hospedado.

## Opções consideradas

| Dimensão | **modernc.org/sqlite** | `mattn/go-sqlite3` (cgo) | bbolt / arquivos JSON |
|---|---|---|---|
| Cross-compile sem cgo | Sim | Não | Sim |
| Desempenho | Um pouco menor | Maior | Alto, mas sem consultas |
| Consultas relacionais | Sim | Sim | Não |

## Consequências

- Binário maior. Irrelevante para o volume de dados esperado.
- Um processo sesim por diretório de dados: lock de instância única no start.

## Ações

1. [ ] Confirmar a licença e a versão do `modernc.org/sqlite` no Chunk 2.

## Fontes

- https://pkg.go.dev/modernc.org/sqlite
