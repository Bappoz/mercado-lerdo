# ADR-0005: Driver de sandbox — Docker Engine API atrás de `SandboxDriver`

**Status:** Proposto
**Data:** 2026-09-24
**Decisores:** Lucas Zanetti

## Contexto

O sesim precisa criar, observar, alterar ao vivo (CPU/mem, kill) e destruir os componentes de um cenário, com limpeza verificável. O MVP é local (ADR-0002); o futuro hospedado vai exigir microVMs.

## Decisão

- Interface `SandboxDriver` (`docs/architecture/sandbox.md` §3) com uma única implementação no MVP: **Docker Engine API via SDK Go `github.com/moby/moby/client`**.
- O módulo `github.com/docker/docker` **não** será usado: foi deprecado a partir do Docker v29 em favor de `github.com/moby/moby/client` e `github.com/moby/moby/api`, e correções de segurança recentes só saem no módulo novo.
- A spec da sandbox é gerada pelo pacote `topology`, não por arquivos compose escritos à mão.

## Opções consideradas

| Dimensão | **Docker Engine API (SDK)** | `docker compose` via CLI | Podman (API compatível) | microVM (Firecracker/Kata) já no MVP |
|---|---|---|---|---|
| Controle fino (labels, update ao vivo, exec TTY, stats) | Total | Parcial (parse de saída) | Quase total | Total, com muito mais código |
| Dependência externa | Daemon Docker | Daemon + plugin compose | Podman | KVM, kernel e rootfs próprios |
| Testabilidade | Interface mockável | Subprocesso | Idem SDK | Baixa |
| Base instalada da persona | Alta | Alta | Média | Baixa (exige Linux + KVM) |
| Isolamento | Namespaces/cgroups | Idem | Idem (rootless por padrão) | Forte |

## Análise de trade-offs

O compose seria mais rápido de começar, mas o sesim precisa de operações que o compose não expõe bem (update ao vivo, kill de réplica específica, exec com TTY por WebSocket, verificação de limpeza por label). O SDK dá controle ao custo de mais código, e esse código fica contido num pacote. A compatibilidade de API do Podman é um bônus a testar, não uma promessa.

## Consequências

- Mais fácil: testes de ciclo de vida; adicionar driver de microVM depois.
- Mais difícil: acompanhar breaking changes do SDK (o v29 mudou structs de opções e nomes de métodos).
- Docker Desktop (macOS/Windows): IPs da bridge não são roteáveis a partir do host. O `Endpoint()` precisa de port-forward em `127.0.0.1`.
- Um `docker-compose.yml` de **desenvolvimento** continua útil para o app demo nos Chunks 3–5, antes do driver existir.

## Ações

1. [ ] Confirmar a API atual do `moby/moby/client` (create, update, exec attach, events) no context7 no Chunk 8.
2. [ ] Teste de limpeza com kill do processo no meio (Chunk 8).
3. [ ] Testar com Docker rootless e com Docker Desktop macOS.

## Fontes

- Depreciação do módulo `github.com/docker/docker`: https://github.com/moby/moby/discussions/52404
- Pacote `moby`: https://pkg.go.dev/github.com/moby/moby
- Redes `internal`: https://docs.docker.com/reference/cli/docker/network/create/
