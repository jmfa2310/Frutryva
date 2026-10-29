# Desenvolvimento — Sprint 0

## Organização

`backend/main.go` compõe o processo e gere arranque e paragem. `internal/config` lê configuração; `internal/httpapi` contém transporte HTTP e acesso ao catálogo; `internal/migrate` aplica SQL versionado. `database/` é a fonte única das migrações. `apps/` continua reservado às experiências React futuras.

Os futuros módulos `identity`, `organizations`, `memberships`, `teams`, `athletes` e restantes devem ter dependências explícitas, serviços de domínio separados dos handlers e autorização no servidor. Não criar microsserviços nem quatro cópias da lógica comum. Conteúdo privado permanece privado por defeito. A cena e timeline do futuro editor terão uma representação versionada comum aos renderizadores 2D e 3D.

Usar `gofmt`, erros sem segredos, contextos com prazo e SQL parametrizado. Adicionar testes de comportamento à alteração e atualizar OpenAPI. Toda a primeira funcionalidade privada exige testes de isolamento por clube; as constraints existentes não substituem autorização HTTP. Não há frontend nesta entrega para auditar acessibilidade: os primeiros componentes devem incluir navegação por teclado, labels e contraste testados.

## Configuração

O Compose exige `DB_PASSWORD` no `.env` local ignorado pelo Git. Para executar o backend fora do Compose, definir `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` e `PGSSLMODE`, ou usar `DATABASE_URL`. Não imprimir estas variáveis nem inserir valores em scripts ou documentação. `DATABASE_URL`, quando definido, usa o formato suportado por pgx. `MIGRATIONS_DIR` tem o valor predefinido `../database` ao executar a partir de `backend/`; a imagem usa `/app/database`. `PORT` é 8080 por omissão e `HOST` permite selecionar a interface.

O PostgreSQL do Compose não publica uma porta no host. Para Go no host, usar uma instância PostgreSQL de desenvolvimento separada ou um override local explícito que publique a porta apenas em loopback. Não expor a base à rede pública. As credenciais do `.env` são só de desenvolvimento; produção exige TLS, gestão de segredos e revisão operacional.

## Migrações e seed

O arranque liga à base, adquire um advisory lock transacional e aplica `database/NNN_nome.sql` por ordem. O ledger `schema_migrations` guarda nome, checksum SHA-256 e data. A transação impede registos de sucesso se um SQL falhar; o lock evita duas APIs migrarem simultaneamente. Se uma migração aplicada for alterada ou desaparecer, o arranque falha em vez de aceitar divergência silenciosa.

`001_init.sql` permanece inalterado. O SQL idempotente permite adotar o esquema do starter sem apagar registos e mantém o seed de cinco modalidades e sete variantes. `CREATE TABLE IF NOT EXISTS` não reconcilia alterações manuais incompatíveis; inspecionar a base se a adoção falhar. Acrescentar alterações em novos ficheiros (`002_...sql`, etc.); não alterar o histórico. Rever migrações, fazer backup e validar numa cópia antes de aplicar a dados importantes. Não há comando de rollback destrutivo automático.

A ordem é numérica, incluindo versões superiores a 999. Versões zero, duplicadas, nomes SQL inválidos e migrações acrescentadas abaixo da última versão aplicada são rejeitados. `.gitattributes` mantém SQL com finais de linha LF, evitando divergências de checksum entre checkouts Windows e Linux.

## Verificações

Na raiz, `sh scripts/check.sh` verifica formatação, análise estática, testes e build com Go instalado. Alternativamente:

```sh
cd backend
go mod download
go vet ./...
go test -race -count=1 ./...
go build ./...
```

Definir `DATABASE_TEST_URL` no processo de teste para ativar a integração PostgreSQL. Usar uma base exclusiva de testes, nunca produção nem uma base partilhada com dados pessoais. Os testes criam schemas isolados e dados sintéticos e preservam esses schemas; não limpam dados existentes. A ausência da variável deixa os testes de integração explicitamente ignorados. A suite valida a repetição das migrações, preservação de dados existentes, integridade do histórico e restrições de associação entre clubes; verificar os nomes efetivos no output da execução.

Com o Compose saudável, a partir da raiz:

```sh
python3 scripts/smoke.py
```

No Windows com Python instalado, usar `py scripts/smoke.py`. O smoke testa a API real, o catálogo esperado e erros 404/405; não basta um PID ou uma porta aberta. O OpenAPI em `docs/openapi.yaml` descreve apenas os endpoints implementados. As rotas propostas em `05_API_E_FLUXOS.md` continuam planeamento.

## CI

O workflow `.github/workflows/ci.yml` usa Go 1.26 e PostgreSQL 16. Gera uma credencial aleatória por execução, mascara-a nos logs e passa-a ao serviço de teste sem a versionar. Executa formatação, `go vet`, testes com race detector (incluindo integração), build e smoke da API real. O PostgreSQL de CI é descartável e só publica loopback; não usa a base nem os volumes de desenvolvimento.

Criar o workflow não prova uma execução no GitHub. O relatório da tarefa deve distinguir validação local concluída, checks ignorados e execução remota ainda por observar.

## Ambiente na nuvem

Cada tarefa já tem isolamento próprio: usar o checkout existente, sem criar worktrees salvo pedido explícito. Dependências e ficheiros podem ser retidos num snapshot; processos precisam de voltar a arrancar. As instruções guardadas no ambiente devem incluir diretório, arranque do Compose e pedidos de readiness. Uma configuração guardada ainda precisa de revisão e publicação pelo utilizador.
