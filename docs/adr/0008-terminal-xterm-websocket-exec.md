# ADR-0008: Terminal — xterm.js + WebSocket + `docker exec` com TTY

**Status:** Proposto
**Data:** 2026-09-24
**Decisores:** Lucas Zanetti

## Contexto

O jogador precisa de um shell real na rede da sandbox (psql, curl, ss, top). O brief pede opções maduras, sem emulador de terminal feito do zero. No local-first o jogador também tem o próprio terminal.

## Decisão

- Frontend: `@xterm/xterm` + `@xterm/addon-fit` (os pacotes antigos `xterm-addon-*` foram deprecados).
- Transporte: WebSocket no servidor do sesim (`github.com/coder/websocket`, sucessor mantido de `nhooyr.io/websocket`) → `SandboxDriver.Exec(..., tty=true)` no container **toolbox**, com repasse de resize.
- Autenticação: token de sessão + checagem de `Origin` no handshake (threat model T1–T3).
- Alternativa na CLI: `sesim shell` faz o exec direto no terminal do jogador.

## Opções consideradas

| Dimensão | **xterm.js + WS + exec** | `ttyd` dentro da sandbox | Servidor SSH no sesim (ex.: Charm `wish`) |
|---|---|---|---|
| Controle de auth | Total no sesim | No ttyd, porta exposta | Total, chaves SSH |
| Superfície | Um endpoint WS | Mais um serviço exposto | Porta SSH local |
| Terminal próprio do jogador | Via `sesim shell` | Não | **Sim** (atrativo para a persona) |
| Esforço | Médio | Baixo | Médio |

## Análise de trade-offs

O WS + exec mantém uma única porta, um único ponto de auth e nenhuma porta publicada pela sandbox. SSH é o próximo passo natural para quem prefere o próprio terminal, mas `sesim shell` já cobre isso no local.

## Consequências

- Mais fácil: auditoria (toda sessão passa pelo sesim).
- Mais difícil: tratar backpressure e reconexão do WS.

## Ações

1. [x] Confirmar a API do exec attach e do `coder/websocket` (Chunk 9, 2026-09-25). Conferida pelo `go doc` das versões instaladas: `moby/moby/client` v0.6.0 (`ExecCreateOptions.TTY`, `ExecAttach`, `ExecResize`) e `coder/websocket` v1.8.15.
2. [x] Implementado em `internal/ui/terminal.go` + `internal/sandbox/docker` (`ExecTTY`). A autenticação escolhida é o token na **primeira mensagem**; o `Origin` é checado no handshake (padrão do `websocket.Accept`). Não se usa `@xterm/addon-attach`: o protocolo tem mensagens de controle (resize), então o front fala com o socket diretamente.

## Fontes

- `@xterm/addon-attach`: https://www.npmjs.com/package/@xterm/addon-attach
- `coder/websocket`: https://github.com/coder/websocket · https://coder.com/blog/websocket
