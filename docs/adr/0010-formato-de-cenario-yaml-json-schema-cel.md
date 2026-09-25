# ADR-0010: Formato de cenário e objetivos — YAML + JSON Schema + CEL

**Status:** Proposto — objetivos em CEL substituídos por objetivos estruturados no v0 (ADR-0015)
**Data:** 2026-09-24
**Decisores:** Lucas Zanetti

## Contexto

Cenários precisam ser declarativos, versionados, reproduzíveis e testáveis automaticamente (o conteúdo é o gargalo). Os objetivos precisam expressar condições compostas ("p95 < 300 **e** nenhum pagamento duplicado") sem virar código arbitrário, porque um dia pacotes de fora podem ser carregados.

## Decisão

- Manifesto `scenario.yaml` com `apiVersion: sesim.dev/v0alpha1`, validado por JSON Schema (`schemas/scenario.schema.json`), mais um linter com as invariantes de `docs/architecture/simulation-engine.md` §2.
- Medições nomeadas (`measurements`), cada uma com fonte declarada; objetivos e pré-condições como expressões **CEL** (`github.com/google/cel-go`) sobre essas medições e fatos.
- `topology.yaml` do workspace segue o mesmo padrão (schema + catálogo fechado).
- O formato é **instável** (`v0alpha1`) até o Playtest #2.

## Opções consideradas

| Dimensão | **YAML + CEL** | YAML com comparadores fixos (`lt`, `gt`) | OPA/Rego | Go/JS embutido (scripts) |
|---|---|---|---|---|
| Expressividade | Boa (booleanos, aritmética, listas) | Baixa | Alta | Total |
| Segurança com pacote não confiável | Não Turing-completo, custo limitável | Total | Boa | **Ruim** |
| Curva para autor | Baixa | Mínima | Alta | Média |
| Mensagem de erro/explicação | Boa (AST inspecionável) | Boa | Média | Ruim |
| Licença | Apache-2.0 | — | Apache-2.0 | — |

## Análise de trade-offs

Comparadores fixos não expressam "razão de pedidos perdidos = 0 **e** p99 < X durante a janela do kill". Rego é poderoso demais para o problema. CEL fica no meio e é seguro para conteúdo de terceiros no futuro.

## Consequências

- O linter proíbe objetivos que usem `topology` fora de `cost`/`complexity` (princípio "propriedades, não ferramentas").
- Todo cenário exige ≥2 referências PASS e o harness `sesim scenario verify` no CI.

## Ações

1. [ ] Confirmar a API atual do `cel-go` (env, custo máximo) no context7 (Chunk 6).
2. [ ] Escrever o schema v0 junto com o primeiro cenário (Chunk 6), não antes.
