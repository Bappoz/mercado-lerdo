# Debrief — Connection Pressure

## O que aconteceu

Eram dois problemas com a mesma raiz: **conexões com o banco são um recurso finito e caro**.

1. **A soma dos pools passava do que o banco aceita.** Três instâncias × 20 conexões = 60, e o
   Postgres aceita 50 (três delas reservadas ao administrador). Enquanto o tráfego era baixo,
   os pools ficavam pequenos. Sob carga eles cresciam, e a conexão de número 48 era recusada
   ("too many clients"). O app devolvia erro. A CPU do banco estava folgada: não faltava
   capacidade de processar, faltava lugar para conectar.
2. **O checkout segurava a transação durante o pagamento.** Ele abria a transação, travava o
   produto, chamava o gateway (2 s, às vezes 4 s) e só então gravava o pedido. A conexão ficava
   presa o tempo todo. Pior: quando o cliente desistia de esperar, o cancelamento desfazia a
   transação **depois** que o gateway já tinha cobrado. O dinheiro saía, e o pedido sumia.

"Subir mais instâncias" piorou as duas coisas: mais pools disputando as mesmas 50 conexões.

## Soluções medidas neste mesmo teste

Mediana de 3 execuções do Production Test (rush, 60 req/s), em 2 rodadas, na máquina de
referência; os intervalos mostram as duas rodadas. Não são da sua máquina.

| Solução | Erros | p95 lista/produto | Pedidos com dinheiro divergente (total do teste) | Custo/mês | Veredito |
|---|---|---|---|---|---|
| Código original (3 × 20) | 4,4–5,4 % | 8–10 ms | 8–12 | US$ 110 | FAIL em erros e integridade |
| Duas transações curtas + pools 3 × 15 | 0,13 % | 3 ms | 0 | US$ 110 | **PASS** |
| Transação mantida sem cancelamento + pools 3 × 15 | 0,09–0,13 % | 114–199 ms | 0 | US$ 110 | **PASS** |
| Só pools 3 × 15 | 0,06–0,19 % | 168–173 ms | 5–13 | US$ 110 | FAIL em integridade |
| Só PgBouncer (transaction, 40 conexões) | 0,06–0,13 % | 55–119 ms | 6–10 | US$ 118 | FAIL em integridade |
| Banco M com 100 conexões | 0,09–0,13 % | 4 ms | 10 | US$ 150 | FAIL em integridade e custo |

## Por que "só infraestrutura" não bastava

Diminuir os pools, colocar um PgBouncer ou comprar um banco maior acabam com os erros de
conexão. Nenhuma dessas mudanças impede que um cliente impaciente cancele uma transação depois
da cobrança: esse bug está no código, e só o código resolve. Por isso o teste confere cada
cobrança do gateway contra os pedidos gravados.

- **Duas transações curtas** (reservar como `pending`, cobrar fora da transação, marcar `paid`):
  a conexão só é usada por milissegundos, e o sistema aguenta muito mais carga com as mesmas 50
  conexões. Custa um estado intermediário (`pending`) que o resto do sistema precisa entender,
  e um plano para pedidos que ficam `pending` se o app cair entre as duas transações.
- **Manter a transação, mas sem cancelamento e com pools que cabem:** passa nesta carga, mas
  cada checkout ainda ocupa uma conexão (e trava o produto) por segundos: veja o p95 das páginas
  subir de 3 ms para até 199 ms, porque elas esperam na fila do pool. Conexões ocupadas ≈ checkouts
  por segundo × duração do checkout: com mais tráfego, as 45 conexões voltam a faltar. É uma
  correção que compra tempo.

## Ressalva de escala

Aqui o gateway é emulado, e o banco tem 0,5 CPU. Em produção de verdade, o padrão para "cobrar e
depois gravar" costuma ir além: chave de idempotência no gateway, uma tabela de *outbox* e um
processo que concilia pagamentos e pedidos, porque um app pode cair entre as duas etapas. O C3
volta nesse assunto.

## Antes de seguir

Responda para você mesma(o), em duas frases: **por que a conexão presa durante o pagamento
causava erro em páginas que nem usam pagamento**, e **por que diminuir o pool não recuperava o
dinheiro dos clientes**.
