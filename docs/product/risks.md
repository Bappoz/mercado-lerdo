# Riscos

Classificação qualitativa: **Probabilidade** (B/M/A) × **Impacto** (B/M/A). A análise de segurança detalhada está em `docs/security/threat-model.md`.

## Técnicos

| Risco | P×I | Por que importa | Mitigação |
|---|---|---|---|
| **Ruído de medição → avaliação instável.** O p95 num notebook varia com throttling térmico, outros apps e a VM do Docker Desktop. | A×A | Um PASS/FAIL que inverte com o mesmo código destrói a confiança na promessa central. | Física escalada com limites rígidos de cgroup, para que o componente restrito domine e não o host; limiares com folga grande entre o estado quebrado e o corrigido (≥5×, não 10 %); execuções repetidas e resultado INCONCLUSIVE; **calibração** no início que recusa hosts fracos; teste de estabilidade no CI (H5). |
| **Realismo de escala.** "1M usuários" não é reproduzível localmente. | A×M | Se apresentado como literal, é desonesto. | Física escalada + selos de proveniência. |
| **Latência de iteração.** Rebuild + restart + seed + teste podem levar minutos. | M×A | Loop lento = jogo chato. | Build Go incremental com cache em volume; troca de binário sem reconstruir imagem; snapshot do volume após o seed; Quick Check ≤ 60 s. |
| **Jogador quebra a sandbox** (dropa tabelas, apaga o seed, enche o disco). | A×M | Campanha travada. | Reset + estado canônico; snapshots; quotas; detecção com mensagem clara ("sua tabela `products` sumiu"). |
| **Burlar o evaluator.** O app devolve 200 falso, pula escrita, detecta o gerador de carga. | B (local) / A (hospedado) | Tira o sentido do PASS. | Medir fora do app; oráculo de corretude; integridade conferida pelo mock de pagamento e pelo registro do gerador, na zona observadora. |
| **Consumo de recursos na máquina do jogador.** | M×M | Exclui notebooks de 8 GB, comuns entre estudantes. | Orçamento: sandbox inteira ≤ 3 GB de RAM e ≤ 4 cores; medir nos chunks 3–5; `sesim doctor` avisa. |
| **Diferenças de plataforma** (Linux nativo vs. Docker Desktop, cgroup v1/v2, arm64). | M×M | Números diferentes por SO. | Imagens multi-arch; calibração; matriz de CI. |
| **Custo de autoria de conteúdo.** | A×A | É o gargalo real; o projeto trava depois de 3 cenários. | Formato de cenário pensado para velocidade de autoria; harness que roda o estado quebrado e as referências automaticamente. |
| **Breaking changes do SDK Docker.** O módulo `github.com/docker/docker` foi deprecado no v29 em favor de `github.com/moby/moby/client`. | M×M | Mudança de API na atualização; correções de segurança só no módulo novo. | Usar o módulo novo desde o início; isolar atrás do `SandboxDriver`. |

## Pedagógicos

| Risco | P×I | Por que | Mitigação |
|---|---|---|---|
| **Orientação mínima falha com novatos.** Aprendizado por descoberta rende menos que ensino guiado para novatos (Kirschner, Sweller & Clark, 2006). | A×A | "Investigue. Nada mais." pode gerar tentativa e erro, não aprendizado. | Persona com pré-requisitos; onboarding calmo; escada de dicas; moldura de **productive failure** (Kapur, 2008): esforço *seguido* de instrução explícita, com debrief obrigatório. |
| **Corrige sem entender.** | A×M | PASS ≠ aprendizado. | Pergunta de reflexão no debrief; gráfico de referências; conceitos reaparecem disfarçados. |
| **Lição errada da física escalada.** O comportamento com 0,25 CPU pode diferir qualitativamente de produção. | M×M | Ensinar algo falso. | Revisão de cada cenário ("o mecanismo vale em escala real?"); ressalvas no debrief. |
| **Caça a tecnologia em vez de raciocínio.** | M×M | O oposto da tese. | Cenários em que a correção da moda viola uma restrição. |
| **Bug de ambiente confundido com dificuldade.** | M×A | O jogador se culpa e desiste. | Painel de saúde da sandbox ("é o jogo ou sou eu?"). |

## Produto

| Risco | P×I | Por que | Mitigação |
|---|---|---|---|
| **É um lab, não um jogo.** | M×A | H6 falha. | Moldura de sobrevivência, curva de tensão, chat da empresa, gráfico "aha" no debrief; playtest cedo. |
| **Atrito de setup** (exige Docker). | M×A | A persona desiste antes do primeiro minuto. | Um binário, um comando; `sesim doctor` com erros acionáveis. |
| **Escopo inflado.** | A×A | Nada é entregue. | Critério de feature (brief §43) + non-goals. |
| **Concorrente se move** (Uptime Labs mirando indivíduos; ZTH-SRE ganhando avaliação). | M×M | Diferencial se desgasta. | Qualidade de cenário; entregar e aprender rápido. |
| **Desenvolvedor solo.** | A×M | Roadmap longo para uma pessoa. | Cada chunk entrega algo demonstrável. |
| **Código fechado reduz alcance e contribuições.** | M×M | Sem contribuição externa de cenários; menos confiança de quem roda um binário com acesso ao Docker. | Binários assinados com checksum; threat model publicado; repositório público de issues/releases (em aberto). |
| **Doação não cobre custos futuros** (se houver modo hospedado). | M×B agora | O local-first não tem custo de servidor; o hospedado teria. | Nada no MVP; reavaliar antes do modo hospedado. |

## Segurança

Resumo; detalhes em `docs/security/threat-model.md`.

| Risco | MVP local | Hospedado (futuro) |
|---|---|---|
| Fuga de container → host | O jogador ataca a própria máquina. Mesmo assim, cenários nunca exigem `--privileged`, socket Docker ou bind mount do host. | **Crítico**; exige gVisor/Kata/Firecracker. |
| Serviços do host alcançáveis pelo gateway da rede `internal` | Real: containers em rede `internal` ainda alcançam o IP do gateway (o host). | Idem. |
| Mineração / abuso de recursos | N/A | Alto. |
| Rede (scan, DDoS a partir da sandbox) | Egress bloqueado protege a LAN do jogador. | Alto. |
| Metadata de cloud | N/A | Bloquear 169.254.169.254. |
| Cadeia de suprimentos (imagens) | Imagens fixadas por digest. | Idem + SBOM. |
| Segredos falsos no app demo (cenário de vazamento) | Devem ser falsos e rotulados; entram no allowlist do gitleaks com justificativa. | Idem. |
| UI/terminal expostos na LAN ou a páginas maliciosas | Bind em `127.0.0.1`, token por sessão, checagem de `Host` e `Origin`. | AuthN/Z completo. |

## Referências

- Kirschner, P. A., Sweller, J., & Clark, R. E. (2006). *Why Minimal Guidance During Instruction Does Not Work.* Educational Psychologist, 41(2), 75–86.
- Kapur, M. (2008). *Productive Failure.* Cognition and Instruction, 26(3), 379–424.
- Depreciação de `github.com/docker/docker`: https://github.com/moby/moby/discussions/52404
