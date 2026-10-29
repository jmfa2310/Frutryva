# Instalação Windows / PowerShell

## Ferramentas

Instalar Docker Desktop com WSL2 e containers Linux, Git e VS Code. Iniciar o Docker Desktop antes de continuar. Go 1.26 é opcional para trabalhar fora dos containers; Node.js e Blender serão necessários quando os respetivos módulos existirem.

Abrir a pasta do projeto, por exemplo `C:\Projetos\FUTRYVA_Plataforma_Multidesporto`, no VS Code. No terminal PowerShell, a partir dessa pasta:

```powershell
docker --version
docker compose version
git --version
```

## Configuração local

Criar `.env` apenas se ainda não existir. Não copiar credenciais para o Git.

```powershell
if (-not (Test-Path .env)) { Copy-Item .env.example .env }
notepad .env
```

Definir `DB_PASSWORD` com uma palavra-passe forte gerada pelo gestor de palavras-passe e guardar o ficheiro. O exemplo não contém uma credencial utilizável. A API recebe a palavra-passe numa variável PostgreSQL separada; caracteres reservados de URLs não são interpolados num URI.

Se já existir um volume PostgreSQL do starter, usar inicialmente a credencial desse volume. Mudar apenas `.env` não muda a palavra-passe interna do PostgreSQL. Uma rotação exige um procedimento próprio; apagar volumes não é solução.

## Arranque e confirmação

```powershell
docker compose up --build -d --wait
docker compose ps
Invoke-RestMethod http://localhost:8080/health
Invoke-RestMethod http://localhost:8080/v1/sports | Format-Table
```

O primeiro pedido deve devolver `status: ok`; o segundo, sete variantes das cinco modalidades. O primeiro build pode demorar. A API só inicia depois de ligar à base e aplicar as migrações. O health check confirma a disponibilidade da base e do esquema necessário. Não existe ainda uma interface web.

## Diagnóstico e paragem

```powershell
docker compose logs --tail=100 api
docker compose logs --tail=100 db
docker compose down
```

`down` preserva o volume de dados. Não usar `down -v`, `docker volume prune`, `DROP`, `TRUNCATE` ou scripts de reset. Se a porta 8080 estiver ocupada, parar o processo que a ocupa ou alterar conscientemente o mapeamento local antes de arrancar.

As migrações aplicam-se também a volumes existentes. `database/001_init.sql` é mantido como base histórica e seed; novas alterações entram em novos ficheiros numerados. Uma migração aplicada não deve ser editada: o checksum provoca uma falha explícita. Fazer backup antes de alterações de esquema em bases com dados importantes.

## Testes fora de containers

Com Go 1.26 instalado:

```powershell
Set-Location backend
go mod download
go vet ./...
go test -race -count=1 ./...
go build ./...
Set-Location ..
```

`-race` exige compilador C compatível no Windows. Em alternativa, executar a suite pelo Linux/WSL ou pelo CI. Para integração, configurar `DATABASE_TEST_URL` numa base exclusiva de testes, conforme [desenvolvimento](09_DESENVOLVIMENTO.md). Os testes ignorados por ausência dessa base não equivalem a testes aprovados.
