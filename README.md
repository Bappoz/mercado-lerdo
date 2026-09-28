# Mercado Lerdo

Jogo para aprender engenharia de software operando um sistema **real**: você recebe uma aplicação rodando, vê ela degradar sob carga, investiga com métricas, logs e terminal, muda o código ou a infraestrutura e prova a correção com um teste objetivo. Várias soluções podem estar certas.

> Estado: os três cenários do Stage 1 (**C1 — Busca Lenta**, **C2 — Pressão de Conexões**, **C3 — Queda de Instância**) formam uma campanha jogável ponta a ponta no navegador (**LerdOS**) e no terminal: o seu código segue de um cenário para o outro, e cada cenário avança a história do **Mercado Lerdo** (ADR-0020). Um cenário avulso: `--scenario scenarios/stage1/<cenário>` num workspace novo. Roadmap em [`docs/product/mvp.md`](docs/product/mvp.md) §8.

```bash
just images && just build && bin/sesim ui   # o jogo no navegador; requisitos em docs/development/local-setup.md
bin/sesim play                              # o mesmo jogo no terminal
bin/sesim campaign                          # missões do stage e o resultado de cada uma
bin/sesim advance [--canonical]             # depois do PASS: próximo cenário com o seu código (ou do início oficial)
```

Você é a primeira pessoa de engenharia do **Mercado Lerdo**, o marketplace dos pequenos vendedores (*"Sem pressa. Com quem faz."*), e herda um sistema feito às pressas. O **LerdOS** é o computador da empresa, e cada app é uma forma de agir sobre o sistema real:

| App | Para quê |
|---|---|
| E-mail | a empresa falando com você: o briefing, o incidente, o debrief e, no meio, pedidos, broncas, emergências com prazo (contagem regressiva no topo) e elogios. No terminal: `sesim inbox` |
| Missões | os cenários do stage, o resultado de cada um e o botão de seguir para o próximo |
| Pulse | latência, tráfego, erros e CPU ao vivo |
| Terminal | shell na rede da loja; `psql` abre o banco, já autenticado |
| Testes | Quick Check (indicativo) e Production Test (o que vale) |
| Arquitetura | o `topology.yaml`: componentes, réplicas, tamanhos e custo contra o orçamento |
| Deploy | coloca o workspace em produção (código, migrações, topologia, `nginx/default.conf`) |
| Lab | laboratório de carga: ensaie tráfego (e a queda de uma instância) antes do teste que vale |
| Docs | catálogo de componentes; instalar uma ferramenta é um clique que edita a topologia |
| Suporte | reclamações dos clientes, tiradas do que eles viveram de fato: erros, lentidão, cobrança sem pedido |
| Git | o que você mudou (desde o último commit ou desde o início do cenário), histórico da campanha e commit |
| Perfil | a sua carreira no Mercado Lerdo: nível (Júnior I a Sênior III), experiência, salário, bônus e o extrato do que o salário pagou (dicas extras, experimentos no Lab). No terminal: `sesim profile` |
| Notas | anotações da investigação |
| Configurações | escolha entre três temas e ajuste de cores, moldura, texto, animações e pelagem da Tabby |

A Tabby, a gata do escritório, conduz o treinamento do primeiro dia (ela anda pelo LerdOS mostrando cada app) e depois dá uma dica por vez quando você pede.

O código é editado no seu editor, dentro do workspace.

- Visão do produto: [`docs/product/vision.md`](docs/product/vision.md) · universo e história: [`docs/product/universe.md`](docs/product/universe.md)
- Arquitetura: [`docs/architecture/overview.md`](docs/architecture/overview.md)
- Decisões: [`docs/adr/`](docs/adr/README.md)
- Threat model: [`docs/security/threat-model.md`](docs/security/threat-model.md)
- Setup local: [`docs/development/local-setup.md`](docs/development/local-setup.md)

Gratuito, código fechado. Distribuição e link de doação: a definir (ADR-0009).
