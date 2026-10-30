# Desenvolvimento — Sprint 1

## Organização e módulos

`backend/main.go` compõe o processo e gere arranque/paragem. `internal/config` lê configuração; `internal/httpapi` fornece catálogo, readiness e proteção HTTP; `internal/migrate` aplica SQL versionado; `internal/identity` gere contas, sessões e códigos; `internal/organizations` valida pertenças, clubes, convites e branding. `database/` é a fonte única das migrações. `apps/manager` contém a primeira interface React/TypeScript; as restantes aplicações continuam reservadas.

Futuros módulos `teams`, `athletes`, `training`, `matches` e restantes devem ter dependências explícitas, domínio separado do transporte e autorização por recurso no servidor. Não criar microsserviços nem duplicar lógica por modalidade. A futura cena/timeline do editor será partilhada por renderizadores 2D e 3D.

Usar `gofmt`, SQL parametrizado, contextos limitados e erros sem credenciais. Acrescentar testes de comportamento, autorização e contrato OpenAPI a cada alteração. No frontend, manter labels, navegação por teclado, estados de foco, erros e layout responsivo; a validação de contraste do branding no backend não substitui uma auditoria de acessibilidade da aplicação.

## Configuração e serviços

O `.env` ignorado pelo Git contém `DB_PASSWORD` e pode conter `COMPOSE_PROJECT_NAME`, `API_PORT`, `MANAGER_PORT`, `MAIL_PORT`. Preservar estas variáveis e o diretório/nome do projeto ao atualizar. As portas publicadas por omissão são 18080, 18081 e 18025; PostgreSQL e SMTP permanecem na rede interna do Compose.

Fora do Compose, configurar PostgreSQL com `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD`, `PGSSLMODE`, ou `DATABASE_URL`. Não interpolar palavras-passe num URI sem codificação adequada e não imprimir configuração completa. `MIGRATIONS_DIR` é `../database` ao arrancar de `backend/` e `/app/database` na imagem; `PORT` interno é 8080 por omissão. `HOST` seleciona a interface de escuta.

`SMTP_HOST=mailpit` e `SMTP_PORT=1025` no Compose usam uma caixa de email local, consultável no browser em 18025. Fora do Compose, SMTP pode apontar para loopback. O adaptador fornecido só permite Mailpit/loopback: não é um conector de email de produção. Não colocar chaves SMTP nem credenciais no código. `COOKIE_SECURE=false` serve HTTP local; um deployment HTTPS exige cookies Secure, proxy e configuração operacional próprios.

## Autenticação e publicação

Passwords têm entre 12 e 72 bytes e são guardadas com bcrypt. A sessão opaca em cookie `futryva_session` é HttpOnly, SameSite=Lax, limitada a 24 horas, guardada como hash e revogável no servidor. Refresh roda o identificador; reset de palavra-passe revoga sessões. Códigos de confirmação duram 24 horas; recuperação, 30 minutos; convites, sete dias. Os códigos são de uso único e os hashes, não os valores originais, ficam nas tabelas de autenticação/convites.

O browser usa pedidos da mesma origem ao proxy Manager. Escritas validam Origin/Sec-Fetch-Site e JSON; não há CORS aberto. Clientes sem Origin podem usar a API em JSON. Não guardar sessões em localStorage nem inserir códigos em URLs duradouras. O Mailpit local contém os emails e os seus códigos e só deve ser exposto em loopback durante o desenvolvimento.

Confirmar email antes das escritas privadas. A pertença ao clube é verificada em cada operação; URLs/clubId do cliente não conferem acesso. Presidente/diretor gerem branding e convites; apenas presidente arquiva o clube e convida diretores. Draft, histórico, membros e emails de convites ficam restritos à gestão. Um treinador lê o clube a que pertence, mas não obtém gestão por aceitar um convite.

Publicar branding acrescenta uma versão e muda a referência pública. Alterar o rascunho não altera a versão pública; rollback publica uma nova versão com o tema histórico e alinha o rascunho. `DELETE /v1/clubs/{clubId}` arquiva logicamente, sem apagar registos. Os endpoints públicos só devolvem identidade publicada de clubes não arquivados.

Imagens de branding aceitam HTTPS ou data URLs de PNG/JPEG/WebP estático até 200 KiB por imagem e 16 megapixels. SVG e formatos ativos não são permitidos. Esta opção mantém a primeira entrega independente de S3/MinIO; armazenamento dedicado e processamento de media pertencem a uma fase própria. O backend exige contraste mínimo 4,5:1 entre texto e superfície, até 20 patrocinadores e seis cores de equipamento.

## Migrações e preservação

O arranque adquire advisory lock transacional, valida o histórico/checksums e aplica novas migrações numericamente. `schema_migrations` guarda nome, SHA-256 e data. SQL falhado não deixa DDL nem ledger parcialmente aplicados. Nomes SQL inválidos, versões zero/duplicadas/retroativas e ficheiros históricos alterados ou ausentes provocam falha.

`001_init.sql` permanece inalterado. O seu seed repete-se sem duplicar cinco modalidades/sete variantes e adota bases do starter sem reset. `002_identity_and_clubs.sql` acrescenta identidade, sessões, códigos, arquivo e branding/convites. Não há rollback destrutivo automático. Criar alterações em novos ficheiros numerados; `.gitattributes` mantém SQL com LF entre Windows e Linux.

Antes de aplicar migrações a dados importantes, fazer backup e validar numa cópia. A migração de emails normalizados exige que não existam duplicados que só diferem em maiúsculas/minúsculas; se existirem, a transação falha e os dados ficam preservados para resolução explícita. Nunca corrigir falhas eliminando volumes ou alterando checksums.

## Verificações locais

```sh
sh scripts/check.sh
cd apps/manager
npm ci
npm test
npm run build
```

O script Go executa formatação, vet, testes com race detector e build. Os testes PostgreSQL exigem `DATABASE_TEST_URL` numa base exclusiva, nunca produção/dados pessoais. Criam schemas aleatórios com dados sintéticos e mantêm-nos; a ausência da variável produz skips explícitos. Cobrem migrações, autenticação, expiração/replay, revogação e autorização entre clubes.

Com o Compose saudável:

```sh
python3 scripts/smoke.py http://127.0.0.1:18080
python3 scripts/smoke_sprint1.py --bootstrap-owner --base-url http://127.0.0.1:18081 --mailpit-url http://127.0.0.1:18025
```

O smoke Sprint 1 usa cookies independentes, emails únicos e Mailpit real; verifica códigos, isolamento, branding, convites, rotação/logout, reset e arquivo. Mantém os registos e mensagens sintéticos. Pode apontar diretamente à API, mantendo Origin igual ao endereço usado. No Windows com Python instalado, substituir `python3` por `py`.

Para frontend fora de Docker, `npm run dev` em `apps/manager` usa o proxy Vite para a API. Consultar o README dessa app para a variável do destino local. O build TypeScript faz parte de `npm run build`.

## CI e limites da validação

`.github/workflows/ci.yml` usa Go 1.26, Node.js 22, PostgreSQL 16 e Mailpit. Gera uma credencial de DB por execução e mascara-a; executa testes Go com integração/race, build, smoke da API real, testes do Manager e build TypeScript/Vite. Os serviços CI são separados dos volumes de desenvolvimento e publicados só em loopback.

Criar ou alterar o workflow não prova uma execução remota no GitHub. O relatório deve distinguir checks locais efetivamente executados, skips e resultado remoto observado. Esta entrega não substitui testes de carga, auditoria de segurança, MFA, email de produção, backups/restauro e revisão de proteção de dados necessários ao lançamento.

## Ambiente na nuvem

Cada tarefa já tem isolamento próprio: usar o checkout existente, sem criar worktrees salvo pedido explícito. Dependências/ficheiros podem ser retidos em snapshot; processos têm de arrancar novamente. Instruções guardadas devem conter diretório, arranque de API/Manager/Mailpit e readiness; configuração guardada ainda exige revisão/publicação pelo utilizador. O build deve manter proxy/CA e verificações TLS e de integridade.
