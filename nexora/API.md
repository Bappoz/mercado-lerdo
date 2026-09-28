# API do Mercado Lerdo — contrato v1

Este contrato é o que os geradores de tráfego, as verificações de corretude e os
próximos cenários usam. Você pode mudar tudo o que está por trás dele (consultas,
schema, infraestrutura), mas quebrá-lo reprova as verificações de corretude.

Todos os corpos são JSON. Erros têm a forma `{"error": "<mensagem>"}`.

## Catálogo

### `GET /products?page=<n>&limit=<n>`

Produtos mais novos primeiro. `page` ≥ 1 (padrão 1), `limit` de 1 a 100 (padrão 20).

```json
{"page": 1, "products": [{"id": 1, "sku": "NX-000001", "name": "…", "category": "…", "price_cents": 1999, "stock": 12, "created_at": "…"}]}
```

### `GET /products/{id}`

Um produto, incluindo `description`. `404` quando ele não existe.

### `GET /search?q=<termo>&limit=<n>`

- Devolve os produtos cujo **nome contém `q` em qualquer posição**, sem diferenciar maiúsculas e minúsculas.
  `q` é comparado literalmente (`%` e `_` não são curingas).
- Os resultados são ordenados por `created_at`, mais novos primeiro, e depois cortados em `limit` (1 a 100, padrão 20).
- `q` precisa ter de 2 a 64 caracteres depois de tirar os espaços das pontas; senão, `400`.

```json
{"query": "lamp", "products": [ … ]}
```

## Contas

### `POST /auth/login`

```json
{"email": "user00042@nexora.test", "password": "…"}
```

`200` → `{"token": "<opaco>", "expires_in": 86400}`. E-mail ou senha errados → `401`.
Envie o token como `Authorization: Bearer <token>` nas rotas autenticadas.

## Pedidos (autenticado)

### `POST /checkout`

Cabeçalhos: `Authorization: Bearer <token>`, `Idempotency-Key: <texto>` (obrigatório).

```json
{"order_ref": "c0ffee-000123", "items": [{"product_id": 42, "quantity": 2}]}
```

- `order_ref`: de 8 a 64 caracteres, escolhido pelo cliente. Identifica a compra
  de ponta a ponta e é enviado ao gateway de pagamento.
- `items`: de 1 a 20 produtos diferentes, quantidade de 1 a 10 cada.

| Status | Significado |
|---|---|
| `201` | Pedido feito e pago. Corpo: `{"id", "order_ref", "status", "total_cents", "created_at"}` |
| `400` | Requisição inválida ou sem `Idempotency-Key` |
| `401` | Sessão ausente ou inválida |
| `409` | Estoque insuficiente |
| `422` | Produto desconhecido |
| `5xx` | Pedido não foi feito |

Um `201` significa que o pedido está gravado **e** que o cliente foi cobrado exatamente uma vez pelo `order_ref`.

### `GET /orders`

Os 50 pedidos mais recentes de quem chama.

## Operação

| Endpoint | Porta | Para quê |
|---|---|---|
| `GET /healthz` | 8080 | `200` quando o banco responde em até 1 s |
| `GET /metrics` | 9091 | Formato de texto do Prometheus: contagem e latência de requisições por rota, estatísticas do pool do banco |

## Configuração

| Variável | Padrão | Significado |
|---|---|---|
| `NEXORA_DATABASE_URL` | — (obrigatória) | String de conexão do PostgreSQL |
| `NEXORA_PAYMENT_URL` | `http://payments:8090` | URL base do gateway de pagamento |
| `NEXORA_DB_MAX_CONNS` | `10` | Tamanho do pool de conexões por instância |
| `NEXORA_ADDR` | `:8080` | Endereço em que a API escuta |
| `NEXORA_METRICS_ADDR` | `:9091` | Endereço em que as métricas escutam |

## Gateway de pagamento (externo)

O app da loja chama o gateway; ele não faz parte da API do Mercado Lerdo. URL base: `NEXORA_PAYMENT_URL`.

### `POST /charge`

```json
{"order_ref": "c0ffee-000123", "amount_cents": 4990}
```

- Cobra o cliente e responde `201` com a cobrança. Pode levar segundos.
- Respeita um cabeçalho opcional `Idempotency-Key`: a primeira requisição com uma chave
  é cobrada; qualquer requisição seguinte com a mesma chave (mesmo uma enviada enquanto a
  primeira ainda está em andamento) recebe de volta a cobrança original com `200` e não é
  cobrada de novo.
- Sem o cabeçalho, cada requisição é uma cobrança nova.
