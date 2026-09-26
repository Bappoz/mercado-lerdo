# ADR-0017: Topologia — `topology.yaml` com catálogo fechado, custo modelado e deploy por reconciliação

**Status:** Proposto
**Data:** 2026-09-25
**Decisores:** Lucas Zanetti

## Contexto

Até o Chunk 9, a sandbox era fixa: um app, um Postgres de 0,5 CPU e o nginx com a configuração do repositório. O C2 (Connection Pressure) e o C3 (Single Instance Failure) pedem que o jogador mude a infraestrutura, não só o código:

- mais réplicas;
- um pooler;
- `max_connections`;
- `proxy_next_upstream` no nginx.

O ADR-0005 já previa que a spec da sandbox viria de uma topologia. O ADR-0010 pedia catálogo fechado e schema estrito. O `evaluator.md` prevê a dimensão `cost`, alimentada pela topologia.

O jogador também pediu instalar ferramentas de forma fácil, pela documentação, e um jeito explícito de fazer deploy.

## Decisão

1. **`topology.yaml` no workspace** (`internal/topology`):
   - Versionado pelo jogador junto com o código.
   - Componentes do catálogo: nginx, app (1–4 réplicas), postgres e pgbouncer (opcional). Valkey fica de fora até algum cenário precisar.
   - Cada campo é um tamanho (S/M/L) ou um número com limites.
   - A decodificação rejeita campos desconhecidos: um erro de digitação não pode virar o valor padrão sem aviso.
   - O arquivo não tem como expressar imagem, montagem, capability nem rede (threat model T7).
   - Workspaces antigos, sem o arquivo, rodam a topologia padrão.
2. **Configuração do nginx no workspace** (`nginx/default.conf`). O deploy roda `nginx -t` num container descartável antes de tocar em qualquer coisa. Uma configuração inválida é recusada, e a produção não muda.
3. **Custo modelado.**
   - Preço por tamanho e por instância no catálogo (US$/mês, fictício e coerente).
   - A medida `monthly_cost` só pode alimentar a dimensão `cost`, e a dimensão `cost` só aceita essa medida. A validação do pacote de cenário impõe as duas regras.
   - A topologia diz quanto o sistema custa, nunca se ele funciona ("propriedades, não tecnologias", ADR-0010).
4. **Deploy por reconciliação** (`stack.Plan` / `stack.Apply`):
   - Compara a spec derivada da topologia em produção com a do workspace, componente por componente, na ordem de dependência (postgres → pgbouncer → app → nginx).
   - Recria só o que mudou.
   - Recriar o app também recria o nginx, que resolve o upstream uma única vez, ao iniciar.
   - A topologia em produção e o hash da configuração do nginx ficam no registro de execuções.
   - **Não há rolling update.** Uma réplica recriada perde as requisições em andamento: é realista para o MVP e é assunto do C3.
5. **"Instalar" = editar o arquivo.** O botão do app Docs (e o `sesim install`) acrescenta o bloco padrão ao `topology.yaml`, preservando comentários e ordem (`yaml.Node`). Nada muda em produção até o deploy. O jogador vê o que a instalação fez e pode desfazer apagando o bloco.
6. **PgBouncer como imagem própria** (`images/pgbouncer`, Alpine + `apk add pgbouncer`):
   - A configuração é gerada do ambiente num tmpfs.
   - `max_prepared_statements` fica ligado, porque o pgx usa prepared statements de protocolo e eles precisam sobreviver ao modo `transaction`.
   - O jogador inspeciona pelo console `SHOW POOLS`.
7. **C1 ganha a dimensão custo** (pacote 0.2.0):
   - Orçamento de US$ 100/mês; o início custa US$ 70.
   - Uma quinta referência, `bigger-database` (Postgres M, sem mexer na busca), mostra que "comprar CPU" não resolve. Medido no `verify` de 2026-09-25 (3 execuções, 1 rodada):
     - a CPU do banco continua no teto (p90 100%);
     - a busca fica no timeout (p95 5 s);
     - os erros caem de 94% para 50%;
     - e o custo passa para US$ 110, acima do orçamento.

## Opções consideradas

| Dimensão | **Arquivo declarativo + reconciliação** | Comandos imperativos (`sesim scale app 2`) | Compose do jogador |
|---|---|---|---|
| Revisável e versionável | Sim (diff no git) | Não | Sim |
| Superfície de segurança | Catálogo fechado | Pequena | **Grande** (imagens, montagens, privilégios) |
| Reprodutibilidade do teste | O arquivo é a entrada do evaluator | Estado implícito | Sim |
| Custo de implementação | Médio (diff + ordem de dependência) | Baixo | Baixo, mas inseguro |

## Consequências

- **Mais fácil:**
  - C2 e C3 viram configuração, e a tela de Arquitetura é derivada do arquivo, não desenhada à mão;
  - o custo entra no julgamento.
- **Mais difícil:**
  - a CPU de um componente com réplicas passa a ser a soma do uso sobre a soma das quotas (`stack.CPU` e o Pulse);
  - o registro de execuções guarda mais estado;
  - calibração: os tamanhos S são a máquina calibrada do Stage 1, e os tamanhos maiores não foram calibrados.
- **Não resolvido:** rolling update e health-check ativo do nginx ficam para o C3.
