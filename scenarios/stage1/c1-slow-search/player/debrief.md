# Debrief — Slow Search

## O que aconteceu

A busca usava `ILIKE '%termo%'` sem um índice que servisse para "contém". Cada busca lia as
100 mil linhas de `products`. Com 4× mais tráfego, o banco (0,5 CPU) ficou no teto, e **tudo**
ficou lento, não só a busca: a CPU do banco é compartilhada por todas as consultas.

## Outras soluções, medidas neste mesmo teste

Valores medianos de 9 execuções do Production Test (Flash Sale, 60 req/s) na máquina de referência.
Não são da sua máquina.

| Solução | busca p95 | CPU do banco (p90) | Veredito |
|---|---|---|---|
| Código original | 5 s (timeout) | 97 % | FAIL |
| Índice trigram (`pg_trgm` + GIN) | 45 ms | 55 % | PASS |
| Índice em memória no app | 1,9 ms | 22 % | PASS |
| Busca só por prefixo | 11 ms | 25 % | FAIL em corretude |
| Full-text search (`tsvector`) | 13 ms | 38 % | FAIL em corretude |
| Banco 2× maior (Postgres M), busca intocada¹ | 5 s (timeout) | 100 % | FAIL em latência, erros (50 %) e custo (US$ 110) |

¹ 3 execuções, não 9. Dobrar a CPU não tira o trabalho de ler a tabela inteira; só compra tempo, e caro.

- **Trigram:** mantém o banco como fonte da verdade, e a busca sempre vê os dados atuais. Custa
  um índice maior e escritas um pouco mais lentas.
- **Em memória:** tira o trabalho do banco, mas cada instância guarda uma cópia e os resultados
  podem estar até 30 s atrasados (produto novo, estoque). Pode ser ótimo ou inaceitável, dependendo do produto.
- **Prefixo e full-text:** rápidos e errados. Otimização que muda o requisito não é otimização.

## Ressalva de escala

Este banco tem 0,5 CPU e 100 mil produtos, uma versão encolhida de produção para caber no seu
notebook. O mecanismo (ler a tabela inteira vs. usar índice) é o mesmo em escala real; os
números absolutos, não.

## Antes de seguir

Responda para você mesma(o), em duas frases: **por que a sua mudança funcionou**, e **qual outra
solução da tabela também passaria, e com que custo**.
