# Debrief — Queda de Instância

## O que aconteceu

Duas coisas que só aparecem quando algo quebra:

1. **Uma instância só é um ponto único de falha.** Quando o processo morreu, não havia mais
   ninguém para atender. Reiniciar automaticamente traz a loja de volta, mas quem chegou
   durante a volta ficou sem resposta, e um processo que cai uma vez costuma cair de novo.
   *Recuperação* (voltar rápido) não é *redundância* (não parar).
2. **Retries exigem idempotência.** O checkout que estava no meio do caminho quando a instância
   caiu volta como um novo clique do cliente, com o mesmo `order_ref` e a mesma
   `Idempotency-Key`. O código tratava cada chegada como um pedido novo e cobrava de novo. O
   gateway oferecia a solução (ele cobra cada `Idempotency-Key` uma vez só), mas o app não
   mandava a chave.

## Soluções medidas neste mesmo teste

Mediana de 3 execuções do Production Test (60 s de compras a 50 req/s, instância 1 derrubada
duas vezes em cada execução, clientes que clicam de novo), em 2 rodadas, na máquina de
referência; os intervalos mostram as duas rodadas. Não são da sua máquina.

| Solução | Erros | Pedidos com dinheiro divergente (total do teste) | Custo/mês | Veredito |
|---|---|---|---|---|
| Uma instância, código original | 64 % | 30–35 (cobrados sem pedido) | US$ 70 | FAIL em erros, latência e integridade |
| Reinício automático | 1,0–1,2 % | 57–62 (cobrados em dobro) | US$ 70 | FAIL em erros e integridade |
| Reinício automático + checkout idempotente | 0,9–1,2 % | 0 | US$ 70 | FAIL em erros |
| Duas instâncias | 0,22–0,26 % | 17–21 (cobrados em dobro) | US$ 90 | FAIL em integridade |
| Duas instâncias + checkout idempotente | 0,22 % | 0 | US$ 90 | **PASS** |
| + nginx repete em outra instância (inclusive POST) | 0,00 % | 0 | US$ 90 | **PASS** |

## Por que redundância sem idempotência piora

Com duas instâncias, a loja não para, e é justamente por isso que os retries passam a dar
certo: o segundo clique encontra uma instância viva e cobra de novo. Quanto mais o sistema se
esforça para completar uma requisição (o cliente repete, o nginx repete em outra instância),
mais importante é que repetir não mude o resultado.

## Ressalva de escala

Em produção de verdade, as instâncias ficam em máquinas (e zonas) diferentes, o balanceador
faz health check ativo, e o deploy troca uma instância de cada vez (rolling update). A
idempotência costuma morar no banco (chave única por operação) e no provedor de pagamento
(chave de idempotência), exatamente como aqui.

## Antes de seguir

Responda para você mesma(o), em duas frases: **por que reiniciar rápido não bastava**, e **por
que a segunda instância, sozinha, aumentou as cobranças em dobro**.
