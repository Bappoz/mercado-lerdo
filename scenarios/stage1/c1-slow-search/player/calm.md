# Bem-vinda(o) ao Mercado Lerdo

Você é a primeira pessoa de engenharia do **Mercado Lerdo**, o marketplace dos pequenos vendedores:
artesãs, brechós, produtores do interior. *Sem pressa. Com quem faz.*
Hoje são ~130 usuários por dia. Tudo funciona.

O site foi feito pelo Kleber, um freelancer, com pressa e sem medir nada. Agora ele é seu.

**O sistema é seu.** O código que está rodando é o do seu workspace:

    {{workspace}}

Abra no seu editor. `API.md` descreve o que a API promete.

## Suas ferramentas

| Ferramenta | Para quê |
|---|---|
| {{status}} | sinais de produção agora: tráfego, erros, latência, CPU |
| {{shell}} | terminal dentro da rede do Mercado Lerdo (`psql`, `curl`, `top`…) |
| {{deploy}} | coloca o workspace em produção: código, migrações e topologia |
| {{check}} | aplica o seu código e mede por ~45 s (rápido, indicativo) |
| {{test}} | o teste de produção que vale (≈ 4 min) |
| {{hint}} | uma dica por vez, quando travar |

**Orçamento:** o Otávio, do financeiro, libera até **US$ 100/mês** de infraestrutura (custo modelado; hoje são US$ 70).
O que roda em produção, e quanto custa, está em `topology.yaml` no workspace; {{architecture}} mostra
e {{docs}} explica cada componente.

A API está em {{app_url}} e o Prometheus em {{prometheus_url}}
(endereços dentro da sua máquina, válidos enquanto a loja estiver rodando).

## Como uma mudança chega à produção

- **Código Go:** edite no workspace. {{deploy}}, {{check}} e {{test}} recompilam e reiniciam o app.
- **Banco de dados:** crie uma migração nova em `migrations/` (ex.: `0002_minha_mudanca.sql`).
  Elas são aplicadas em ordem a cada deploy.
- **Infraestrutura:** `topology.yaml` (componentes, réplicas, tamanhos) e `nginx/default.conf`.
  Também entram no deploy.
- **Mexer direto no `psql` serve para investigar, não para entregar:** o {{test}} recria o
  banco do zero a cada execução, e só o que está no workspace sobrevive.

## Três pedidos do Álvaro, o fundador (para você conhecer a casa)

Não precisa responder a ninguém: são só para você se ambientar antes do movimento.

1. Quantos pedidos o Mercado Lerdo já recebeu?
2. Qual é o p95 da busca agora?
3. Onde fica o código que responde `GET /search`?

Quando terminar, {{next}}.
