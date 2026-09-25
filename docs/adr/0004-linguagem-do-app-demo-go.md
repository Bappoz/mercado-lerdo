# ADR-0004: Linguagem do app demo (Nexora) — Go, `net/http` + `pgx`, sem ORM

**Status:** Aceito
**Data:** 2026-09-24
**Decisores:** Lucas Zanetti

## Contexto

O app demo é o código que o jogador **lê e modifica**. A linguagem define o alcance, os tipos de problema que surgem naturalmente e a velocidade de rebuild no loop.

## Decisão

- Go, com `net/http` da stdlib e `github.com/jackc/pgx/v5` (`pgxpool`), SQL explícito em arquivos ou constantes, sem ORM.
- Migrações SQL versionadas em `nexora/migrations/`, aplicadas pela plataforma no início de cada execução de teste.
- Dependências vendorizadas (`vendor/`) para build sem rede (`docs/architecture/sandbox.md` §5).
- Código deliberadamente simples e idiomático; os problemas dos cenários ficam em pontos específicos e realistas, não espalhados.

## Opções consideradas

| Dimensão | **Go** | Python (FastAPI + SQLAlchemy) | TypeScript (Fastify + pg) |
|---|---|---|---|
| Legibilidade para a persona | Boa; sintaxe pequena | Muito boa | Boa |
| Problemas naturais de pool | Explícitos (`pgxpool.Config.MaxConns`) | Explícitos (engine pool) | Explícitos (`Pool.max`) |
| N+1 via ORM (Stage 2) | Precisa ser escrito à mão | Natural | Possível com ORM |
| Rebuild no loop | Compilação rápida, binário estático | Sem build; restart | Build TS rápido |
| Ruído de performance | Baixo e previsível | GIL e modelo de workers confundem | Event loop bloqueável |
| Coerência com a plataforma | Total | — | — |

## Análise de trade-offs

Go dá medições mais previsíveis (menos ruído no evaluator) e um mesmo idioma no projeto. Perde parte do alcance entre estudantes, que costumam ver mais Java, Python ou JS na graduação. Mitigação: o código do Stage 1 é pequeno e comentado onde o porquê não é óbvio. Sem ORM, o SQL fica visível, o que é bom para o C1.

## Consequências

- O N+1 do Stage 2 terá de nascer de um loop explícito de queries. É realista em Go.
- A arquitetura permite "sabores" futuros em outra linguagem, desde que respeitem o **contrato da API v1** e o schema. O oráculo e o workload são agnósticos.
- A revisitar se o Playtest #1 mostrar que Go é barreira para a persona.

## Ações

1. [ ] Confirmar a API atual do `pgxpool` (métricas de `Stat()`) no context7 antes do Chunk 3.
2. [ ] Documentar o contrato da API v1 (`order_ref`, `Idempotency-Key`, formato da busca) no Chunk 3.
