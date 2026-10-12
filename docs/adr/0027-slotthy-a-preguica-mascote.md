# ADR-0027: A mascote é a Slotthy, a preguiça da logo

**Status:** Proposto
**Data:** 2026-10-12
**Decisores:** Lucas Zanetti

## Contexto

A mascote do LerdOS era a Tabby, uma gata listrada (ADR-0016, ADR-0024, ADR-0026). A marca do jogo, porém, é uma preguiça: ela está na logo, no favicon e no nome (Mercado *Lerdo*). A gata não tinha relação com a marca, e a sua animação era curta: nove poses em 24×24, trocadas por temporizadores soltos (piscar, balançar o rabo, mexer a boca).

O autor pediu uma mascote mais interessante: a preguiça da logo, chamada **Slotthy**, com quadros suficientes para uma animação fluida.

## Decisão

1. **A mascote é a Slotthy**, uma preguiça (feminino, como a Tabby era): mesma história (apareceu no coworking num dia de chuva e ficou) e mesmo papel (treinamento do primeiro dia, C1 guiado, dicas, leituras). Só muda o bicho e o nome.
2. **O nome muda em tudo o que está vivo:** textos, identificadores, arquivos (`web/src/slotthy.ts`, `os/Slotthy.tsx`, `os/SlotthyActor.tsx`), tokens de cor (`--slotthy-*`) e o comando do terminal, que passa de `sesim tabby` para `sesim slotthy`. Os ADRs anteriores e as linhas já feitas do roadmap ficam como foram escritos: onde dizem Tabby, leia-se Slotthy.
3. **A arte é 32×32 na escala 3** (os mesmos 96 px na tela da gata em 24×24 na escala 4), com uma cor a mais na paleta: `l`, o pelo claro da barriga. As cores padrão vêm da logo (contorno azul-marinho, pelo castanho, rosto creme); cada tema tem a sua paleta e as pelagens são castanho, cinza e mel.
4. **Os quadros são gerados, não digitados:** `scripts/slotthy-frames.py` desenha cada quadro com formas (elipses, membros, garras) e um contorno automático, e escreve as linhas de `SLOTTHY_FRAMES`. Mexer num braço ou no rosto muda todos os quadros de uma vez. O script é ferramenta de desenho: não roda no build nem no `just check`, e o que vale é o bitmap commitado.
5. **A animação é declarada em clipes** (`web/src/slotthyClips.ts`): cada clipe é uma lista de (quadro, duração). São 30 quadros em 15 clipes:
   - em repouso: respirar, piscar (com o quadro de olho meio fechado), olhar para os lados, bocejar, coçar a cabeça;
   - sozinha por um minuto num dia calmo, ela cochila e dorme, até a próxima fala ou clique: é uma preguiça;
   - falar (três bocas), com aceno nas boas-vindas;
   - feliz e preocupada, cada uma com o seu ciclo e a sua fala;
   - apontar (dois quadros) e andar (quatro quadros, de lado).

   Uma regra pura, `nextClip(humor, silêncio, sorteio)`, escolhe o próximo clipe; `useIdlePose` só toca. O balanço em CSS do sprite saiu: a respiração agora está nos quadros.
6. **Sem animações** (preferência do jogador ou `prefers-reduced-motion`), ela mostra o quadro parado do humor (`REST`), como antes.

## Consequências

- **Mais fácil:** a mascote e a marca são a mesma personagem; acrescentar um gesto é um quadro no script e uma linha num clipe; a escolha do próximo gesto é testável sem relógio.
- **Mais difícil:** quem editar a arte precisa do Pillow para rodar o script (ou editar as linhas à mão, como antes).
- **Quebra:** `sesim tabby` deixa de existir (o jogo ainda não tem jogadores fora do autor); a pelagem salva nas preferências (`laranja`, `frajola`) volta para o padrão.
- **Limites conhecidos:** o sorteio dos gestos usa `Math.random`, então duas sessões não se mexem igual; a caminhada tem o mesmo ritmo da gata (`walkMs`), não o de uma preguiça de verdade.
