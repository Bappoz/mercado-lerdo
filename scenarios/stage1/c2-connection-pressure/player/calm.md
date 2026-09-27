# Duas semanas depois

A busca voou depois da sua correção. O Mercado Lerdo cresceu, e o Álvaro, com medo de outro
incidente, chamou o Kleber de volta para "aumentar a capacidade". Ele subiu o app para
**3 instâncias**, cada uma com um pool de 20 conexões com o banco. Ninguém mediu se ajudou.

O código continua sendo o do seu workspace:

    {{workspace}}

Ele já tem o índice trigram do C1 e a topologia nova (`topology.yaml`).

## Suas ferramentas

| Ferramenta | Para quê |
|---|---|
| {{status}} | sinais de produção agora: tráfego, erros, latência, CPU |
| {{support}} | o que os clientes estão reclamando |
| {{shell}} | terminal dentro da rede do Mercado Lerdo (`psql`, `curl`, `top`…) |
| {{deploy}} | coloca o workspace em produção: código, migrações e topologia |
| {{check}} | aplica o seu código e mede por ~45 s (rápido, indicativo) |
| {{test}} | o teste de produção que vale (≈ 5 min) |
| {{hint}} | uma dica por vez, quando travar |

**Orçamento:** a infraestrutura pode custar até **US$ 130/mês** (hoje: US$ 110, com as 3 instâncias).
{{architecture}} mostra o que roda e quanto custa; {{docs}} explica cada componente.

**O gateway de pagamento é externo:** ele responde em cerca de 1,5 s, às vezes 4 s. Você não
controla isso. Um cliente espera no máximo 5 s por qualquer página antes de desistir.

## Três pedidos do Álvaro (para você conhecer a casa)

Não precisa responder a ninguém: são só para você se ambientar.

1. Quanto custa a infraestrutura por mês, e o que mudou desde a busca?
2. Qual é o p95 do checkout agora? Por que ele é tão maior que o das outras páginas?
3. Onde fica o código que chama o gateway de pagamento?

Quando terminar, {{next}}.
