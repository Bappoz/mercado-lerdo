# Bem-vinda(o) à Nexora

Você é a primeira pessoa de engenharia de um marketplace pequeno.
Hoje são ~130 usuários por dia. Tudo funciona.

**O sistema é seu.** O código que está rodando é o do seu workspace:

    {{workspace}}

Abra no seu editor. `API.md` descreve o que a API promete.

## Suas ferramentas

| Comando | Para quê |
|---|---|
| `sesim status` | sinais de produção agora: tráfego, erros, latência, CPU |
| `sesim shell` | terminal dentro da rede da Nexora (`psql`, `curl`, `top`…) |
| `sesim check` | aplica o seu código e mede por ~45 s (rápido, indicativo) |
| `sesim test` | o teste de produção que vale (≈ 4 min) |
| `sesim hint` | uma dica por vez, quando travar |

O Prometheus está em http://127.0.0.1:9090 e a API em http://127.0.0.1:8080.

## Três pedidos do fundador (para você conhecer a casa)

1. Quantos pedidos a Nexora já recebeu?
2. Qual é o p95 da busca agora?
3. Onde fica o código que responde `GET /search`?

Quando terminar, rode `sesim next`.
