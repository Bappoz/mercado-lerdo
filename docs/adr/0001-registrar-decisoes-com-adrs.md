# ADR-0001: Registrar decisões arquiteturais com ADRs

**Status:** Aceito
**Data:** 2026-09-24
**Decisores:** Lucas Zanetti

## Contexto

O brief exige que as decisões relevantes sejam documentadas e que a documentação acompanhe as decisões reais. O produto também usa ADRs *como mecânica de jogo*, então o repositório deve praticar o que ensina.

## Decisão

- ADRs em `docs/adr/NNNN-titulo-em-kebab.md`, em PT-BR, no formato: Status, Data, Decisores, Contexto, Decisão, Opções consideradas, Análise de trade-offs, Consequências, Ações e Fontes.
- Status possíveis: `Proposto`, `Aceito`, `Rejeitado`, `Substituído por ADR-XXXX`.
- ADR aceito não é editado. Mudança de decisão = novo ADR que substitui o anterior.
- Toda decisão que dependa do estado atual de uma ferramenta cita a fonte oficial consultada e a data.
- O índice fica em [`README.md`](README.md).

## Opções consideradas

| Opção | Prós | Contras |
|---|---|---|
| **ADR em Markdown no repo** | Versionado com o código, revisável em PR, grep-ável | Disciplina manual |
| Wiki / Notion | Edição fácil | Desalinha do código; some quando o repo é clonado |
| Só comentários e commits | Zero custo | Sem visão de alternativas e trade-offs |

## Consequências

- Mais fácil: entender por que algo é como é; onboarding.
- Mais difícil: toda decisão relevante custa um documento.
