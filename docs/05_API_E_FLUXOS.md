# API e fluxos (contratos propostos, não implementados)

## Autenticação
`POST /v1/auth/register`, `/login`, `/logout`, `/refresh`, `/forgot-password`, `/reset-password`, `/verify-email`; `GET /v1/me`.

## Clube e permissões
`POST /v1/clubs`; `GET /v1/clubs/{clubId}`; `PATCH /v1/clubs/{clubId}`; `GET/PUT /v1/clubs/{clubId}/branding/draft`; `POST /v1/clubs/{clubId}/branding/publish`; `POST /v1/clubs/{clubId}/invitations`; `POST /v1/invitations/{token}/accept`.

## Equipas e treino
`GET/POST /v1/clubs/{clubId}/teams`; `GET/POST /v1/teams/{teamId}/athletes`; `GET/POST /v1/exercises`; `GET/PUT /v1/exercises/{id}`; `POST /v1/teams/{teamId}/training-sessions`.

## Jogos, publicação, loja
`GET/POST /v1/teams/{teamId}/matches`; `POST /v1/matches/{id}/events`; `POST /v1/matches/{id}/publish`; `GET /v1/public/clubs/{slug}`; `GET /v1/public/clubs/{slug}/matches`; `GET /v1/public/clubs/{slug}/products`; `POST /v1/orders/checkout`.

## Fluxo 1 — Clube
Presidente regista-se → confirma email → cria clube → define cores/emblema em rascunho → pré-visualiza → publica → cria modalidade/equipa → convida treinador.

## Fluxo 2 — Treinador
Recebe convite expirá­vel → autentica-se/cria conta → aceita associação → seleciona equipa → gere jogadores → cria sessão → cria exercício animado → guarda → reproduz em 2D/3D.

## Fluxo 3 — Adepto
Escolhe clube → vê conteúdo público publicado → segue clube (com consentimento) → compra produto → recebe confirmação do pagamento do prestador → acompanha encomenda.

## Fluxo 4 — Jogo
Treinador regista eventos → valida e corrige → responsável publica → app Club mostra resultado/estatísticas autorizadas → alteração posterior gera nova versão auditada.

## Regras de API
OpenAPI obrigatório; paginação; validação; idempotência em pagamentos e convites; erros padronizados; autorização por recurso; testes de isolamento de clubes; não expor tokens em URLs de navegação duradouras.
