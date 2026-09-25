# ADR-0006: Gerador de carga — k6 em container separado

**Status:** Proposto
**Data:** 2026-09-24
**Decisores:** Lucas Zanetti

## Contexto

Precisamos gerar tráfego que reproduza jornadas de usuário (visitante, cliente que compra), com **modelo aberto** (as chegadas não dependem da resposta do sistema, como num pico de tráfego real), medição de latência no cliente e scripts versionáveis por cenário. O produto é código fechado (ADR-0009).

## Decisão

- **k6**, imagem oficial `grafana/k6` fixada por digest, **não modificada**, rodando como container na zona observadora.
- Scripts dos cenários em JavaScript, dentro do pacote de cenário.
- Executores `constant-arrival-rate` / `ramping-arrival-rate`. `dropped_iterations > 0` invalida a execução (INCONCLUSIVE).
- Resultados: resumo JSON no fim e séries temporais via saída Prometheus remote write (nome e flags exatos a confirmar na versão fixada).
- **Nunca** importar o k6 como biblioteca Go no binário do sesim.

## Opções consideradas

| Dimensão | **k6** | Vegeta | Locust | Gerador próprio em Go |
|---|---|---|---|---|
| Modelo aberto | Sim (arrival-rate) | Sim (taxa constante) | Fechado por padrão | Qualquer |
| Jornadas com estado (login → carrinho → checkout) | Sim | Não (alvos soltos) | Sim | Sim, com esforço |
| Scripts versionáveis por cenário | JS | Arquivos de alvos | Python | Código Go |
| Pegada por VU | Média (runtime JS) | Muito baixa | Alta | Baixa |
| Correção de latência (coordinated omission, HDR) | Madura | Madura | Cuidado necessário | Reinventar |
| Licença | **AGPL-3.0** | MIT | MIT | Nossa |

## Análise de trade-offs

O k6 é o único que junta modelo aberto, jornadas com estado e maturidade. A AGPL-3.0 conflita com código fechado **só se** o k6 for incorporado ou modificado. Rodar a imagem oficial, sem modificação, como programa separado que conversa por CLI/arquivos/rede é uso comum. Isto não é parecer jurídico; revisar antes do primeiro release público. Se o risco de licença ou a pegada por VU virarem problema, o plano B é um gerador próprio em Go com HDR histogram. O contrato de saída do `workload` isola essa troca.

## Consequências

- Mais fácil: escrever perfis realistas rápido.
- Mais difícil: a taxa não muda "a quente" de forma simples. O tráfego de fundo reinicia com a nova taxa (a verificar no Chunk 4).
- Dependência do nosso formato de resultado → definir um `workload.Result` próprio e converter o resumo do k6 para ele.

## Ações

1. [ ] Confirmar no context7/docs oficiais: executores arrival-rate, saída remote write, `handleSummary` (Chunk 4).
2. [ ] Medir memória por VU no perfil Flash Sale.
3. [ ] Revisão de licença antes do primeiro release.

## Fontes

- Licença do k6: https://github.com/grafana/k6/blob/master/LICENSE.md
- Constant arrival rate: https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/constant-arrival-rate/
- Dropped iterations: https://grafana.com/docs/k6/latest/using-k6/scenarios/concepts/dropped-iterations/
