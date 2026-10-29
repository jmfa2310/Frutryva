# Modelo de dados alvo (planeamento)

## Núcleo
users; sessions; clubs; club_branding_versions; club_memberships; invitations; sports; sport_variants; seasons; teams; team_staff; athletes; team_rosters; media_assets; audit_events.

## Treino
exercise_library; exercises; exercise_versions; exercise_scenes (JSONB versionado); training_sessions; training_session_exercises; training_attendance; training_notes; exercise_permissions.

## Competição
competitions; matches; match_rosters; match_lineups; match_events; player_match_stats; sport_stat_definitions; publication_records; standings (manual ou integração licenciada).

## Comércio
products; product_variants; inventory_movements; carts; orders; order_items; payments; refunds; shipping_methods; followers; notification_preferences; subscriptions; invoices/references do prestador de pagamento.

## Regras estruturais
- IDs UUID para entidades principais; timestamps UTC; soft-delete quando justificável; versionamento e migrações.
- ClubId nas entidades do clube, com FKs compostas para impedir cruzamento de jogadores/equipas.
- Um atleta pode integrar várias equipas/épocas; distinguir pessoa, inscrição e estatística.
- Eventos e métricas são configurados por modalidade e competição; nunca assumir que pontos/golos/cartões são universais.
- Estatísticas derivadas de eventos auditáveis, com correções e recálculo; impedir contagens duplicadas.
- Dados privados do treinador não são automaticamente propriedade do clube; definir propriedade/visibilidade ao criar.

## Estado atual do repositório
`database/001_init.sql` preserva as tabelas iniciais de sports, sport_variants, users, clubs, club_memberships, teams, team_coaches, players, team_players e exercises. A migração aditiva `002_identity_and_clubs.sql` acrescenta verificação de email, sessões e códigos de acesso, arquivo de clubes, rascunhos e versões de branding e convites. **As entidades restantes são desenho futuro, ainda não migrado.**
