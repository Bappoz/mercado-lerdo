# ADR-0007: Stack de métricas — Prometheus + exporters, dashboard próprio

**Status:** Proposto
**Data:** 2026-09-24
**Decisores:** Lucas Zanetti

## Contexto

O jogo precisa de séries temporais de nginx, Postgres, containers, k6 e app, com resolução de 1–5 s, consultáveis pelo evaluator e visíveis no dashboard. O brief pede ferramentas reais, sem esconder a complexidade.

## Decisão

- **Prometheus** na zona observadora, retenção igual à duração da run, com `postgres_exporter`, exporter do nginx e stats de containers (via API do Docker coletada pelo sesim, ou cAdvisor; decidir pela medição de consumo no Chunk 5).
- O evaluator consulta via HTTP API (PromQL) pelo client Go oficial.
- **Dashboard próprio** na SPA (uPlot), não Grafana, para controlar a narrativa (anotações de eventos, selos de proveniência).
- Prometheus acessível pelo toolbox para quem quiser aprender PromQL.

## Opções consideradas

| Dimensão | **Prometheus** | VictoriaMetrics single-node | Sem TSDB (sesim guarda amostras) |
|---|---|---|---|
| Valor pedagógico | Alto (padrão de mercado) | Médio (PromQL compatível) | Nenhum |
| Pegada | Média | Menor | Mínima |
| Ecossistema de exporters | Padrão | Compatível | Precisa reimplementar o scrape |
| Licença | Apache-2.0 | Apache-2.0 | — |

## Análise de trade-offs

O Prometheus ensina algo transferível para o mercado. O VictoriaMetrics é o plano B, compatível e com a mesma interface, se a meta de ≤ 3 GB de RAM (`sandbox.md`) não fechar. O Grafana (AGPL-3.0) fica fora do MVP; ver ADR-0009 para usá-lo depois como imagem não modificada.

## Consequências

- Mais fácil: evaluator em PromQL; exporters prontos.
- Mais difícil: a cardinalidade precisa de cuidado (rotas do app com labels limitados).
- A revisitar: substituir por VictoriaMetrics se o consumo medido estourar o orçamento.

## Ações

1. [ ] Medir RAM/CPU do Prometheus com scrape de 1 s durante o Flash Sale (Chunk 5).
2. [ ] Decidir entre cAdvisor e stats do Docker (Chunk 5).
