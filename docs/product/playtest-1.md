# Playtest #1 — C1 (Slow Search), só no terminal

> Protocolo para conduzir as sessões. Hipóteses em [`mvp.md`](mvp.md) §6; metas em §7.
> Este playtest valida o **loop** (investigar → mudar → medir), não a UI: ainda não há UI web.

## 1. O que queremos aprender

| Hipótese | Pergunta prática | Como medimos nesta sessão |
|---|---|---|
| H1 | A pessoa chega ao PASS em 30–90 min usando dicas até o nível 3? | `scenario_completed` e `hint_requested` no log; cronômetro |
| H2 | "Produção degradada. Investigue." motiva ou paralisa? | tempo até o primeiro comando depois do `next`; pergunta de frustração |
| H3 | Depois do debrief, a pessoa explica o porquê e cita uma alternativa? | entrevista (§5), com rubrica |
| H4 | O Quick Check é rápido o bastante para iterar? | `quick_check_run.total_ms` no log |
| H6 | A pessoa quer o próximo cenário? | pergunta direta ao final |
| H7 | O setup é aceitável? | cronômetro do clone ao briefing; falhas |

## 2. Participantes

- **3 a 5 pessoas** da persona: fim de curso de Computação/Engenharia de Software, programam, sabem SQL básico e **nunca fizeram plantão nem tuning de banco**.
- Não convidar: quem conhece o projeto, quem já trabalhou como DBA/SRE.
- Uma pessoa por sessão, com um facilitador (você). Sessões de no máximo 90 min.

## 3. Preparação

### Máquina

Linux ou macOS com Docker Engine 29+ (ou Docker Desktop), ≥ 8 GB de RAM livres e ≥ 4 cores. Go 1.27 e `just`, conforme [`../development/local-setup.md`](../development/local-setup.md).

A **mesma máquina do facilitador** é o caminho mais controlado para a primeira rodada. Se a sessão for na máquina da pessoa, cronometre o setup (H7).

### Antes de cada participante

```bash
bin/sesim stop                   # se houver tráfego de uma sessão anterior
rm -rf workspace                 # workspace da pessoa anterior (guarde o events.jsonl antes, se houver consentimento)
just build                       # bin/sesim atualizado
```

Não é preciso resetar o banco: `sesim play` num workspace novo parte do estado original do cenário.

### O que entregar à pessoa

Só isto, impresso ou na tela:

> Você vai jogar um jogo de terminal sobre engenharia de software. Rode `bin/sesim play` e siga o que ele diz.
> Pense em voz alta: diga o que está vendo, o que acha que está acontecendo e o que vai tentar.
> Pode usar a internet como usaria no trabalho. Não existe resposta errada: estamos testando o jogo, não você.

Use o editor que a pessoa preferir, apontado para `workspace/`.

## 4. Roteiro da sessão (≤ 90 min)

| Tempo | O quê |
|---|---|
| 0–5 min | Consentimento (§7) e o texto acima. Liga o cronômetro. |
| 5–70 min | Jogo, pensando em voz alta. Você **observa e anota**; não ajuda. |
| 70–85 min | Entrevista (§5). |
| 85–90 min | Duas perguntas finais (§5) e pedido para compartilhar o log. |

**Regras do facilitador**

- Se pedirem ajuda: *"O jogo tem um jeito de te ajudar, se você quiser."* (é o `sesim hint`). Nada além disso.
- Se o **ambiente** quebrar (Docker, build da plataforma, comando que trava sem mensagem), aí sim ajude e anote como bug. É a pergunta "é o jogo ou sou eu?".
- Se a pessoa travar por mais de 20 min sem pedir dica, pergunte: *"O que você está tentando descobrir agora?"*. Não sugira nada.
- Aos 65 min sem PASS, encerre o jogo e vá para a entrevista. A sessão não passar também é resultado.

### Folha de observação

| Marco | Horário | Nota |
|---|---|---|
| Primeiro comando | | |
| `sesim next` (incidente começa) | | |
| Primeira hipótese dita em voz alta | | qual? |
| Primeira hipótese **correta** (o banco / a busca) | | |
| Momento de maior frustração | | o que estava fazendo |
| Primeira mudança no workspace | | o que mudou |
| Cada `sesim hint` | | nível e por quê |
| Cada `sesim check` / `sesim test` | | resultado |
| PASS | | |

Anote também **frases literais** de confusão ("não sei o que é isso", "achei que…"). Valem mais que resumos.

## 5. Entrevista

### H3 — explicação (depois do debrief)

1. *"Por que a sua mudança resolveu o problema?"*
2. *"Cite outra solução que também passaria no teste. O que ela custa?"*
3. *"Por que buscar só pelo começo do nome não seria aceitável aqui?"*

Rubrica (por pergunta): **correta** (mecanismo certo: a tabela inteira era lida a cada busca, e o índice ou a memória evita isso), **parcial** (nomeia a solução sem o mecanismo: "coloquei um índice"), **incorreta**.

Se a pessoa não chegou ao PASS, faça as perguntas mesmo assim, sobre o que ela acha que resolveria.

### H2 e H6

4. *"Qual foi o momento mais frustrante? De 1 a 5, quão perdido(a) você se sentiu logo depois de 'Produção degradada. Investigue.'?"*
5. *"Se o próximo cenário estivesse pronto agora, você começaria?"* (sim / talvez / não, e por quê)

## 6. Dados

- **Log de eventos:** `workspace/.sesim/events.jsonl`. Tem só tipo de evento, horário, fase, nível de dica, tempos de build/teste e vereditos. **Não tem** código, SQL, texto digitado, caminhos pessoais nem identidade. Mostre o arquivo à pessoa antes de pedir para guardar.
- **Relatórios de teste:** `.runs/*-eval.json`. Mesma natureza.
- Guarde por participante (ex.: `playtest-1/P1/`), fora do repositório.
- **Não grave tela nem áudio** sem consentimento explícito e separado.

## 7. Consentimento (ler em voz alta)

> Estamos testando um protótipo de jogo. Vou observar e anotar o que você faz e diz. No fim, peço para guardar um arquivo de log que registra só quais comandos do jogo você usou e quando, sem seu código nem nada pessoal. Você pode parar a qualquer momento, e pode pedir para apagar suas anotações e o log depois.

## 8. Metas e decisão (go / no-go)

Metas de [`mvp.md`](mvp.md) §7, avaliadas sobre as sessões:

| Métrica | Meta | Fonte |
|---|---|---|
| Conclusão do C1 | ≥ 60 % | `scenario_completed` |
| Tempo até a primeira hipótese | mediana ≤ 15 min | folha de observação |
| Quick Check | mediana ≤ 90 s | `quick_check_run.total_ms` |
| Explicação correta (H3) | ≥ 70 % das respostas 1 e 2 | rubrica |
| "Começaria o próximo" | ≥ 70 % "sim" | pergunta 5 |
| Setup (se na máquina da pessoa) | ≤ 10 min | cronômetro |

- **Go** para o Chunk 8 (sandbox programática) e o Chunk 9 (UI web): conclusão e H3 nas metas, sem bug de ambiente bloqueante.
- **Ajustar antes de seguir:** conclusão abaixo da meta com pessoas travando no mesmo ponto. Primeiro mexer em conteúdo (briefing, dicas, onboarding), não em tecnologia.
- **Repensar:** pessoas chegam ao PASS pedindo a dica 5 e não conseguem explicar o porquê (H3). Nesse caso o jogo está ensinando a copiar, não a investigar.

## 9. Teste prévio com um "jogador cego" (2026-09-25)

Um agente de IA jogou seguindo só o que o jogo mostra, sem acesso ao pacote do cenário nem aos docs. Chegou ao PASS em ~6,5 min (1 `check`, 1 `test`, 0 dicas) e não encontrou erro de ferramenta.

**Isso não valida a dificuldade:** o agente já conhecia `pg_trgm`, e um estudante provavelmente tentaria um B-tree em `name`, veria que não muda nada e travaria ali (o agente estima 30–60 min com 1–3 dicas).

O que ele **validou** foram três problemas de clareza, corrigidos antes do playtest: não estava dito que migrações em `migrations/` são o jeito de mudar o banco; não estava claro que os pedidos do fundador são opcionais; faltava um ponto de partida neutro depois do `next`.

## 10. Limitações conhecidas desta versão

- Só terminal; sem gráficos. O `sesim status` é um retrato de 10 s.
- A fase calma não tem tour guiado: são três perguntas no briefing, e o jogo não confere as respostas.
- `sesim shell` exige um terminal interativo.
- O Quick Check sob incidente pode passar de 90 s quando a mudança cria um índice com o banco saturado (medido: 110 s). Isso é realista, mas afeta a H4.
- O resultado do `sesim test` sai em inglês técnico (tabela), e o resto em PT-BR.
