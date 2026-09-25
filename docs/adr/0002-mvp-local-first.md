# ADR-0002: MVP local-first (sandbox no Docker do jogador)

**Status:** Aceito
**Data:** 2026-09-24
**Decisores:** Lucas Zanetti

## Contexto

O MVP existe para validar H0 (`docs/product/mvp.md`). O brief previa sandboxes por jogador numa plataforma hospedada. Isso traz três custos antes de qualquer validação:

1. executar código não confiável de terceiros em infraestrutura nossa, o que exige isolamento forte (microVM) e um threat model maior;
2. custo de cloud por sessão ativa, sem receita (o produto é gratuito com doação, ADR-0009);
3. contas, autenticação, fila e cotas.

A persona (estudante do fim do curso) tem notebook e, em geral, Docker.

## Decisão

O MVP é **um binário local** (`sesim`) que orquestra a sandbox no Docker do próprio jogador e serve a UI em `127.0.0.1`. O modo hospedado fica para depois do Playtest #2, preservado pela interface `SandboxDriver` (ADR-0005).

## Opções consideradas

| Dimensão | **Local-first** | Hospedado (containers) | Hospedado (microVM) |
|---|---|---|---|
| Complexidade | Baixa | Média | Alta |
| Custo | ~0 | Por sessão | Por sessão, maior; bare metal para Firecracker |
| Segurança da plataforma | O jogador roda na própria máquina | Containers insuficientes para multi-tenant hostil | Adequada |
| Fricção do jogador | Precisa de Docker e de recursos | Só navegador | Só navegador |
| Ruído de medição | Varia por máquina | Controlável | Controlável |
| Editor | O do jogador | Precisa de editor web | Precisa de editor web |

## Análise de trade-offs

Troca **fricção de setup e variância de hardware** por **zero custo e zero superfície de ataque hospedada**. A variância é tratada com física escalada, calibração e limiares com folga (`evaluator.md`). A fricção é medida pela H7.

## Consequências

- Mais fácil: começar agora; o jogador usa o próprio editor e terminal.
- Mais difícil: suporte a SOs diferentes (Docker Desktop no macOS e Windows); notebooks fracos; anti-trapaça (aceito).
- A revisitar: se a H7 falhar (setup > 10 min ou muitas falhas), antecipar o modo hospedado.

## Ações

1. [ ] `sesim doctor` com verificações acionáveis (Chunk 2/3).
2. [ ] Definir os requisitos mínimos de máquina depois de medir o consumo (Chunk 5).
