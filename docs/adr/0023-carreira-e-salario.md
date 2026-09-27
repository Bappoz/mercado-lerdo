# ADR-0023: Carreira e salário derivados da campanha, com dinheiro que nunca bloqueia

**Status:** Proposto
**Data:** 2026-09-27
**Decisores:** Lucas Zanetti

## Contexto

O universo (`docs/product/universe.md` §4) dá ao jogador uma carreira, de Júnior I a Sênior III, e um salário em R$, com bônus e aumentos. O salário é dele, separado do orçamento da empresa (que paga a infraestrutura e entra na nota de custo), e serve para comprar extras de forma consciente. A etapa 3 (ADR-0022) já registra o resultado dos eventos da história, e é daí que vêm os bônus.

## Decisão

1. **Um núcleo puro** (`internal/career`). `Compute(contratação, missões concluídas, compras)` deriva o perfil: nível, experiência, saldo, holerites, promoções e extrato. Nada de saldo ou XP gravado: tudo é recalculado da campanha.
   - **As entradas:**
     - `Finished` do histórico, agora com as compras e o custo contra o orçamento no PASS;
     - a missão atual, depois do PASS;
     - os eventos de cada pacote, para saber o que foi emergência e o que foi pedido.
   - **Vantagem:** mudar um valor de calibração recalcula as carreiras existentes, em vez de deixar números velhos gravados.
2. **Valores de jogo, a calibrar no Playtest:**
   - **Escada:** 9 níveis com limiares de XP de 0 a 3400. São cerca de 2 missões por nível, então as 6 missões planejadas do Stage 1 levam de Júnior I a Pleno I.
   - **Salário por missão:** de R$ 4.000 a R$ 19.000, pago no nível em que a missão termina. A promoção vale a partir da missão seguinte.
   - **XP por missão:** base de 150. Somam-se 60 por emergência no prazo, 20 por pedido, 40 sem dica ou 15 com até 2 dicas, e 20 por economia.
   - **Bônus em % do salário:** 20 % por emergência no prazo; 5 % por pedido atendido; 15 % sem dicas ou 5 % com até 2; 10 % se o custo ficou ≤ 90 % do orçamento. Cada tipo de bônus ocupa uma linha no holerite, por mais vezes que aconteça.
   - **Bônus de contratação:** R$ 1.500, para as primeiras missões poderem pagar uma dica.
3. **O que o salário compra** (só o que já existe no jogo):
   - **dicas da Tabby:** a primeira de cada missão é grátis; depois R$ 150, 250, 350, 450. As cinco do C1 cabem no bônus de contratação;
   - **experimentos extras no Laboratório:** 3 por missão por conta da casa; depois R$ 100.

   Cursos e ferramentas de observabilidade ficam anunciados como "próximos stages": nada é vendido antes de existir.
4. **O dinheiro nunca bloqueia.** Sem saldo, a compra acontece e o saldo fica negativo: o Otávio adianta o salário. Nenhuma dica é negada por falta de dinheiro, porque o jogo ensina e não pune (`universe.md` §3).
5. **Onde aparece:**
   - app **Perfil**: avatar, nível, barra de XP, saldo, preços dos próximos extras, holerites e extrato;
   - atalho na barra do topo ("Júnior I · R$ 1.500");
   - preço no botão de dica da Tabby e no botão do Laboratório;
   - e-mails do Otávio: boas-vindas à folha no primeiro dia, holerite a cada PASS, promoção quando houver;
   - no terminal, `sesim profile` e o preço impresso por `sesim hint`.

## Consequências

- **Mais fácil:** cada missão nova entra na carreira sem código, pelos eventos (ADR-0022) e pelo orçamento do cenário. A calibração é um arquivo Go só, com testes.
- **Mais difícil:**
  - pagar por dica muda o comportamento que a H1 mede (dicas usadas até o PASS). O Playtest #2 precisa registrar se o preço inibiu dicas úteis;
  - o saldo não é fonte de verdade gravada. Se um pacote antigo não carrega mais, as missões dele ficam sem bônus de história, mas o salário continua (há log de aviso).
- **Limite conhecido:** o saldo pode ficar negativo por tempo indefinido. Hoje isso não tem consequência além do aviso no Perfil.
