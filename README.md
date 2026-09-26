# Software Engineering Simulator (nome provisório)

Jogo para aprender engenharia de software operando um sistema **real**: você recebe uma aplicação rodando, vê ela degradar sob carga, investiga com métricas, logs e terminal, muda o código ou a infraestrutura e prova a correção com um teste objetivo. Várias soluções podem estar certas.

> Estado: o primeiro cenário (**C1 — Slow Search**) é jogável ponta a ponta no navegador (**NexoraOS**) e no terminal. Roadmap em [`docs/product/mvp.md`](docs/product/mvp.md) §8.

```bash
just images && just build && bin/sesim ui   # o jogo no navegador; requisitos em docs/development/local-setup.md
bin/sesim play                              # o mesmo jogo no terminal
```

**NexoraOS** é o computador da empresa, e cada app é uma forma de agir sobre o sistema real:

| App | Para quê |
|---|---|
| E-mail | recados do fundador: o briefing, o incidente e o debrief |
| Pulse | latência, tráfego, erros e CPU ao vivo |
| Terminal | shell na rede da Nexora, com o `psql` já conectado |
| Testes | Quick Check (indicativo) e Production Test (o que vale) |
| Arquitetura | o `topology.yaml`: componentes, réplicas, tamanhos e custo contra o orçamento |
| Deploy | coloca o workspace em produção (código, migrações, topologia, `nginx/default.conf`) |
| Docs | catálogo de componentes; instalar uma ferramenta é um clique que edita a topologia |
| Notas | anotações da investigação |
| Configurações | escolha entre três temas e ajuste de cores, moldura, texto, animações e pelagem da Tabby |

A Tabby, a mascote, dá uma dica por vez quando você pede.

O código é editado no seu editor, dentro do workspace.

- Visão do produto: [`docs/product/vision.md`](docs/product/vision.md)
- Arquitetura: [`docs/architecture/overview.md`](docs/architecture/overview.md)
- Decisões: [`docs/adr/`](docs/adr/README.md)
- Threat model: [`docs/security/threat-model.md`](docs/security/threat-model.md)
- Setup local: [`docs/development/local-setup.md`](docs/development/local-setup.md)

Gratuito, código fechado. Distribuição e link de doação: a definir (ADR-0009).
