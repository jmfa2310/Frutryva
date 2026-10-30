# FUTRYVA — identidade e clubes

**FUTRYVA é um nome provisório.** Plataforma SaaS multiclube e multidesporto em desenvolvimento: Coach, Manager, Club e Admin. Futebol 11/9/7, futsal, voleibol, basquetebol e andebol partilham o mesmo núcleo.

## Estado real — Sprint 1

- API modular Go e PostgreSQL 16, migrações versionadas e catálogo de cinco modalidades/sete variantes.
- Registo, login, confirmação de email, recuperação de acesso, sessões revogáveis e rotação da sessão. Passwords com bcrypt; códigos e sessões guardados como hashes.
- Clubes criados e atribuídos pelo dono da plataforma. Cada utilizador do Manager tem um único clube; todos os pedidos privados validam a atribuição e a pertença. Alteração de nome pelos gestores e arquivo pelo dono.
- Identidade visual por clube: rascunho, pré-visualização, publicação e reversão com histórico. Paleta de cores e combinações prontas, emblema, capa, patrocinadores e equipamento. O menu e os botões do Manager cliente acompanham as cores publicadas do clube; o rascunho não altera a navegação.
- Convites por email, com validade e uso único; aceitação pela conta verificada do destinatário. Presidente/direção gerem convites e membros; treinadores não gerem o branding.
- Interface Manager React/TypeScript e caixa de email **local** Mailpit para testar os fluxos completos.

Equipas, jogadores, treinos, jogos, estatísticas, editor 2D/3D, loja, pagamentos, subscrições e as restantes aplicações continuam por desenvolver. Esta entrega é um ambiente de desenvolvimento, não um lançamento comercial: não inclui fornecedor de email de produção, MFA, operação de backups ou revisão jurídica/RGPD.

## Iniciar com Docker

Requer Docker Desktop com containers Linux ou Docker Engine e Compose v2. Na raiz, criar `.env` a partir de `.env.example` apenas se não existir e definir `DB_PASSWORD`. Não substituir credenciais ou `COMPOSE_PROJECT_NAME` de uma instalação existente.

```sh
docker compose up --build -d --wait
docker compose ps
```

Portas de desenvolvimento, publicadas apenas em loopback:

| Serviço | Endereço local | Variável opcional |
|---|---|---|
| Manager | `http://127.0.0.1:18081` | `MANAGER_PORT` |
| API | `http://127.0.0.1:18080` | `API_PORT` |
| Caixa de email Mailpit | `http://127.0.0.1:18025` | `MAIL_PORT` |

O PostgreSQL permanece na rede interna do Compose. A API usa 8080 dentro do container; não é necessário ocupar a porta 8080 do Windows. Se outra aplicação usar uma das portas publicadas, alterar a variável correspondente no `.env` e repetir o arranque.

No Manager, criar conta e abrir a caixa Mailpit para copiar o código de confirmação recebido. Voltar ao Manager e confirmar o email. O dono da plataforma cria e atribui o clube na área Administração; uma conta sem atribuição aguarda o responsável. A mesma caixa recebe códigos de recuperação de acesso e convites; os emails não são enviados para a Internet.

Para instalação ou atualização preservando a base existente, seguir [Windows/PowerShell](docs/07_INSTALACAO_WINDOWS.md).

```sh
docker compose logs --tail=100 api manager mailpit
docker compose stop
```

`stop` mantém os serviços e dados para o próximo arranque. Não usar `down -v`, `prune` ou reset de dados. Alterar `DB_PASSWORD` no `.env` não altera a credencial já guardada pelo PostgreSQL.

## Desenvolvimento e validação

Go 1.26, Node.js 22 e npm são usados para desenvolvimento fora dos containers. Não precisam de estar instalados no Windows para arrancar pelo Compose.

```sh
sh scripts/check.sh
cd apps/manager
npm ci
npm test
npm run build
```

Os testes Go de integração exigem `DATABASE_TEST_URL` numa base exclusiva de testes. Sem essa variável são explicitamente ignorados; testes unitários não substituem a integração.

Com os serviços saudáveis, estes comandos fazem pedidos reais:

```sh
python3 scripts/smoke.py http://127.0.0.1:18080
python3 scripts/smoke_sprint1.py --bootstrap-owner --base-url http://127.0.0.1:18081 --mailpit-url http://127.0.0.1:18025
```

O smoke da Sprint 1 cria contas, clubes e emails sintéticos retidos. Executar numa instalação de desenvolvimento/testes. Ver [desenvolvimento](docs/09_DESENVOLVIMENTO.md), [entrega Sprint 1](docs/11_SPRINT1.md) e [clube atribuído e cores](docs/12_CLUBE_ATRIBUIDO.md) e [OpenAPI](docs/openapi.yaml). Os documentos de produto descrevem também requisitos futuros; a presença na documentação não significa implementação.

## Conta do dono da plataforma

Após registar e confirmar o email do dono, promover explicitamente essa conta no ambiente local:

```sh
docker compose exec api /futryva owner set EMAIL_DA_CONTA_CONFIRMADA
```

Voltar a abrir a aplicação: a conta do dono entra diretamente na área FUTRYVA Admin, com cores fixas da plataforma, sem O meu clube ou identidade de clientes. Um registo normal nunca obtém este privilégio. A conta do dono não pode ser atribuída como responsável de um clube. Clientes existentes com um único clube mantêm a atribuição automaticamente. Contas de dono anteriormente associadas a um clube são separadas pela migração 004 sem apagar esse clube ou o histórico; atribuir depois um responsável cliente na Administração.

## Próxima entrega

Sprint 2: modalidades, épocas, equipas, plantéis e atribuição de treinadores, mantendo os testes contra acesso cruzado entre clubes. O editor 2D/3D e o comércio continuam nas fases posteriores do roadmap.
