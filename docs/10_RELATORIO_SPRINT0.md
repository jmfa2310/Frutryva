# Entrega Sprint 0 — 29 outubro 2026

## Âmbito e estado

O starter do ZIP foi importado no checkout existente `/workspace/Frutryva`, inicialmente sem commits nem ficheiros de aplicação. Foram lidos AGENTS, README, a primeira tarefa e todos os documentos. O utilizador confirmou que esta execução termina na Sprint 0.

API Go e PostgreSQL estão funcionais no ambiente atual. A API só expõe o índice, health e catálogo de modalidades. Autenticação, clubes, branding, convites, equipas, jogadores e interfaces são a próxima fase, não funcionalidades entregues. Não houve push para GitHub nem execução remota do CI.

## Alterações face ao ZIP

Ficheiros alterados: `.env.example`, `docker-compose.yml`, `backend/Dockerfile`, `backend/go.mod`, `backend/main.go`, `README.md` e `docs/07_INSTALACAO_WINDOWS.md`.

Ficheiros novos:

- `.dockerignore`, `.gitattributes` e `.github/workflows/ci.yml`.
- `backend/go.sum` e `backend/main_test.go`.
- `backend/internal/config/config.go` e `config_test.go`.
- `backend/internal/httpapi/handler.go`, `handler_test.go`, `store.go` e `store_test.go`.
- `backend/internal/migrate/migrate.go`, `migrate_test.go` e `migrate_validation_test.go`.
- `scripts/check.sh` e `scripts/smoke.py`.
- `docs/09_DESENVOLVIMENTO.md`, `docs/openapi.yaml` e este relatório.

Os restantes ficheiros do starter foram preservados. `database/001_init.sql` foi comparado byte a byte com o ZIP original e está inalterado. Como o repositório não tinha commits, os ficheiros importados e desenvolvidos ainda são novos no Git.

A implementação separa configuração, HTTP, consultas SQL e migrações; adiciona timeouts, encerramento gracioso, healthcheck de container, erros JSON e rotas 404/405 corretas. O runtime executa sem root. A credencial predefinida foi removida; `.env` local é ignorado e não integra o arquivo de entrega. O build na nuvem usa a CA pública do proxy, sem desligar TLS ou verificação de dependências.

## Validação executada

- `go mod download` e `go mod verify`: dependências verificadas, com `go.sum` gerado.
- `sh scripts/check.sh`: gofmt, `go vet -mod=readonly ./...`, `go test -mod=readonly -race -count=1 ./...` e build passaram. Sem `DATABASE_TEST_URL`, a parte PostgreSQL é ignorada, pelo que foi executada também a validação completa seguinte.
- `go test -mod=readonly -race -count=1 -json ./...`, com PostgreSQL 16 separado e credencial efémera: **19 testes principais, 62 resultados aprovados incluindo subtestes, zero falhas e zero testes ignorados**.
- Migrações: adoção de base legada sem perda dos registos sintéticos, repetição sem duplicação, concorrência, rollback transacional de SQL falhado, checksum, histórico ausente, ordem numérica e rejeição de versões retroativas.
- Isolamento referencial: as chaves compostas rejeitam associações entre equipas e jogadores de clubes distintos. Isto não valida autorização HTTP, ainda inexistente.
- `docker compose config --quiet` e `docker compose up --build -d --wait`: build e serviços saudáveis. Foi corrigida uma falha real de permissões dos SQL copiados para o runtime sem root.
- `python3 scripts/smoke.py`: pedidos reais ao índice, health, sete variantes de cinco modalidades, 404 e 405 aprovados.
- A preparação foi repetida sobre o mesmo volume, sem reset. YAML de CI e OpenAPI analisados localmente; CI do GitHub ainda não executado.

## Iniciar e continuar

Os passos PowerShell estão em `07_INSTALACAO_WINDOWS.md`: criar `.env` sem substituir um existente, definir a palavra-passe local e executar `docker compose up --build -d --wait`. Validar com `Invoke-RestMethod http://localhost:8080/health` e `/v1/sports`. Não existe ainda interface web. `docker compose down` mantém os dados; nunca usar `-v` para resolver um erro.

Próxima tarefa: Sprint 1, começando por registo/login, sessões revogáveis e testes de autorização entre clubes; depois clubes, convites e branding versionado. Email, recuperação de acesso, MFA, auditoria e restantes requisitos de lançamento devem ser validados nas fases correspondentes.
