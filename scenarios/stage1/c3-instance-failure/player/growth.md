# #geral — 18:40

**Fundador:** A loja caiu. Do nada. Está todo mundo vendo erro.

**Suporte:** Chegando reclamação de todo lado. E dois clientes dizem que foram cobrados em dobro
da última vez que isso aconteceu.

---

**Produção fora do ar: o processo do app morreu.**

Primeiro, coloque a loja de volta ({{deploy}} reinicia o app). Depois, o que importa: **a próxima
queda não pode derrubar a loja nem cobrar ninguém duas vezes.** {{test}} derruba uma instância no
meio de cada execução, com os clientes clicando de novo, e confere cada cobrança.

{{lab}} deixa você ensaiar a queda antes.
