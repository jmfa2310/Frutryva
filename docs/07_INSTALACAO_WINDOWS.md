# Instalação e atualização Windows / PowerShell

## Ferramentas e pasta

Iniciar Docker Desktop com WSL2 e containers Linux. Confirmar no PowerShell:

```powershell
docker --version
docker compose version
```

Go, Node.js, Git e VS Code são opcionais para executar pelo Compose. No GitHub, selecionar a branch `sprint-1` e descarregar por **Code → Download ZIP** com sessão iniciada se o repositório for privado. Extrair o ZIP antes de usar os ficheiros. Não executar a partir de dentro do arquivo comprimido.

## Atualizar uma Sprint 0 existente

Manter a pasta atual e o seu `.env`: contém a credencial e, quando definido, `COMPOSE_PROJECT_NAME`, que identifica o volume de dados. Usar simplesmente uma pasta extraída com outro nome pode selecionar uma base nova em vez da existente.

Descarregar e extrair a Sprint 1 para uma pasta temporária, separada da instalação atual. Abrir o PowerShell na pasta extraída da Sprint 1 e executar o atualizador indicando a **pasta antiga que contém `docker-compose.yml` e `.env`**:

```powershell
Unblock-File .\scripts\update-windows.ps1
powershell.exe -NoProfile -ExecutionPolicy RemoteSigned -File .\scripts\update-windows.ps1 -ProjectPath "C:\caminho\da\instalacao\existente"
```

Substituir o caminho pelo real. Rever o atualizador antes de executar. `Unblock-File` desbloqueia apenas esse ficheiro descarregado; `RemoteSigned` aplica-se só ao processo iniciado, sem alterar a política global. Também é possível omitir `-ProjectPath` para selecionar a pasta existente numa janela.

Se a política já permitir executar o ficheiro local, basta:

```powershell
.\scripts\update-windows.ps1 -ProjectPath "C:\caminho\da\instalacao\existente"
```

O atualizador copia o código para a instalação existente sem substituir `.env`, Git ou dados de Docker, mantém o nome do projeto e prepara a porta API 18080 se ainda não estiver configurada. Ler o resultado do script e continuar o arranque na pasta existente. Não copiar o `.env.example` por cima do `.env` antigo e não alterar `DB_PASSWORD` para resolver erros de autenticação.

Antes de atualizar uma base importante, guardar um backup e confirmar como restaurá-lo. As novas migrações são aditivas e o SQL inicial não é substituído por um reset.

## Instalação nova

Na pasta extraída que contém `docker-compose.yml`:

```powershell
Test-Path .\docker-compose.yml
if (-not (Test-Path .env)) { Copy-Item .env.example .env }
notepad .env
```

O primeiro comando deve devolver `True`. Preencher `DB_PASSWORD` com uma palavra-passe forte e guardar. Os valores de porta recomendados são `API_PORT=18080`, `MANAGER_PORT=18081` e `MAIL_PORT=18025`. O Compose recusa arrancar sem a palavra-passe. Não partilhar `.env` nem o seu conteúdo.

## Arranque e confirmação

Na pasta da instalação:

```powershell
docker compose up --build -d --wait
docker compose ps
Invoke-RestMethod http://127.0.0.1:18080/health
Invoke-RestMethod http://127.0.0.1:18080/v1/sports | Format-Table
```

O health deve devolver `status: ok`; o catálogo deve conter sete variantes. Se as portas foram personalizadas no `.env`, ajustar os endereços. O primeiro build pode demorar vários minutos. Não continuar com testes HTTP se o comando de arranque terminar com erro.

Abrir a interface e a caixa de email local:

```powershell
Start-Process "http://127.0.0.1:18081"
Start-Process "http://127.0.0.1:18025"
```

No Manager: criar conta, consultar o email no Mailpit, copiar o código e confirmar a conta. O cliente recebe um único clube atribuído pelo dono da plataforma. O dono deve configurar a sua conta como descrito abaixo e usar Administração para criar/atribuir os clubes. O gestor edita a identidade visual em rascunho e publica quando a pré-visualização estiver correta. Para convidar outro utilizador, criar o convite, abrir o email do destinatário e aceitar o código na conta verificada desse email. A recuperação de acesso usa a mesma caixa local. Nenhuma destas mensagens é entregue a destinatários externos.

## Diagnóstico e portas ocupadas

```powershell
docker compose ps -a
docker compose logs --tail=80 api manager mailpit
docker ps --format "table {{.Names}}\t{{.Ports}}\t{{.Status}}"
```

Uma API em `Created` e outro container a ocupar a porta publicada indicam que o arranque pode ter falhado por conflito. Escolher outra porta no `.env`, sem parar aplicações de outros projetos, e repetir `docker compose up --build -d --wait`. Um `404` na porta de outra aplicação não valida a FUTRYVA.

Se PostgreSQL indicar falha de autenticação após atualização, confirmar que se manteve o `.env` e o mesmo `COMPOSE_PROJECT_NAME`. Mudar uma variável não muda a palavra-passe que já existe no volume. Não apagar o volume para resolver o problema.

## Paragem e próximo arranque

```powershell
docker compose stop
```

Para voltar a iniciar, na mesma pasta:

```powershell
docker compose up -d --wait
```

Não usar `down -v`, `docker volume prune`, `DROP`, `TRUNCATE` ou scripts de reset. `docker compose down`, sem `-v`, preserva o volume, mas `stop` é suficiente para a utilização diária.

## Testes fora de containers

Com Go 1.26 e Node.js 22 instalados:

```powershell
Set-Location backend
go mod download
go vet ./...
go test -race -count=1 ./...
go build ./...
Set-Location ..\apps\manager
npm ci
npm test
npm run build
Set-Location ..\..
```

`-race` exige compilador C compatível no Windows; pode ser executado em Linux/WSL ou no CI. Configurar `DATABASE_TEST_URL` numa base exclusiva de testes para ativar a integração. Os testes ignorados não são testes aprovados.

## Configurar o dono da plataforma

Na pasta da instalação existente, após atualizar e confirmar o email dessa conta:

```powershell
docker compose exec api /futryva owner set "EMAIL_DA_TUA_CONTA"
```

Substituir o argumento pelo email com que se registou. Não usar a palavra-passe nesse comando. Atualizar a página do Manager para aparecer Administração. É um comando local de operador, não uma opção disponível no registo.

Contas com um único clube anterior conservam esse clube; a navegação passa a usar as cores publicadas. Contas legadas com várias associações aguardam a escolha do dono, sem apagar dados. Ver [clube atribuído](12_CLUBE_ATRIBUIDO.md).
