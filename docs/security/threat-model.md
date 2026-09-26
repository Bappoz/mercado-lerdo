# Threat Model v0

> Status: Chunk 1. Escopo: **MVP local-first** (ADR-0002). O modo hospedado exige um threat model próprio antes de qualquer implementação. Método: ativos → atores → fronteiras → ameaças (STRIDE) → mitigações → riscos residuais. Revisar a cada chunk que toque sandbox, servidor local ou pacotes de cenário.

## 1. Escopo e premissas

- O sesim roda na máquina do jogador, com acesso ao Docker daemon.
- **Acesso ao socket Docker equivale a root no host.** Quem está no grupo `docker` já é root na prática. O sesim não cria esse risco, mas o usa. Recomendamos Docker rootless quando disponível (`sesim doctor` informa o modo).
- O jogador é dono da máquina. Proteger o host *contra o próprio jogador* está fora do escopo. Proteger o host contra **conteúdo** que o jogador executa sem entender (código do cenário, imagens, páginas web) está dentro.

## 2. Ativos

| Ativo | Por que importa |
|---|---|
| A1 Host do jogador (arquivos, credenciais, outros containers) | Comprometimento = dano real fora do jogo |
| A2 LAN do jogador e serviços do host | A sandbox não pode virar ponto de ataque à rede local |
| A3 Controle do Docker daemon | = A1 |
| A4 Integridade da avaliação | A promessa do produto |
| A5 Workspace do jogador | Trabalho dele; não pode ser apagado ou exfiltrado |
| A6 Cadeia de distribuição (binário, imagens, pacotes) | Um binário ou imagem adulterado compromete todos os jogadores |
| A7 Dados de analytics | Privacidade; mesmo sem PII, não sai sem consentimento |

## 3. Atores

| Ator | Motivação | Capacidade |
|---|---|---|
| Jogador curioso | Explorar, "quebrar o jogo", trapacear | Total dentro da sandbox; total no host (é dele) |
| Página web maliciosa aberta no mesmo navegador | Usar o sesim como ponte para o Docker (A3) | Requisições cross-origin, DNS rebinding |
| Dispositivo na LAN | Acessar a UI/API do sesim | Rede |
| Atacante da cadeia de suprimentos | Distribuir malware via binário, imagem ou dependência | Comprometer registry, CI ou dependência |
| Conteúdo hostil dentro da sandbox (futuro: cenário de terceiros; hoje: dependência vendorizada adulterada) | Escapar para o host, alcançar a LAN | Código arbitrário na zona do jogador |

## 4. Fronteiras (ver `docs/architecture/overview.md` §5)

B1 navegador ↔ sesim · B2 sesim ↔ Docker daemon · B3 zona do jogador ↔ zona observadora · B4 sandbox ↔ host/LAN · B5 pacotes ↔ sesim · B6 sesim ↔ internet (releases, registry).

## 5. Ameaças e mitigações

### B1 — Navegador ↔ sesim

| # | STRIDE | Ameaça | Mitigação | Residual |
|---|---|---|---|---|
| T1 | S/E | Página maliciosa faz `fetch` para `127.0.0.1:<porta>` e dispara ações (CSRF) | **Implementado (Chunk 9):** token aleatório de 256 bits por execução do `sesim ui`, entregue no fragmento da URL (`#t=…`, que o navegador nunca envia) e guardado em `sessionStorage`; toda chamada a `/api/` exige `Authorization: Bearer` (comparação em tempo constante); sem cookies. Testes em `internal/ui/server_test.go` | Baixo |
| T2 | S/E | **DNS rebinding** (`evil.com` resolve para 127.0.0.1) | **Implementado (Chunk 9):** toda requisição, inclusive de arquivo estático, exige `Host` ∈ {`127.0.0.1:P`, `localhost:P`}; o resto recebe 403 | Baixo |
| T3 | E | WebSocket do terminal aberto cross-origin (WS não segue CORS) | **Implementado (Chunk 9):** `websocket.Accept` recusa um `Origin` diferente do `Host`; o token vai na primeira mensagem (5 s de prazo), senão a conexão fecha com 1008 | Baixo |
| T4 | I | Dispositivo da LAN acessa a UI | **Implementado (Chunk 9):** `--addr` aceita só 127.0.0.0/8; `0.0.0.0`, IPv6 e nomes são recusados. Não há flag para abrir para a LAN | Baixo |
| T5 | T | XSS na SPA via conteúdo exibido (logs do app, nomes de produto do seed, saída do terminal) | **Implementado (Chunk 9):** React escapa por padrão e não há `dangerouslySetInnerHTML`; o Markdown dos pacotes vira elementos React, sem nunca montar HTML, e só aceita links `http(s)` (`web/src/markdown.test.ts`); CSP `default-src 'self'` com `script-src 'self'` (o `style-src` permite `'unsafe-inline'` porque o xterm injeta estilos); `frame-ancestors 'none'`; fontes empacotadas, sem CDN | Baixo |

### B2 — sesim ↔ Docker daemon

| # | STRIDE | Ameaça | Mitigação | Residual |
|---|---|---|---|---|
| T6 | E | Container com socket Docker montado = escape trivial | **Implementado (Chunk 8):** o tipo `sandbox.Spec` não consegue expressar `privileged`, `pid: host` nem `network: host`; `Spec.Validate` rejeita socket Docker, bind gravável e bind em caminhos sensíveis (testes em `internal/sandbox/spec_test.go`). `just sandbox-check` confere no Docker real: nenhum privilegiado, todos com `CapDrop: ALL`, nenhum `docker.sock` montado | Baixo |
| T7 | T | `topology.yaml` do jogador pede algo perigoso (bind `/`, capability, imagem arbitrária) | **Implementado (Chunk 10a):** o tipo `topology.Topology` só tem tamanhos e números com limites, e a decodificação é estrita (campo desconhecido = erro). Não há como pedir imagem, montagem, rede ou capability; imagens e hardening vêm do catálogo e de `stack.Spec` (`internal/topology/topology_test.go`) | Baixo |
| T7b | T | `nginx/default.conf` do jogador (Chunk 10a) serve respostas falsas ou alcança a telemetria | O nginx não está na rede `telemetry` (ADR-0014), roda sem privilégio e com rootfs read-only. A latência é medida no k6, fora do nginx; a corretude é conferida pelo oráculo, que passa pelo nginx. Uma resposta falsa reprova a corretude; cache de respostas corretas é engenharia legítima. Configuração inválida é barrada por `nginx -t` antes do deploy | Baixo |
| T8 | D | Recursos órfãos acumulam (disco, redes) | **Implementado (Chunk 8):** labels + registro com PID do dono + `sesim sandbox gc` (também chamado no `up`) + `Leftovers` depois do destroy. Testado matando o sesim no meio de um `up` | Baixo |

### B3 — Integridade da avaliação

| # | STRIDE | Ameaça | Mitigação | Residual |
|---|---|---|---|---|
| T9 | T | App mente nas próprias métricas | Evaluator não usa métricas do app (`evaluator.md` §2) | Baixo |
| T10 | T | App falsifica respostas para o oráculo ou pula escritas | Oráculo com dados escritos durante o teste + integridade cruzada observer-svc × probe | Médio (local) |
| T11 | T | Jogador edita SQLite ou relatório | **Aceito no MVP** (sem leaderboard) | Aceito |
| T10b | T | O app forja pedidos pagos ou esconde pedidos perdidos (Chunk 10b) | A integridade é conciliada contra o registro do gateway emulado, fora do alcance do jogador: pedido pago sem cobrança conta como fantasma, cobrança sem pedido como perdido (ADR-0018). O evaluator lê o banco como superusuário, pelo `docker exec` no container do Postgres (não pela rede do jogador), com o prefixo passado como variável do `psql`, nunca concatenado no SQL | Baixo |
| T12b | T | A injeção de falhas (Chunk 10c) vira um meio de executar comandos no container | O único comando é fixo (`kill -ABRT 1`, no container de um componente da lista `scenario.Crashable`, hoje só `app`), vindo do pacote de cenário embutido ou do laboratório; nenhum texto do jogador entra no comando | Baixo |
| T12c | T/E | O app Git (Chunk 10d) vira um meio de executar código ou de ler arquivos fora do workspace | Comandos `git` fixos por `exec`, sem shell; hooks e `core.fsmonitor` desligados por `-c`, diff sem `ext-diff`, identidade fixa; o diff só aceita caminhos listados pelo `git status` e passa o caminho depois de `--`; link simbólico novo é mostrado como alvo, sem ser seguido (`internal/game/campaign_test.go`). Outras opções do `.git/config` do workspace continuam valendo: um workspace recebido de terceiros é tão confiável quanto um repositório qualquer | Baixo |
| T12 | S | Container da zona do jogador fala com o Prometheus ou o k6 e injeta dados | Prometheus e k6 só na rede `obs`; o exporter e o observer-svc (nas duas redes) só expõem as portas necessárias e ignoram endpoints de escrita vindos da rede `play` (o recorder só aceita na interface `obs`) | Baixo |

### B4 — Sandbox ↔ host/LAN

| # | STRIDE | Ameaça | Mitigação | Residual |
|---|---|---|---|---|
| T13 | E | Escape de container via kernel/runtime | Hardening (sem privilégio, `cap_drop: ALL`, `no-new-privileges`, seccomp padrão, não-root, rootfs read-only); runtime atualizado (`doctor` avisa Docker antigo); gVisor opcional depois | **Médio: containers não são uma fronteira forte.** Aceitável porque o MVP roda só conteúdo oficial na máquina do próprio jogador |
| T14 | I/E | Sandbox acessa a LAN ou a internet | Redes `internal: true` (sem rota default) | Baixo |
| T15 | I/E | Sandbox acessa **serviços do host pelo IP do gateway** da bridge (possível mesmo com `internal`) | O sesim escuta só em 127.0.0.1 (inalcançável pelo gateway da bridge); `doctor` lista serviços do host em `0.0.0.0` e avisa; futuro: regra de firewall por rede (Linux) | **Médio**: serviços do host abertos em todas as interfaces ficam alcançáveis |
| T16 | D | Fork bomb, OOM, disco cheio, CPU travada | `pids_limit`, `memory`, `nano_cpus` e `ulimits` em todo container; quota de disco no volume do Postgres (tamanho do seed × fator) e tmpfs com tamanho; timeout de builds e testes | Baixo (disco: médio em drivers sem quota de volume) |
| T17 | I | Workspace montado permite escrita no host | Montagens do workspace sempre read-only; o build escreve em volume | Baixo |

### B5/B6 — Conteúdo e distribuição

| # | STRIDE | Ameaça | Mitigação | Residual |
|---|---|---|---|---|
| T18 | T | Pacote de cenário malicioso | MVP: só pacotes embutidos no binário; carregamento externo desabilitado | Baixo |
| T19 | T | Imagem adulterada no registry | Imagens próprias publicadas pelo CI e referenciadas por **digest**; imagens de terceiros (postgres, nginx, k6, prometheus) também por digest; atualização via PR | Baixo/médio |
| T20 | T | Binário adulterado | Releases com checksums SHA-256 e assinatura (cosign/minisign, a decidir no Chunk 2); build no CI | Médio até a assinatura existir |
| T21 | T | Dependência Go/npm comprometida | Lockfiles; `govulncheck`, `npm audit`/OSV no CI; poucas dependências (ADR-0003); Renovate/Dependabot com revisão | Médio |
| T22 | I | Segredo real vaza no repo (cenário de "segredo vazado" usa segredo falso) | `gitleaks` no pre-commit e no CI; segredos de cenário claramente falsos, com justificativa no `.gitleaksignore` | Baixo |

### Novo ativo do Chunk 8: senhas por execução

| # | STRIDE | Ameaça | Mitigação | Residual |
|---|---|---|---|---|
| T24 | I | As senhas do banco de cada sandbox ficam no registro de execuções no host | Arquivo `0600` em `$XDG_STATE_HOME/sesim/runs/`, apagado no `down`/`gc`; as senhas valem só para aquele sandbox, que não é alcançável fora do host | Baixo |

### Privacidade (A7)

| # | Ameaça | Mitigação |
|---|---|---|
| T23 | Coleta excessiva | Eventos sem código, SQL, texto livre, caminhos ou identidade; nada sai da máquina sem `sesim telemetry export` |

## 6. Riscos residuais aceitos no MVP

1. Containers como fronteira (T13): aceitável porque só roda conteúdo oficial, na máquina do próprio jogador.
2. Serviços do host alcançáveis pelo gateway (T15): mitigado por aviso, não eliminado.
3. Trapaça local (T10/T11): sem impacto sem leaderboard.
4. Binário sem assinatura até o Chunk 2 definir o processo (T20).

## 7. Gatilhos para revisar este documento

- Carregar pacote de cenário que não esteja embutido.
- Qualquer modo hospedado ou multi-usuário.
- Leaderboard ou qualquer comparação entre jogadores.
- Novo componente no catálogo que exija capability (ex.: `NET_ADMIN` para netem).
- Terminal ou editor acessível fora de 127.0.0.1.
