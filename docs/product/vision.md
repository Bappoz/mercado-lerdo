# Visão do Produto

> Status: Chunk 0, revisado após decisões de 2026-09-24. Nome provisório: **Software Engineering Simulator**. A empresa fictícia é o **Mercado Lerdo** (decidido em 2026-09-26; universo em [`universe.md`](universe.md)); o nome final do jogo ainda está aberto.

## 1. Problema

Desenvolvedores aprendem a *escrever* código na graduação, em bootcamps e em plataformas de desafios. Aprendem a *operar, evoluir e raciocinar sobre sistemas em execução* quase só no trabalho, normalmente durante um incidente real, sob pressão, com usuários reais pagando a conta.

Falta **experiência**, não informação. Existe material de sobra sobre índices, connection pools, load balancing e SLOs. O que falta é ter visto um sistema degradar, formulado uma hipótese errada, medido, mudado algo e provado que a mudança funcionou.

As opções práticas existentes cobrem só uma fatia cada:

- **Labs de troubleshooting** (SadServers, Killercoda, iximiuz Labs): máquinas reais, mas **quebra-cabeças isolados e sem estado**, com verificação binária do tipo "consertou / não consertou".
- **Simuladores de system design** (SysSimulator, Cloud Arch Simulator, Server Survival): trade-offs de arquitetura, mas **o sistema é abstrato**. São caixas com capacidade numérica, sem código nem banco reais.
- **Simuladores de incidente** (Uptime Labs): incidentes realistas, mas **vendidos para times (B2B)**, sem trilha individual.

Não encontramos (ver `competitive-landscape.md`) quem combine: **aplicação real rodando**, que o jogador **modifica**, **medida sob carga gerada**, avaliada em **várias dimensões objetivas**, dentro de uma **narrativa persistente** em que decisões anteriores têm consequência.

## 2. Proposta de valor

> Ganhe a experiência de produção que você só teria quebrando produção: em um sistema real, com medições reais, onde várias soluções podem estar certas.

- **Para o jogador:** um lugar seguro para praticar o ciclo observar → hipotetizar → mudar → medir, com evidências do que fez (diffs, relatórios de carga, ADRs, postmortems).
- **Para educadores (futuro):** cenários reproduzíveis e versionados que avaliam *propriedades*, não palavras-chave.

## 3. Modelo de distribuição

- **Gratuito, código fechado, com opção de doação.** Não há monetização por feature. A doação é um link externo (plataforma a definir) e não adiciona nenhum fluxo de pagamento dentro do produto.
- **Local-first no MVP:** um binário roda no Docker do próprio jogador (ver ADR-0002).
- Consequências: comunidade de cenários e marketplace ficam fora do horizonte próximo. O código do app demo (o site do Mercado Lerdo; `nexora` no código) é *conteúdo* visível e editável pelo jogador no workspace dele, mas a plataforma não é aberta. A política de licenças de dependências está em ADR-0009.

## 4. Persona primária (MVP)

### "Ana — estudante do fim do curso"

- Últimos semestres de Engenharia de Software ou Ciência da Computação.
- Já fez projetos de disciplina e talvez estágio. Escreve uma API CRUD, sabe SQL básico e já usou Docker seguindo um README.
- **Nunca** esteve de plantão, nunca leu um query plan sob pressão, nunca rodou teste de carga.
- **Motivação:** conseguir o primeiro emprego ou efetivação em backend/SRE/plataforma, entender o que acontece em "produção" e ter algo concreto para mostrar em entrevista.
- **Restrições:** sessões de 30–90 min entre aulas; notebook com Docker, às vezes com pouca RAM; não paga conta de cloud.
- **Frustrações:** tutorial que diz o que digitar; LeetCode parece desconectado do trabalho; lab de cloud que expira no meio do raciocínio.

**Não é persona do MVP:** iniciante absoluto, que ainda não lê stack trace nem escreve `SELECT`. Aprendizado por descoberta sem conhecimento prévio falha (ver `risks.md`).

### Personas secundárias (orientam a arquitetura, não o escopo)

| Persona | Necessidade | Por que não no MVP |
|---|---|---|
| Professor(a) de disciplina | Atribuir cenários reproduzíveis e ver os artefatos dos alunos | Exige gestão de turma e modo hospedado |
| Dev júnior indo para SRE/plataforma | Cenários mais difíceis e distribuídos | Exige conteúdo do Stage 3 em diante |
| Time de contratação | Ambiente de entrevista | Exige modo hospedado, anti-trapaça e proctoring; é outro produto |

## 5. Jobs-to-be-Done

1. **Quando** leio sobre um conceito (índice, pool, réplica), **quero** ver a falha concreta que ele previne num sistema que eu toco, **para** entender quando usá-lo e quando não usar.
2. **Quando** um sistema está lento ou quebrado, **quero** praticar um método de investigação em vez de chutar, **para** manter a calma no meu primeiro incidente real.
3. **Quando** faço uma mudança, **quero** provar com números que funcionou, **para** confiar no meu julgamento e saber explicá-lo.
4. **Quando** escolho uma solução, **quero** ver o custo e os trade-offs de outras soluções válidas, **para** parar de achar que arquitetura tem "a resposta certa".
5. **Quando** me candidato a uma vaga, **quero** mostrar evidências de habilidade operacional, **para** me destacar sem experiência prévia em produção.

O MVP atende os JTBD 1–3. O JTBD 4 fica parcialmente atendido pelas soluções de referência no debrief. O JTBD 5 fica para depois do MVP.

## 6. Princípios

1. **Sintoma antes de solução.** O jogo mostra efeitos (latência, erros, saturação), nunca "instale X".
2. **Avaliar propriedades, não ferramentas.** Nenhuma verificação pergunta "o Redis está presente?".
3. **Real onde ensina, emulado onde só custa.** Código, SQL, configuração e processos são reais. Escala, geografia e dinheiro são *modelados*, e a UI diz isso.
4. **O loop é o produto.** Só entra o que torna Observar → Hipotetizar → Mudar → Medir mais rápido, claro ou significativo.
5. **Esforço, depois explicação.** O jogador pode travar, mas todo cenário termina com um debrief explícito.
6. **Várias respostas certas, visíveis.** Depois de passar, o jogador vê onde a solução dele fica em relação às soluções de referência.
7. **Números honestos.** Todo número vem de medição real ou de um modelo rotulado como tal.
8. **Não confiável por padrão.** Tudo o que o jogador executa é tratado como hostil à plataforma e ao evaluator.

## 7. Diferenciação

| Eixo | Desafios de código | Labs de troubleshooting | Simuladores abstratos | Simuladores de incidente B2B | **Este produto** |
|---|---|---|---|---|---|
| Sistema real (código + banco rodando) | Parcial (uma função) | Sim | Não | Sim | **Sim** |
| Jogador muda código *e* topologia | Só código | Config/comandos | Só topologia | Comandos | **Ambos** |
| Avaliação sob carga gerada | Não | Raramente | Simulada | Varia | **Carga real, escalada** |
| Objetivos multidimensionais | Não | Não | Sim (simulados) | Parcial | **Sim (medidos)** |
| Mundo persistente com consequências | Não | Não | Por partida | Não | **Sim (campanha)** |
| Mostra soluções alternativas válidas | Editorial | Não | Implícito | Não | **Sim (debrief)** |

Em uma linha: **no simulador abstrato você coloca uma caixa de cache; aqui você liga o cache, invalida certo e prova que o p95 caiu sem servir preço desatualizado.**

## 8. Onde este documento diverge do brief original

1. **"1.000.000 usuários" não roda literalmente** num notebook. Adotamos *física escalada*: em vez de aumentar a carga, reduzimos os recursos (por exemplo, o Postgres com 0,25 CPU). A contagem de usuários é narrativa; a avaliação usa medições reais no tamanho escalado, e a UI mostra o fator de escala.
2. **MVP local-first** (ADR-0002). Sem sandbox hospedada, sem código hostil de terceiros no nosso servidor, sem custo de cloud na fase de validação.
3. **Editor web fora do caminho crítico.** O workspace é uma pasta no disco e o jogador usa o próprio editor.
4. **Segurança não é "Stage 7".** É um fio que atravessa todos os estágios. O Stage 1 já planta um segredo no histórico do git e uma injeção SQL dormente.
5. **Títulos de carreira** (Founding → Principal) ficam como narrativa. O produto não afirma treinar influência organizacional nem roadmap.
6. **Campanha totalmente emergente é armadilha de conteúdo.** Adotamos uma cadeia autorada, workspace herdado entre cenários e um *registro de decisões* (ver `game-loop.md`).
7. **Mentores de IA e correção de postmortem por IA** ficam fora do MVP. As dicas do MVP são autoradas e determinísticas.
