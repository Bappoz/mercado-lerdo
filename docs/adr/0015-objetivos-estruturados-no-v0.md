# ADR-0015: Objetivos estruturados no formato v0; CEL adiado

**Status:** Proposto
**Data:** 2026-09-25
**Decisores:** Lucas Zanetti
**Substitui parcialmente:** ADR-0010 (a parte "objetivos como expressões CEL")

## Contexto

O ADR-0010 previa objetivos como expressões CEL (`p95_ms < 300`) com uma `margin` opcional. Ao implementar o evaluator (Chunk 6), a regra de veredito (`evaluator.md` §4) mostrou que precisa conhecer **o limiar e a direção** de cada objetivo:

- PASS se todas as execuções ficam do lado bom com folga de `margin`;
- FAIL se todas ficam do lado ruim com folga;
- INCONCLUSIVE no meio.

Numa expressão CEL arbitrária, limiar e direção ficam escondidos: não existe um "`p95_ms` com 10 % de folga" genérico para `a < b && c > d`. Os quatro objetivos do C1 são todos da forma `medida (< | >=) limiar`.

## Decisão

No formato `v0alpha1`, objetivos são **estruturados**:

```yaml
- {id: search-p95, dimension: latency, measure: search_p95_ms, below: 300, margin: 0.10}
- {id: search-matches-contract, dimension: correctness, measure: oracle_pass_ratio, atLeast: 1}
```

- `measure` precisa ser uma medida conhecida pelo evaluator (`scenario.Measures`). O loader rejeita qualquer outra, o que impede objetivos do tipo "tem Redis?" (princípio "propriedades, não tecnologias").
- CEL (`cel-go`) volta quando um cenário real precisar de predicado composto ou de pré-condição sobre fatos do ledger (`requires`, gatilhos tardios). Nesse momento, objetivos CEL serão **booleanos sem margem**.

## Opções consideradas

| Opção | Margem/INCONCLUSIVE | Dependência | Expressividade |
|---|---|---|---|
| **Estruturado (`measure` + `below`/`atLeast`)** | Natural | Nenhuma | Suficiente para o Stage 1 |
| CEL com margem inferida da AST | Frágil; só em comparações simples | `cel-go` | Alta, mas a margem vale só no caso simples |
| CEL sem margem | Sem INCONCLUSIVE, o que volta a gerar veredito falso sob ruído | `cel-go` | Alta |

## Consequências

- Uma dependência a menos agora. A validação por schema continua via `KnownFields` do YAML: campo desconhecido é erro.
- Cenários futuros com regras compostas precisarão de uma extensão do formato (`v0alpha2`).

## Ações

1. [x] `internal/scenario`: tipos + validação; `internal/evaluator.Judge`.
2. [ ] Reintroduzir CEL junto com `requires`/ledger (campanha, pós-Playtest #1).
