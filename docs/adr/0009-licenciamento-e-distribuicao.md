# ADR-0009: Licenciamento e distribuição — gratuito, código fechado, doação

**Status:** Aceito (política de dependências: Proposto)
**Data:** 2026-09-24
**Decisores:** Lucas Zanetti

## Contexto

Decisão do produto: o jogo **não** será open source, **não** será pago, e terá opção de doação. Isso afeta quais dependências podem entrar, como distribuir e o que o jogador pode fazer com o código da Nexora que ele edita.

## Decisão

1. **Plataforma (sesim):** proprietária, distribuída gratuitamente como binário. Termos de uso simples (EULA curta) no primeiro release.
2. **Conteúdo editável (app Nexora, scripts de cenário visíveis):** o jogador pode ler, modificar e publicar o próprio workspace (portfólio). A licença do conteúdo será definida no primeiro release, preferencialmente permissiva para o workspace, porque o portfólio é um objetivo do produto (JTBD 5).
3. **Doação:** link externo (GitHub Sponsors, Ko-fi, Open Collective… a decidir). Nenhum fluxo de pagamento dentro do produto; nenhuma feature condicionada a doação.
4. **Política de licenças de dependências:**

| Tipo | Regra | Exemplos |
|---|---|---|
| Permissiva (MIT, BSD, Apache-2.0, ISC, PostgreSQL) | Pode ser linkada no binário | cel-go, coder/websocket, pgx, xterm.js, React Flow, uPlot, Prometheus client, modernc sqlite (verificar) |
| Copyleft fraca (MPL-2.0, EPL-2.0) | Pode ser usada sem modificação; mudanças no arquivo/módulo seriam publicadas | ELK.js (EPL-2.0; verificar) |
| Copyleft forte / rede (GPL, **AGPL**) | **Nunca linkada nem modificada.** Só como programa/imagem separado, sem modificação, via CLI/rede | k6 (AGPL-3.0), Grafana (AGPL-3.0) |
| Source-available (SSPL, RSAL, BSL) | Evitar | Redis 8 (RSALv2/SSPL/AGPL) → usar **Valkey** (BSD-3) no catálogo |

5. Verificação automática de licenças de dependências Go e npm no CI (Chunk 2).

## Opções consideradas

| Dimensão | **Fechado + gratuito + doação** | Open source (AGPL/Apache) | Pago |
|---|---|---|---|
| Controle do produto | Total | Compartilhado | Total |
| Alcance e confiança | Médio (binário fechado com acesso ao Docker) | Alto | Baixo |
| Contribuição de cenários | Não | Sim | Não |
| Receita | Incerta | Incerta | Direta |

## Consequências

- O k6 e qualquer ferramenta AGPL ficam sempre como container separado (ADR-0006).
- Confiança: rodar um binário fechado com acesso ao Docker exige transparência → threat model publicado, releases assinados e checksums (ações do Chunk 2).
- O brief §39 (core / SDK / cenários / infra) continua como separação interna de módulos, e permite abrir partes no futuro (ex.: formato de cenário) sem reestruturar.
- Isto não é parecer jurídico; revisar licenças antes do primeiro release público.

## Ações

1. [ ] Scanner de licenças no CI (Chunk 2).
2. [ ] Escolher a plataforma de doação e a licença do conteúdo do workspace antes do primeiro release.
3. [ ] Definir onde ficam releases e issues públicos (repo público só de releases?).

## Fontes

- Licença do k6: https://github.com/grafana/k6/blob/master/LICENSE.md
- Relicenciamento do Grafana para AGPLv3: https://grafana.com/blog/grafana-loki-tempo-relicensing-to-agplv3/
- Licenças do Redis: https://redis.io/legal/licenses/
