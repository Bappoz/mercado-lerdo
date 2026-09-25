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

A API está em {{app_url}} e o Prometheus em {{prometheus_url}}
(endereços dentro da sua máquina, válidos enquanto a Nexora estiver rodando).

## Como uma mudança chega à produção

- **Código Go:** edite no workspace. `sesim check` e `sesim test` recompilam e reiniciam o app.
- **Banco de dados:** crie uma migração nova em `migrations/` (ex.: `0002_minha_mudanca.sql`).
  Elas são aplicadas em ordem pelo `check` e pelo `test`.
- **Mexer direto no `psql` serve para investigar, não para entregar:** o `sesim test` recria o
  banco do zero a cada execução, e só o que está no workspace sobrevive.

## Três pedidos do fundador (para você conhecer a casa)

Não precisa responder a ninguém: são só para você se ambientar antes do movimento.

1. Quantos pedidos a Nexora já recebeu?
2. Qual é o p95 da busca agora?
3. Onde fica o código que responde `GET /search`?

Quando terminar, rode `sesim next`.
