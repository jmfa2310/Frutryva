# Roadmap de implementação

## Produto
- **Coach**: planeamento, exercícios animados 2D/3D, jogadores, treinos, jogos, estatísticas.
- **Club**: jogos, plantéis, resultados, estatísticas públicas, notícias, loja e seguidores.
- **Manager**: clubes, modalidades, equipas, treinadores, identidade visual, encomendas, sócios.
- **Admin**: clientes, subscrições, suporte e auditoria.

## Regras inegociáveis
- Uma plataforma e código partilhado; personalização por clube de cores, emblema, capa, equipamento e patrocinadores.
- Isolamento entre clubes validado na API; um utilizador pode ter múltiplos papéis e clubes.
- Treinador independente pode criar biblioteca privada; dados privados do clube não saem com ele.
- Futebol 11/9/7, Futsal, Voleibol, Basquetebol 5x5 e Andebol na arquitetura inicial.
- Estatísticas e regras específicas por modalidade (sets, pontos, golos, faltas, etc.).
- Editor 2D e 3D partilham cenas, coordenadas e animações; implementação faseada.
- Publicação de dados de menores depende de regras e permissões adequadas.

## Entregas
1. Fundação: autenticação, convites, clubes, branding, permissões, equipas.
2. Coach: plantéis, calendário, treinos, jogos, estatísticas.
3. Editor: 2D animado, motor de keyframes, depois 3D e modelos animados.
4. Club: portal público, resultados, plantéis, loja e encomendas.
5. Manager/Admin: faturação, subscrições, dashboards, auditoria.
6. Testes, RGPD, backups, piloto com clubes, publicação.

## Nota
Os nomes, preços e prazos são hipóteses até validação com clientes e verificação de marca.
