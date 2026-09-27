# Universo do jogo — Mercado Lerdo

> Bíblia da história: a empresa, as pessoas, o tom e o arco de carreira que as missões seguem.
> Todo texto do jogo (briefings, e-mails, debriefs, eventos, Tabby) segue esta página. Mudou algo aqui, reveja os textos das missões.
> No código, o app continua se chamando `nexora` (módulo Go, imagens, variáveis `NEXORA_*`). O nome da marca só aparece no que o jogador vê.

## 1. A empresa

**Mercado Lerdo** é um marketplace brasileiro de **pequenos vendedores**: artesãs, brechós, produtores do interior e lojinhas de bairro que não conseguem espaço (nem pagar as taxas) nos marketplaces grandes.

- **Lema:** *"Sem pressa. Com quem faz."* O "lerdo" é de propósito: comércio feito à mão, com calma. A ironia é que o site não pode ser lerdo, e é aí que você entra.
- **Como ganha dinheiro:** uma comissão pequena por venda, bem menor que a dos grandes. A margem é curta e cada real de infraestrutura importa. Por isso existe o orçamento de cada missão.
- **Para quem entrega:**
  - **vendedores**, que cadastram produtos e recebem pelos pedidos;
  - **compradores**, que buscam, compram e pagam pelo gateway externo.

  Um pedido perdido é um vendedor pequeno sem receber e um comprador cobrado sem produto: o pior que pode acontecer com a marca.
- **Onde:** um coworking em Belo Horizonte, com uma mesa, um sofá e uma gata.
- **Origem:** Álvaro Menezes vendia pela internet o crochê da mãe e se revoltou com as taxas. Ganhou o apelido "Lerdo" no colégio por ser metódico demais, e transformou o apelido na marca. A primeira versão do site foi feita por um freelancer, o Kleber, com pressa e sem medir nada. Esse é o sistema que você herda.

## 2. Elenco

| Personagem | Papel | Como fala | Onde aparece |
|---|---|---|---|
| **Álvaro "Lerdo" Menezes** | fundador e CEO | otimista, promete demais, entra em pânico quando dinheiro some; elogia rápido e esquece rápido | `#geral`, pedidos, broncas, emergências |
| **Jéssica** | suporte (a primeira contratada) | direta, cita os clientes pelo nome, não aceita "está funcionando aqui" | reclamações (app Suporte), `#geral` |
| **Otávio** | financeiro, meio período | seco, planilha na mão; fala em reais e em "runway" | orçamento da empresa; salário, bônus e aumentos do jogador |
| **Kleber** | o freelancer da primeira versão | ausente; aparece em comentários, commits antigos e, mais tarde, em pessoa | o código herdado e os bugs plantados |
| **Tabby** | gata listrada (daí o nome: *tabby* é o padrão de listras) que apareceu no coworking num dia de chuva e ficou; vive no LerdOS | calma e curiosa; nunca entrega a resposta pronta, aponta onde olhar | treinamento do primeiro dia, dicas, comentários |
| **Bia** *(Stage 2)* | primeira PM | organizada, traduz pedido do Álvaro em requisito | pedidos com prazo |
| **Rafa** *(Stage 2)* | júnior que você mentora | ansioso, erra de boa-fé | missões em que você revisa o trabalho de alguém |
| **Dra. Helena Prado** *(Stage 3)* | investidora do conselho | fria, faz perguntas de risco | metas de disponibilidade e custo |

**Clientes recorrentes** (dão rosto às reclamações e aos eventos):

- **Dona Lurdes, 67:** mãe do Álvaro, vende crochê. É a primeira vendedora.
- **Seu Jorge:** queijo da Canastra, pedidos grandes no fim do mês.
- **Thaís:** brechó online. Faz lives que derrubam o site.
- **Lu do Crochê:** criadora de conteúdo. O vídeo dela causa o C1.
- **MegaShop:** o marketplace gigante, concorrente. Aparece em notícias e em robôs que raspam preços.

## 3. O mundo do jogo

- **LerdOS** é o computador da empresa, o desktop onde o jogador trabalha. Cada app é uma ferramenta de engenharia.
- **Dois dinheiros, que nunca se misturam:**

  | | Orçamento da empresa | Salário do jogador |
  |---|---|---|
  | Moeda | **US$/mês** (a nuvem cobra em dólar) | **R$** |
  | Paga | a infraestrutura (`topology.yaml`) | extras pessoais: dicas da Tabby, cursos, horas de laboratório, ferramentas de observabilidade |
  | Quem cuida | Otávio, em nome da empresa | o jogador |
  | Entra na nota? | sim, dimensão **custo** | não: é escolha pessoal, com consequência no ritmo |
- **Tempo:** cada missão é mais ou menos **um mês** da empresa ("duas semanas depois", "um mês depois"). O salário cai a cada missão concluída.
- **Tom:** humor leve, nunca cruel. A empresa é pequena e humana. O incidente é sério, mas quem erra aprende. O debrief explica o mecanismo e compara soluções medidas. O jogo nunca pune por aprender: um prazo perdido custa bônus, nunca bloqueia a missão.

## 4. Carreira

Nove níveis, **Júnior I–III, Pleno I–III, Sênior I–III**, e **Staff** como horizonte depois do Stage 3. A promoção vem da experiência das missões, e cada stage cobre uma faixa:

| Stage | Nível ao entrar → ao sair | Título na história | O que muda na empresa |
|---|---|---|---|
| 1 — **A garagem** | Júnior I → Júnior III, promoção a Pleno I no fim | primeira pessoa de engenharia | ~130 usuários/dia no começo, crescendo a cada missão; o legado do Kleber |
| 2 — **Crescendo sem quebrar** | Pleno I → Pleno III | dona(o) da plataforma | rodada de investimento; chegam a Bia e o Rafa; Black Friday |
| 3 — **Escala e confiança** | Sênior I → Sênior III | referência técnica | conselho com metas de disponibilidade; segurança; custo |
| *4 — horizonte* | Staff | arquiteta(o) | decisões de longo prazo, mentoria; fora do escopo por enquanto |

**Salário, bônus e aumentos** (valores de jogo, a calibrar no Playtest; implementados na etapa 4, com os números em ADR-0023 e `internal/career`):

- salário mensal por nível, pago a cada missão concluída;
- **bônus** por resultado da missão: nenhum pedido perdido, custo abaixo do orçamento, poucas dicas, prazo de emergência cumprido;
- **aumento** a cada promoção; o Otávio anuncia por e-mail;
- **compras pessoais:** dica da Tabby (a primeira de cada missão é grátis) e experimentos extras no Laboratório (3 por missão são por conta da casa). Chegam depois: curso (desbloqueia uma página avançada no Docs) e visões de observabilidade (log de consultas lentas, rastreamento). É gastar consciente: informação em troca de dinheiro;
- sem saldo, o Otávio adianta o salário: o dinheiro nunca impede uma dica.

## 5. Arco de missões

Toda missão segue o ADR-0010: o teste avalia **propriedades** (latência, erros, integridade, custo), nunca a presença de uma tecnologia, e há mais de uma solução certa. A coluna "plataforma" diz o que já existe e o que precisa ser construído.

### Stage 1 — A garagem (Júnior)

| # | Missão | Gancho da história | Propriedade avaliada | Plataforma |
|---|---|---|---|---|
| C1 | **Busca Lenta** *(pronta)* | o vídeo da Lu do Crochê leva 4× mais gente ao site | busca p95, CPU do banco | pronta |
| C2 | **Pressão de Conexões** *(pronta)* | na "Semana do Lerdo", o Kleber tinha subido 3 instâncias × 20 conexões | erros, integridade de pagamento | pronta |
| C3 | **Queda de Instância** *(pronta)* | o Otávio corta custos e volta a 1 instância; ela cai | disponibilidade sob queda, cobrança única | pronta |
| C4 | **Deploy de Sexta** | o Álvaro quer uma mudança no ar durante a live da Thaís | zero erro durante o deploy sob carga (graceful shutdown, deploy em rolagem) | falta: deploy em rolagem no `stack` |
| C5 | **A Migração que Trava** | nova coluna numa tabela de pedidos grande trava o checkout | disponibilidade durante a migração (expand/contract, lock curto) | falta: dataset maior de pedidos |
| C6 | **O Backup que Ninguém Testou** | um `DELETE` sem `WHERE` do Kleber, rodado por engano | recuperar com perda ≤ N min de pedidos, em ≤ M min | falta: backup/WAL no catálogo |

Fecha com a promoção a Pleno I e o Otávio anunciando o primeiro aumento.

### Stage 2 — Crescendo sem quebrar (Pleno)

| # | Missão | Gancho | Propriedade | Plataforma |
|---|---|---|---|---|
| C7 | **Black Friday** | 10× o tráfego num dia | p95 do catálogo e custo sob pico | falta: cache no catálogo |
| C8 | **O E-mail que Segura o Checkout** | o provedor de e-mail fica lento, e o checkout espera por ele | p95 do checkout independente do e-mail; nenhum e-mail perdido | falta: fila e worker |
| C9 | **Minhas Vendas** | o painel dos vendedores faz uma consulta por pedido (N+1) | p95 do painel | pronta (código) |
| C10 | **Crochê ou Croche** | "crochê" não acha "croche": a busca ignora acentos | corretude da busca em português | pronta (banco) |
| C11 | **O Robô da MegaShop** | um robô raspa preços e derruba a busca | usuários legítimos sem impacto; robô limitado | pronta (nginx) |
| C12 | **O Relatório do Otávio** | a planilha mensal roda no banco de produção e trava as compras | compradores sem impacto durante o relatório | falta: réplica de leitura |

Fecha com o Rafa chegando: a partir daqui, algumas missões começam com código dele para revisar.

### Stage 3 — Escala e confiança (Sênior)

| # | Missão | Gancho | Propriedade | Plataforma |
|---|---|---|---|---|
| C13 | **A Chave no Repositório** | a chave do gateway vazou num commit antigo | chave antiga inválida, troca sem indisponibilidade | falta: rotação no gateway emulado |
| C14 | **Exatamente Uma Vez** | o app cai entre cobrar e gravar (o gancho que o debrief do C2 deixou) | nenhuma cobrança sem pedido sob quedas repetidas (outbox, conciliação) | parcial (injeção de falhas existe) |
| C15 | **Cinquenta Milhões de Pedidos** | a tabela de pedidos cresceu e tudo ficou lento | p95 e armazenamento no orçamento (particionar, arquivar) | falta: dataset grande |
| C16 | **Corte de 40 %** | a Dra. Helena exige cortar a nuvem sem perder qualidade | mesmos limites de latência e erro, custo 40 % menor | pronta (topologia) |
| C17 | **Plantão** | três problemas ao mesmo tempo, numa madrugada | todas as propriedades de uma vez; a ordem importa | pronta (combina falhas) |
| C18 | **Região Fora do Ar** | o provedor perde uma zona | recuperação com perda e tempo limitados | falta: segunda zona na sandbox |

**Restrições que o arco respeita:** Kubernetes, microsserviços e mentores de IA ficam fora até decisão depois do Playtest #2 (`mvp.md` §2). Missões que pedem componente novo entram no catálogo fechado (ADR-0017), com custo e limites, antes da missão.

## 6. Eventos

Implementado na etapa 3 (ADR-0022): cada pacote declara os seus em `player/events.yaml`. O gatilho é um tempo dentro de uma fase ou um resultado do jogador (teste reprovado ou aprovado, loja caída). O objetivo vem de um conjunto fechado: Production Test passando, loja de volta, deploy feito, olhar o Pulse ou o Suporte. As mensagens chegam na Caixa de entrada (e em `sesim inbox`), e o prazo mais urgente aparece em contagem regressiva na barra do topo.

| Tipo | Exemplo | Efeito |
|---|---|---|
| **Pedido** | Álvaro: "coloca um banner da Semana do Lerdo até as 18h?" | tarefa opcional; cumprida, rende bônus |
| **Bronca** | Álvaro, depois de um Production Test reprovado com clientes afetados | só tom, sem penalidade; o texto muda com o tamanho do estrago |
| **Emergência** | "o site caiu no meio da live da Thaís: 15 min" | prazo de relógio; perdê-lo custa o bônus da missão, nunca a missão |
| **Elogio** | Jéssica: "a Dona Lurdes mandou um bolo" | depois de um PASS; às vezes vem com bônus |
| **Notícia** | "MegaShop anuncia frete grátis" | contexto; prepara a próxima missão |

## 7. Guia de escrita

- PT-BR, frases curtas, sem jargão sem explicação. Personagens falam como gente: o Álvaro exagera, a Jéssica é concreta, o Otávio fala de dinheiro.
- A marca é **Mercado Lerdo**, no masculino: "o Mercado Lerdo caiu", "a loja", "o site". O computador é o **LerdOS**.
- Nenhum texto do jogo entrega a solução. Quem aponta o caminho é a Tabby, uma dica por vez.
- Números que aparecem na história (usuários, tráfego, orçamento) precisam bater com o cenário: o `scenario.yaml` é a fonte.
