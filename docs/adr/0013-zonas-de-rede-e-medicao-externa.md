# ADR-0013: Duas zonas de rede e medição externa ao sistema avaliado

**Status:** Proposto
**Data:** 2026-09-24
**Decisores:** Lucas Zanetti

## Contexto

O evaluator é a promessa do produto. Se o sistema avaliado pudesse influenciar as medições (métricas do próprio app, acesso ao Prometheus, forjar registros), o PASS perderia o sentido, ainda mais em um futuro modo hospedado ou com comparação entre jogadores.

## Decisão

- Duas redes Docker `internal` por run: **play** (tudo que o jogador controla) e **obs** (k6, Prometheus, observer-svc, exporters).
- Pontes explícitas e mínimas: nginx (entrada do tráfego), observer-svc (gateway de pagamento "externo" e recorder) e exporters/probe com papel Postgres somente leitura.
- **Objetivos só usam medições da zona observadora, do daemon/host ou do oráculo.** Métricas expostas pelo app nunca entram em objetivos.
- A integridade de pedidos é verificada de forma cruzada (observer-svc × probe SQL), com `order_ref` como parte do contrato da API.

## Opções consideradas

| Dimensão | **Duas zonas** | Rede única | Medição via métricas do app |
|---|---|---|---|
| Integridade | Alta | Média (o jogador alcança o Prometheus) | Nenhuma |
| Complexidade | Média | Baixa | Baixa |
| Realismo pedagógico | Alto (dependência externa real, o "pagamento") | Médio | — |

## Consequências

- O mock de pagamento vira peça central dos cenários de integridade e de falha de dependência (C2), o que é bom para o jogo.
- Custo: mais containers e configuração de rede; no Docker Desktop, port-forwards.
- Localmente o jogador ainda pode adulterar o SQLite. Aceito (threat model T11).
