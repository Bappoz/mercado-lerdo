# Arquitetura — Visão Geral

> Status: Chunk 1, proposta. Decisões em `docs/adr/`. Nada foi implementado nem medido ainda.

## Índice

1. [Forças que moldam a arquitetura](#1-forças-que-moldam-a-arquitetura)
2. [C4 nível 1 — Contexto](#2-c4-nível-1--contexto)
3. [C4 nível 2 — Containers](#3-c4-nível-2--containers)
4. [C4 nível 3 — Componentes do `sesim`](#4-c4-nível-3--componentes-do-sesim)
5. [Fronteiras de confiança](#5-fronteiras-de-confiança)
6. [Fluxos principais](#6-fluxos-principais)
7. [Layout do repositório](#7-layout-do-repositório)
8. [Stack resumida](#8-stack-resumida)
9. [O que o brief pedia e foi simplificado](#9-o-que-o-brief-pedia-e-foi-simplificado)

Documentos relacionados: [`sandbox.md`](sandbox.md) · [`simulation-engine.md`](simulation-engine.md) · [`evaluator.md`](evaluator.md) · [`data-model.md`](data-model.md) · [`observability.md`](observability.md) · [`../security/threat-model.md`](../security/threat-model.md)

## 1. Forças que moldam a arquitetura

| Força | Consequência arquitetural |
|---|---|
| Local-first, sem servidor (ADR-0002) | Um binário único contém CLI, API, UI e orquestração. Nenhum serviço remoto é obrigatório. |
| Medição confiável é a promessa central | Separação física entre a **zona do jogador** e a **zona observadora** (ADR-0013). O evaluator nunca confia em números vindos do app avaliado. |
| Iteração < 90 s | Build incremental do app em container com cache; troca de binário sem reconstruir imagem; reset rápido via template database. |
| Modo hospedado futuro | Toda orquestração passa por uma interface `SandboxDriver`. Nenhum código fora de `internal/sandbox/docker` importa o SDK Docker. |
| Conteúdo é o gargalo | Cenário é um pacote declarativo validado por schema, com um harness que testa o cenário automaticamente (ADR-0010). |
| Código fechado (ADR-0009) | Componentes AGPL (k6) rodam como processos/containers separados e não modificados, nunca linkados ao binário. |

## 2. C4 nível 1 — Contexto

```mermaid
flowchart TB
    player([Jogador<br/>estudante do fim do curso])
    subgraph host["Máquina do jogador"]
        sesim["sesim<br/>(binário local: CLI + servidor + UI)"]
        docker[["Docker Engine"]]
        editor["Editor do jogador<br/>(NeoVim, VS Code…)"]
        ws[("Workspace<br/>repo git no disco")]
    end
    registry[["Registro de imagens<br/>(ghcr.io / Docker Hub)"]]
    releases[["Página de releases<br/>+ link de doação"]]

    player -->|navegador em 127.0.0.1| sesim
    player -->|terminal| sesim
    player --> editor --> ws
    sesim -->|Docker API| docker
    sesim -->|lê/commita| ws
    docker -->|pull por digest| registry
    player -.download do binário.-> releases
```

Não há serviço remoto da plataforma no MVP. As únicas saídas de rede são o pull de imagens (feito pelo Docker) e, opcionalmente, a verificação de nova versão.

## 3. C4 nível 2 — Containers

"Container" aqui é no sentido C4 (unidade executável). Quase todos também são containers Docker.

```mermaid
flowchart LR
    subgraph proc["Processo sesim (host)"]
        api["API HTTP + WebSocket<br/>+ SPA embutida"]
        core["Núcleo: cenário, campanha,<br/>sandbox, evaluator"]
        db[("SQLite + JSONL<br/>+ artefatos")]
    end

    subgraph pz["Zona do jogador (rede play, internal)"]
        nginx["nginx<br/>(edge)"]
        app1["app nexora #1..N"]
        pg[("postgres")]
        tb["toolbox<br/>(alvo do terminal)"]
        tx["toxiproxy<br/>(falhas de rede)"]
        pay["observer-svc<br/>(mock de pagamento)"]
        exp["exporters<br/>(postgres, nginx)"]
        agent["agente de coleta<br/>(prometheus --agent)"]
    end

    subgraph ing["rede ingress"]
        k6["k6<br/>(carga)"]
    end

    subgraph tel["rede telemetry"]
        prom[("prometheus")]
    end

    builder["builder<br/>(go build, network none)"]

    api --- core --- db
    core -->|Docker API| pz & builder
    k6 --> nginx --> app1 --> pg
    app1 -->|via toxiproxy| pay
    exp --> pg
    agent -.scrape.-> app1 & exp
    agent -->|remote write| prom
    k6 -->|remote write| prom
    core -->|OTLP: stats de containers| prom
    core -->|PromQL| prom
```

Pontos de ligação entre as redes, todos intencionais e mínimos (ADR-0013, refinado pelo ADR-0014):

- `nginx`: `play` + `ingress`. É a porta de entrada do tráfego do k6 e **não** está na `telemetry`, porque a configuração dele é do jogador.
- `k6`: `ingress` + `telemetry`. Gera carga e envia as próprias métricas.
- Agente de coleta: `play` + `telemetry`. É imagem nossa, faz scrape na `play` e só escreve no Prometheus.
- O `observer-svc` conta as cobranças por `order_ref` (evaluator, integridade). O evaluator lê esses registros por fora da `play`, via host (Chunk 6).
- `postgres_exporter` e o *probe* SQL do evaluator acessam o Postgres com um papel **somente leitura** criado pela plataforma.
- As stats de containers vêm do host (Docker API). Nenhum container recebe o socket.

Detalhes de redes, limites e ciclo de vida em [`sandbox.md`](sandbox.md).

## 4. C4 nível 3 — Componentes do `sesim`

```mermaid
flowchart TB
    cli["cli<br/>comandos"]
    server["ui<br/>API + WS + SPA"]
    game["game<br/>regras da partida"]
    campaign["campaign<br/>progresso, ledger,<br/>estado canônico"]
    scenario["scenario<br/>carregar pacote, validar,<br/>timeline"]
    topology["topology<br/>topology.yaml → spec,<br/>custo"]
    sandbox["sandbox<br/>SandboxDriver"]
    dockerdrv["sandbox/docker<br/>moby client"]
    workload["workload<br/>executar k6"]
    telemetry["telemetry<br/>cliente PromQL"]
    evaluator["evaluator<br/>medições, CEL, veredito"]
    store["store<br/>SQLite, migrações"]
    events["events<br/>JSONL de produto"]
    wsp["workspace<br/>git, build do app"]

    cli --> game
    server --> game
    game --> campaign
    campaign --> scenario
    campaign --> store
    scenario --> topology
    scenario --> sandbox
    scenario --> workload
    scenario --> evaluator
    sandbox --> dockerdrv
    evaluator --> telemetry
    evaluator --> workload
    wsp --> sandbox
    campaign --> wsp
    cli & server --> events
```

| Componente | Responsabilidade | Não faz |
|---|---|---|
| `cli` | `sesim doctor`, `play`, `status`, `check`, `test`, `reset`, `shell`, `down`, `gc` | Lógica de negócio |
| `ui` | NexoraOS: API local com token, terminal por WebSocket, SPA embutida (implementado no Chunk 9; ADR-0016) | Regra de jogo; acesso direto ao Docker |
| `game` | Uma partida: sessão no workspace, sandbox, fases, Quick Check, Production Test, dicas, eventos (Chunk 9) | Apresentar (a CLI e a UI apresentam) |
| `campaign` | Estado da campanha, tentativas, ledger, adoção de estado canônico | Executar cenário |
| `scenario` | Carrega e valida pacotes, conduz a máquina de estados e a timeline do cenário | Medir |
| `topology` | Valida `topology.yaml` contra o catálogo, gera a spec da sandbox, calcula custo modelado e o grafo de arquitetura | Falar com o Docker |
| `sandbox` | Interface `SandboxDriver` + implementação Docker | Conhecer cenários |
| `workspace` | Inicializa o repo do jogador, faz commits de checkpoint e dispara o build no container `builder` | — |
| `workload` | Monta e roda o k6 com perfil, taxa e seed; coleta o resumo | Avaliar |
| `telemetry` | Consultas PromQL com janela de tempo | Guardar histórico longo |
| `evaluator` | Medições → objetivos (CEL) → veredito por dimensão → relatório | Confiar em métricas do app |
| `store` | SQLite (ADR-0012) | — |
| `events` | Eventos de produto em JSONL | Enviar para a rede |

## 5. Fronteiras de confiança

```mermaid
flowchart TB
    subgraph T0["Confiável: processo sesim + Docker daemon"]
        sesim
        dockerd[Docker daemon]
    end
    subgraph T1["Semi-confiável: navegador local"]
        browser[SPA em 127.0.0.1]
    end
    subgraph T2["Integridade: zona observadora"]
        k6 & prom & pay[observer-svc]
    end
    subgraph T3["NÃO confiável: zona do jogador"]
        app[app / nginx / postgres / toolbox / builder]
    end
    subgraph T4["Externo"]
        reg[registro de imagens]
        web[outras páginas no navegador]
    end
    browser <-->|token + Origin| sesim
    web -.bloqueado: Host/Origin/token.-> sesim
    sesim --> dockerd --> T2 & T3
    T3 -.somente via nginx / observer-svc.-> T2
    dockerd --> reg
```

| Fronteira | Protege | Mecanismo principal |
|---|---|---|
| B1 navegador ↔ sesim | Controle do Docker a partir de páginas maliciosas (CSRF, DNS rebinding) | Bind em `127.0.0.1`, token por sessão, validação de `Host` e `Origin` |
| B2 sesim ↔ Docker daemon | O host (acesso ao socket = root) | Socket nunca montado em container; sesim é o único cliente |
| B3 jogador ↔ observadora | Integridade da avaliação | Redes separadas; medição no cliente (k6) e no observer-svc; Postgres acessado com papel somente leitura |
| B4 sandbox ↔ host | O host e a LAN | Sem privilégio, `cap_drop: ALL`, `no-new-privileges`, limites de cgroup, redes `internal` |
| B5 pacotes de cenário ↔ sesim | O motor | MVP: só pacotes oficiais embutidos no binário |

Análise completa: [`../security/threat-model.md`](../security/threat-model.md).

## 6. Fluxos principais

### 6.1 Iniciar cenário

```mermaid
sequenceDiagram
    actor J as Jogador
    participant S as sesim
    participant D as Docker
    J->>S: sesim play (ou botão na UI)
    S->>S: carrega pacote, valida pré-condições no workspace
    S->>D: cria redes, volumes, containers (labels dev.sesim.run)
    S->>D: builder: go build do workspace → volume bin
    S->>D: inicia postgres → restaura seed (template DB)
    S->>D: inicia app(s), nginx, observers
    S->>S: health checks → estado READY (fase calma)
    S->>D: k6 de fundo com o tráfego da história
    S-->>J: briefing + dashboard ao vivo
    Note over S: timeline dispara o evento de crescimento
```

### 6.2 Quick Check (iteração)

```mermaid
sequenceDiagram
    actor J as Jogador
    participant S as sesim
    participant D as Docker
    participant P as prometheus
    J->>S: sesim check
    S->>S: git commit de checkpoint do workspace
    S->>D: builder: go build incremental
    S->>D: restart das instâncias do app (rolling se N≥2)
    S->>S: aquecimento 15 s (descartado)
    S->>P: mede 45 s do tráfego de fundo
    S-->>J: métricas indicativas (sem veredito)
```

### 6.3 Production Test

Detalhado em [`evaluator.md`](evaluator.md). Resumo: pausa o tráfego de fundo → preflight → N execuções controladas do k6 com a timeline de falhas → coleta → oráculo + integridade → veredito por dimensão → relatório salvo → retoma o tráfego de fundo.

## 7. Layout do repositório

Proposta para o Chunk 2. Os limites espelham o brief §39 (plataforma, SDK de cenário, cenários oficiais, infraestrutura) mesmo sem abrir o código.

```text
.
├── cmd/sesim/                 # main
├── internal/
│   ├── cli/
│   ├── game/                  # regras de uma partida (CLI e UI usam)
│   ├── ui/                    # NexoraOS: API, WS, SPA de web/dist
│   ├── campaign/
│   ├── scenario/              # pacote, schema, timeline
│   ├── topology/              # catálogo, validação, custo, grafo
│   ├── sandbox/               # interface SandboxDriver
│   │   └── docker/            # única importação de moby/moby/client
│   ├── workspace/
│   ├── workload/              # k6
│   ├── telemetry/             # PromQL
│   ├── evaluator/             # CEL, estatística, relatório
│   ├── store/                 # SQLite + migrações
│   └── events/
├── web/                       # SPA React + Vite (ADR-0011, 0016); web/embed.go embute web/dist
├── nexora/                    # app demo: módulo Go separado, template do workspace
├── scenarios/                 # pacotes oficiais (embutidos via embed.FS)
│   └── stage1/{c1-slow-search,c2-connection-pressure,c3-instance-failure}/
├── images/                    # Dockerfiles: toolbox, builder, runtime, observer-svc
├── schemas/                   # JSON Schema de scenario.yaml e topology.yaml
└── docs/
```

O `nexora/` é um módulo Go separado, com `vendor/` commitado. O workspace do jogador é uma cópia dele e compila offline.

## 8. Stack resumida

| Camada | Escolha | ADR |
|---|---|---|
| Plataforma | Go, stdlib `net/http` (roteamento com padrões), `log/slog` | 0003 |
| App demo | Go, `net/http` + `pgx/v5` (`pgxpool`), SQL explícito, sem ORM | 0004 |
| Orquestração | Docker Engine API via `github.com/moby/moby/client`, atrás de `SandboxDriver` | 0005 |
| Carga | k6 (imagem oficial, não modificada) com executores arrival-rate | 0006 |
| Métricas | Prometheus + postgres_exporter + nginx exporter + cAdvisor ou stats do Docker | 0007 |
| Terminal | xterm.js (`@xterm/xterm`, `@xterm/addon-fit`) + WebSocket (`github.com/coder/websocket`) + `docker exec` com TTY | 0008 |
| Cenários/objetivos | YAML + JSON Schema + CEL (`cel-go`) | 0010 |
| Frontend | React + Vite + TypeScript; React Flow + ELK.js; uPlot | 0011 |
| Persistência | SQLite (`modernc.org/sqlite`, sem cgo) + JSONL + arquivos | 0012 |
| Falhas | Toxiproxy (rede app↔dependência), `docker kill/pause`, `docker update` (CPU/mem ao vivo), `tc netem` (futuro) | 0005 / `simulation-engine.md` |
| Cache no catálogo | Valkey (BSD-3) em vez de Redis 8 (tri-licença RSAL/SSPL/AGPL) | 0009 |

As versões exatas serão fixadas no Chunk 2 pelo lockfile, conferindo com a documentação oficial (context7).

## 9. O que o brief pedia e foi simplificado

| Brief | Aqui | Motivo |
|---|---|---|
| Platform API → Scenario Orchestrator → Sandbox Manager como serviços | Pacotes Go dentro de um processo | Local-first; serviços separados seriam complexidade sem usuário. A interface `SandboxDriver` preserva o ponto de corte para o modo hospedado. |
| Growth / Failure / Security / Business Engines separados | Um **runner de timeline** com tipos de evento (`traffic`, `fault`, `resource`, `message`, `trigger`) | Os quatro "engines" são tipos de evento sobre o mesmo relógio. Separar agora é abstração prematura. |
| Editor de código web | Editor do jogador | ADR-0002 |
| Visualização de arquitetura derivada do ambiente real | Derivada do `topology.yaml` validado + estado dos containers | É a fonte da verdade do que roda |
