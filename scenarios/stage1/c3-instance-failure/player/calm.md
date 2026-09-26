# Um mês depois

O checkout ficou rápido e ninguém mais foi cobrado sem pedido. Para fechar o mês no azul, o
fundador pediu para cortar custos, e a Nexora voltou para **uma instância** do app.

O código continua sendo o do seu workspace:

    {{workspace}}

Ele tem a sua correção do C2 (duas transações curtas, pools que cabem) e a topologia enxuta.

## Suas ferramentas

| Ferramenta | Para quê |
|---|---|
| {{status}} | sinais de produção agora: tráfego, erros, latência, CPU |
| {{support}} | o que os clientes estão reclamando |
| {{lab}} | experimentos de carga contra a produção, sem nota (dá para derrubar uma instância no meio) |
| {{shell}} | terminal dentro da rede da Nexora (`psql`, `curl`, `top`…) |
| {{deploy}} | coloca o workspace em produção: código, migrações, topologia e `nginx/default.conf` |
| {{check}} | aplica o seu código e mede por ~45 s (rápido, indicativo) |
| {{test}} | o teste de produção que vale (≈ 5 min) |
| {{hint}} | uma dica por vez, quando travar |

**Orçamento:** até **US$ 100/mês** (hoje: US$ 70). {{architecture}} mostra o que roda e quanto
custa; {{docs}} explica cada componente.

**Os clientes são insistentes:** se o checkout falha, eles clicam de novo um segundo depois. O
navegador reenvia o mesmo pedido, com a mesma `Idempotency-Key`. O gateway de pagamento respeita
essa chave; o seu app, você vai descobrir.

## Três pedidos do fundador (para você conhecer a casa)

1. Quantas instâncias do app atendem a loja agora, e o que acontece se uma cair?
2. O que o gateway de pagamento faz com duas cobranças com a mesma `Idempotency-Key`?
   (`API.md` e {{docs}} ajudam.)
3. Onde o app decide se um checkout é um pedido novo?

Quando terminar, {{next}}.
