# Roadmap executável e critérios de aceitação

## Sprint 0 — Preparação
Repositório privado, README, AGENTS.md, convenções, `.env.example`, lint, testes, CI, Docker Compose, migrações e seed. Verificar build e health; não apagar volumes/dados.

## Sprint 1 — Identidade e clubes
Registo/login/recuperação, sessões, roles e memberships, convites, CRUD de clube, rascunho e publicação de branding. Aceitação: dono cria/atribui clubes, cada utilizador tem apenas o seu clube, dois utilizadores com cores/emblemas distintos não acedem ao clube do outro; convite expira; testes automáticos.

## Sprint 2 — Modalidades e equipas
Cinco modalidades, variantes, épocas, equipas, plantéis e treinadores. Aceitação: clube cria futebol, futsal, voleibol, basquetebol e andebol e atribui treinadores com permissões distintas.

## Sprint 3 — Treinos e jogos básicos
Sessões, presenças, calendário, jogos, resultados e eventos próprios de cada modalidade. Aceitação: registar e consultar jogo de cada modalidade; corrigir evento sem duplicar estatística.

## Sprint 4 — Editor 2D
Campo, objetos, setas, keyframes, timeline, save/load, undo/redo. Aceitação: exercício reproduz igual após recarregar.

## Sprint 5 — Visualização 3D
Renderizador Three.js/R3F partilha cena/timeline com 2D, câmara, modelos leves e fallback. Aceitação: troca 2D/3D sem perder posições e sincronização temporal.

## Sprint 6 — Club e Manager comercial
Publicação seletiva, resultados, plantéis, seguidores, notícias, produtos, encomendas, pagamentos em sandbox; métricas de gestão. Aceitação: adepto compra produto de clube A sem acesso aos dados do clube B; webhook idempotente.

## Sprint 7 — SaaS e piloto
Planos, limites, onboarding, backups, RGPD, auditoria, acessibilidade, performance, testes e observabilidade; piloto com treinadores/clubes reais. Aceitação: restauração de backup testada, testes E2E principais, validação de preço e suporte.

## Definition of Done
Código revisto, testes relevantes, migrations reversíveis quando viável, sem segredos no Git, documentação atualizada, autorização no backend, compatibilidade responsiva, erros tratados e nenhum dado existente apagado.

## Dependências e riscos
Editor 3D e exportação de vídeo têm maior risco técnico; dados de menores e pagamentos maior risco legal/operacional; resultados/classificações externos dependem de fontes e direitos. Estimativas de tempo só após medir velocidade das primeiras sprints.
