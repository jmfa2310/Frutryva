# FUTRYVA — fundação de desenvolvimento

**FUTRYVA é um nome provisório.** Plataforma SaaS multiclube e multidesporto em desenvolvimento: Coach, Manager, Club e Admin. Futebol 11/9/7, futsal, voleibol, basquetebol e andebol partilham o mesmo núcleo.

## Estado real

A Sprint 0 prepara API Go, PostgreSQL 16 e Docker Compose, configuração local sem palavra-passe predefinida, migrações SQL versionadas, seed das cinco modalidades e sete variantes, testes, verificações de código, CI e contrato OpenAPI. A API disponibiliza apenas `GET /`, `GET /health` e `GET /v1/sports`.

Autenticação, convites, gestão de clubes/equipas/jogadores, interfaces React, editor 2D/3D, loja, pagamentos e subscrições **ainda não estão implementados**. As pastas em `apps/` são reservadas. MinIO será adicionado com o módulo de ficheiros.

## Iniciar com Docker

Na raiz do projeto, instalar/iniciar Docker Desktop com containers Linux ou Docker Engine e Compose v2. Criar `.env` a partir de `.env.example` sem substituir um `.env` existente e definir `DB_PASSWORD` com uma palavra-passe local forte. O Compose recusa arrancar sem esse valor.

```sh
docker compose up --build -d --wait
docker compose ps
curl --fail http://localhost:8080/health
curl --fail http://localhost:8080/v1/sports
```

No Windows, usar `Invoke-RestMethod` conforme [instalação Windows](docs/07_INSTALACAO_WINDOWS.md). Apenas a API é publicada, em loopback na porta 8080. A base de dados permanece na rede interna do Compose.

```sh
docker compose logs --tail=100 api
docker compose down
```

O último comando para os serviços e mantém o volume `postgres_data`. Não executar `down -v`, `prune` ou reset de dados. Alterar `DB_PASSWORD` num volume existente não altera a palavra-passe já guardada no PostgreSQL; manter a credencial existente até fazer uma rotação planeada.

## Desenvolvimento e validação

Go 1.26 é a toolchain usada no Docker e CI; não é necessário instalá-lo no host para arrancar via Compose. Para desenvolvimento com Go instalado, executar `sh scripts/check.sh` na raiz ou:

```sh
cd backend
go mod download
go vet ./...
go test -race -count=1 ./...
go build ./...
```

Os testes de integração exigem `DATABASE_TEST_URL` apontado a uma base PostgreSQL **exclusiva de testes**. Sem essa variável, são identificados como ignorados; testes unitários não substituem essa validação. Ver [desenvolvimento](docs/09_DESENVOLVIMENTO.md) para scripts, migrações, CI e convenções. O contrato dos endpoints existentes está em [OpenAPI](docs/openapi.yaml).

## Próxima entrega

Sprint 1: identidade e clubes, com sessões revogáveis, autorização por recurso, convites e branding versionado. Os testes contra acesso cruzado entre clubes devem acompanhar a primeira API privada. O produto completo e os requisitos futuros estão em `docs/`; a presença na documentação não significa implementação.
