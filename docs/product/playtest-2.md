# Playtest #2 — campanha do Stage 1 no LerdOS

> Protocolo para conduzir as sessões. Hipóteses em [`mvp.md`](mvp.md) §6; metas em §7. O formato segue o [Playtest #1](playtest-1.md): o que mudou está nesta página.
> O Playtest #1 validava o loop no terminal. Este valida **a campanha inteira (C1 → C2 → C3) no navegador** e traz dados para duas decisões: **Stage 2** e **modo hospedado** (§8).
> Os achados do Playtest #1 entram aqui quando forem registrados. Se mudarem o fluxo ou o conteúdo do C1, ajuste este protocolo antes da primeira sessão.

## 1. O que queremos aprender

| Hipótese | Pergunta prática | Como medimos |
|---|---|---|
| H1 | A pessoa chega ao PASS no C1, dentro da sessão, usando dicas até o nível 3? | `scenario_completed`, `hint_requested`; cronômetro |
| H2 | O incidente motiva ou paralisa, agora com Pulse, Terminal e Suporte à mão? | tempo até o primeiro app aberto depois do incidente; pergunta 4 |
| H3 | Depois de cada debrief, a pessoa explica o porquê e cita uma alternativa? | entrevista por cenário (§5), rubrica |
| H4 | O Quick Check continua rápido o bastante? | `quick_check_run.total_ms` |
| H5 | O veredito se mantém nas máquinas das pessoas? | `production_test_run.db_probe_ms` e `slow_host` (ADR-0021) por participante |
| H6 | A pessoa segue sozinha para o C2 e o C3? | `scenario_advanced` e `scenario_completed` nos 7 dias depois da sessão |
| H7 | O setup na máquina da pessoa é aceitável? | cronômetro do clone ao LerdOS aberto; falhas |
| U1 | Os apps do LerdOS são encontráveis? | ordem dos apps abertos (folha de observação) |
| U2 | O código levado de um cenário para o outro ajuda ou confunde? | uso do `--canonical`/"estado canônico"; pergunta 6 |

## 2. Participantes

Mesma persona e mesmas exclusões do Playtest #1 (§2), de **5 a 8 pessoas**. Quem jogou o Playtest #1 não entra: já conhece o C1.

## 3. Preparação

### Máquina

**Linux** com Docker Engine 29+, pelo menos 8 GB de RAM livres e 4 cores. Docker Desktop (macOS, Windows) ainda não funciona: os endereços da loja são IPs dos containers.

- **Na máquina da pessoa**, quando ela tiver Linux: é o que mede H5 e H7 de verdade. Cronometre o setup.
- **Na máquina do facilitador**, quando não tiver. Anote o motivo (sistema, Docker, memória): é dado para o modo hospedado.

**Energia:** notebook **na tomada** e perfil de energia **equilibrado** ou **desempenho**. Na bateria ou em economia de energia, a CPU pode cair a um quarto da velocidade e mudar vereditos (ADR-0021). O jogo avisa antes do Production Test, mas é melhor não precisar: confira antes de a pessoa sentar.

### Antes de cada participante

```bash
bin/sesim stop --down            # desliga a loja da pessoa anterior
rm -rf workspace                 # workspace dela (guarde workspace/.sesim/events.jsonl antes, com consentimento)
bin/sesim sandbox gc --all       # garante que nenhum sandbox ficou para trás
just images && just build        # imagens e bin/sesim atualizados
```

### O que entregar à pessoa

> Você vai jogar um jogo sobre engenharia de software. Rode `bin/sesim ui` e siga o que aparece no navegador.
> Pense em voz alta: diga o que está vendo, o que acha que está acontecendo e o que vai tentar.
> Pode usar a internet como usaria no trabalho. Não existe resposta errada: estamos testando o jogo, não você.

O código é editado no editor que a pessoa preferir, apontado para `workspace/`.

## 4. Roteiro

A campanha não cabe em 90 minutos. Por isso ela é dividida em duas sessões com um intervalo em casa:

| Quando | O quê |
|---|---|
| **Sessão A** (≤ 90 min, presencial) | 0–5 min: consentimento (§7) e o texto acima. 5–70 min: C1 no LerdOS, pensando em voz alta. 70–85 min: entrevista do C1 (§5). 85–90 min: explicar que o C2 e o C3 ficam abertos em casa e combinar a sessão B. |
| **Em casa** (7 dias) | A pessoa segue pelo app Missões se quiser. Não mande lembrete: a vontade de voltar é o que a H6 mede. |
| **Sessão B** (≤ 30 min) | Entrevista do C2 e do C3 (só dos cenários que ela jogou) e perguntas finais. Recolha o log com consentimento. |

As regras do facilitador são as do Playtest #1 (§4). Três acréscimos:

- **Ambiente quebrado:** ajude e anote como bug se a sandbox não subir, se o terminal não conectar ou se um app mostrar erro. A dica do jogo é a Tabby: *"O jogo tem um jeito de te ajudar, se você quiser."*
- **Tempo:** aos 65 min sem PASS, encerre e vá para a entrevista. A pessoa pode continuar em casa, e isso também conta para a H6.
- **INCONCLUSIVE que menciona a máquina:** anote como problema de ambiente, não do jogador.

### Folha de observação (sessão A)

| Marco | Horário | Nota |
|---|---|---|
| LerdOS aberto (fim do setup) | | tempo desde o clone, se foi na máquina dela |
| Primeiro app aberto | | qual |
| Incidente começa | | |
| Primeiro app aberto depois do incidente | | qual (Pulse? Suporte? Terminal?) |
| Primeira hipótese dita em voz alta | | qual? |
| Primeira hipótese **correta** (o banco / a busca) | | |
| Momento de maior frustração | | o que estava fazendo |
| Primeira mudança no workspace | | o que mudou |
| Cada pedido de dica à Tabby | | nível e por quê |
| Cada Quick Check / Production Test | | resultado; aviso de máquina lenta? |
| Uso do Lab, do Git ou do Deploy | | para quê |
| PASS | | |

Anote **frases literais** de confusão, principalmente sobre onde fica cada coisa no LerdOS (U1).

## 5. Entrevista

### H3 — por cenário, depois do debrief dele

| Cenário | Perguntas |
|---|---|
| C1 | 1. *"Por que a sua mudança resolveu o problema?"* 2. *"Cite outra solução que também passaria. O que ela custa?"* |
| C2 | 1. *"Por que uma conexão presa durante o pagamento causava erro em páginas que nem usam pagamento?"* 2. *"Por que diminuir o pool não devolvia o dinheiro dos clientes?"* |
| C3 | 1. *"Qual a diferença entre voltar rápido depois da queda e não parar?"* 2. *"Por que tentar de novo podia cobrar duas vezes, e o que impede isso?"* |

A rubrica é a do Playtest #1 (§5): **correta** (mecanismo certo), **parcial** (nomeia a solução sem o mecanismo) ou **incorreta**. Se a pessoa não chegou ao PASS, pergunte sobre o que ela acha que resolveria.

### Perguntas finais (sessão B)

3. *"Mostre no app Git o que você mudou no C1. O que você mudaria de novo?"* (U2: ela reconhece a própria mudança?)
4. *"Qual foi o momento mais frustrante? De 1 a 5, quão perdido(a) você se sentiu quando o incidente começou?"*
5. *"Se existisse um Stage 2 amanhã, você começaria?"* (sim / talvez / não, e por quê)
6. *"O C2 começou com o seu código do C1. Isso ajudou, atrapalhou ou tanto faz?"*
7. *"Você jogaria isso num site, sem instalar nada? E instalando, como hoje?"* (modo hospedado)

## 6. Dados

- **Log de eventos:** `workspace/.sesim/events.jsonl`. Desde o Playtest #1, ele ganhou:
  - `scenario_advanced`, `lab_run` e `git_commit`;
  - a velocidade da máquina no `production_test_run` (`db_probe_ms`, `slow_host`).

  Continua sem código, SQL, texto digitado, caminhos ou identidade (`docs/architecture/observability.md`). Mostre o arquivo à pessoa antes de pedir para guardar.
- **Relatórios de teste:** `.runs/*-eval.json`.
- Guarde por participante (ex.: `playtest-2/P1/`), fora do repositório. Nada de gravação de tela ou áudio sem consentimento separado.

## 7. Consentimento

O texto é o do Playtest #1 (§7), com uma frase a mais: *"O log pode incluir quão rápida a sua máquina é: é um número, sem nada que identifique você ou a máquina."*

## 8. Metas e decisões

As metas são as de [`mvp.md`](mvp.md) §7, avaliadas sobre as sessões. Os limiares novos abaixo são **propostos**, não medidos.

| Métrica | Meta | Fonte |
|---|---|---|
| Conclusão do C1 na sessão A | ≥ 60 % | `scenario_completed` |
| Tempo até a primeira hipótese (C1) | mediana ≤ 15 min | folha de observação |
| Explicação correta (H3) | ≥ 70 % das respostas, por cenário | rubrica |
| Seguiu para o C2 sozinha (H6) | ≥ 50 % | `scenario_advanced` |
| Terminou o Stage 1 em 7 dias | registrar; sem meta | `scenario_completed` do C3 |
| Production Tests em máquina lenta (H5) | 0 nas máquinas preparadas; registrar nas outras | `slow_host` |
| Setup na máquina da pessoa (H7) | ≤ 10 min; ≤ 1 falha em 8 | cronômetro |

### Stage 2

- **Go:** as metas de C1 e H3 batem, pelo menos metade segue para o C2, e ninguém trava no mesmo ponto do LerdOS.
- **Ajustar antes:** as pessoas travam num mesmo cenário ou app. Primeiro conteúdo e UI, não cenário novo.
- **Repensar:** o PASS vem com dica 5 e sem explicação (H3), como no critério do Playtest #1.

### Modo hospedado

Registre, por participante:

- se o jogo rodou na máquina dela;
- se não rodou, o motivo (sistema, Docker, memória, máquina lenta);
- a resposta à pergunta 7.

| Sinal | Leitura |
|---|---|
| Metade ou mais não consegue jogar na própria máquina (sem Linux, Docker ou memória) | o local-first está barrando a persona: priorizar o modo hospedado, ou Docker Desktop, antes do Stage 2 |
| O jogo roda, mas `slow_host` aparece em máquinas comuns | a calibração é exigente demais para a persona: revisar os limites antes de hospedar |
| Todas rodam localmente e preferem instalar | adiar o modo hospedado; o Stage 2 vem primeiro |

A decisão e os números reais vão para um documento de resultados, como o Playtest #1, e para o roadmap (`mvp.md` §8).

## 9. Limitações conhecidas desta versão

- Só Linux (ver §3).
- Um sandbox que a CLI subiu não aparece na UI já aberta: use só a UI durante a sessão (ADR-0020).
- O C2 e o C3 foram calibrados a partir do início canônico. Uma solução válida do C1 diferente das referências pode encontrar outra física no C2 (ADR-0020). Se isso acontecer, anote a solução.
- Os textos técnicos do relatório (`rule`, `reason`) estão em inglês; o resto está em PT-BR.
