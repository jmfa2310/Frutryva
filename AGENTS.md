# Instruções permanentes para Codex — FUTRYVA

1. Antes de alterar código, ler `README.md`, `docs/00_VISAO_PRODUTO.md`, `docs/06_ROADMAP_E_CRITERIOS.md` e documentação do módulo relevante.
2. Projeto SaaS multiclube/multidesporto: futebol 11/9/7, futsal, voleibol, basquetebol, andebol. Coach, Manager, Club, Admin.
3. Branding obrigatório e configurável por clube: cores, emblemas, imagens, equipamentos, patrocinadores; sem forks de código.
4. Um utilizador pode ter múltiplos clubes/papéis; treinadores independentes; permissões verificadas no backend. Privacidade e dados de menores por defeito.
5. Editor: uma cena/timeline versionada, dois renderizadores 2D e 3D. Não desenvolver sistemas de animação incompatíveis.
6. Preservar TODOS os ficheiros, dados e funcionalidades existentes. Não apagar volumes, não fazer reset de DB, não executar `down -v`, `prune`, `DROP`, `TRUNCATE` ou migrações destrutivas sem autorização explícita e backup.
7. Implementar em passos pequenos; executar testes/build disponíveis e reportar o que foi ou não verificado. Não afirmar funcionalidade inexistente.
8. Não inventar integrações com federações, preços, regras desportivas ou serviços externos. Confirmar requisitos antes de integrações irreversíveis.
9. Nunca versionar segredos, credenciais ou dados pessoais reais. Criar `.env.example` sem segredos quando necessário.
10. Manter documentação e OpenAPI atualizadas; incluir testes de autorização cruzada entre clubes, migrações e acessibilidade.
11. Primeira prioridade: Sprint 0 e Sprint 1 do roadmap; só depois módulos desportivos, editor, loja e billing.
12. Nome FUTRYVA é provisório até confirmação jurídica e comercial.
