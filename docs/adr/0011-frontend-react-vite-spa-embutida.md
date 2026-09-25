# ADR-0011: Frontend — React + Vite (SPA) embutida no binário

**Status:** Proposto
**Data:** 2026-09-24
**Decisores:** Lucas Zanetti

## Contexto

A UI roda em `127.0.0.1`, servida pelo próprio sesim. Não há SEO, SSR nem backend JS. As peças principais são um terminal, gráficos ao vivo em alta frequência, um grafo de arquitetura e telas de briefing, resultado e debrief.

## Decisão

- **React + Vite + TypeScript**, build estático embutido no binário Go com `embed.FS`.
- Bibliotecas-alvo (versões fixadas no Chunk 9):

| Necessidade | Escolha | Por quê |
|---|---|---|
| Terminal | `@xterm/xterm` | ADR-0008 |
| Gráficos ao vivo | **uPlot** | Canvas, pequeno e rápido para streaming; libs SVG por ponto engasgam |
| Grafo de arquitetura | **React Flow (`@xyflow/react`)** + **ELK.js** para layout automático | Grafo derivado do `topology.yaml`; arestas customizadas para animar o fluxo |
| Dados do servidor | TanStack Query + SSE/WebSocket para dados ao vivo | Cache e revalidação sem reinventar |
| Estado de UI | Zustand (ou só `useReducer`) | Mínimo |
| Componentes/estilo | Tailwind + primitivos Radix (via shadcn/ui) | Acessibilidade sem reescrever |
| Markdown de briefing/debrief | Renderizador com sanitização | Threat model T5 |

## Opções consideradas

| Dimensão | **React + Vite** | Next.js | SvelteKit / Solid |
|---|---|---|---|
| Necessidade de SSR/servidor JS | Nenhuma | Duplica o servidor Go | Opcional |
| Ecossistema das peças acima | Completo | Completo | Parcial (React Flow é React) |
| Build embutível como estático | Direto | `output: export`, com limitações | Direto |

## Consequências

- Mais fácil: um único binário; zero CORS (mesma origem).
- Mais difícil: dois toolchains no repo (Go + Node) → `justfile` unifica (Chunk 2).
- A UI só começa no Chunk 9, depois do Playtest #1 no terminal.
