# FUTRYVA — identidade e clubes

**FUTRYVA é um nome provisório.** Plataforma SaaS multiclube e multidesporto em desenvolvimento: Coach, Manager, Club e Admin. Futebol 11/9/7, futsal, voleibol, basquetebol e andebol partilham o mesmo núcleo.

## Estado real — Sprint 1

- API modular Go e PostgreSQL 16, migrações versionadas e catálogo de cinco modalidades/sete variantes.
- Registo, login, confirmação de email, recuperação de acesso, sessões revogáveis e rotação da sessão. Passwords com bcrypt; códigos e sessões guardados como hashes.
- Clubes com pertenças e papéis por utilizador, alteração de nome e arquivo lógico. Cada pedido privado valida a pertença ao clube no servidor.
- Identidade visual por clube: rascunho, pré-visualização, publicação e reversão com histórico. Cores, emblema, capa, patrocinadores e cores de equipamento; conteúdo público apenas após publicação.
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

No Manager, criar conta e abrir a caixa Mailpit para copiar o código de confirmação recebido. Voltar ao Manager, confirmar o email e criar o primeiro clube. A mesma caixa recebe códigos de recuperação de acesso e convites; os emails não são enviados para a Internet.

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
python3 scripts/smoke_sprint1.py --base-url http://127.0.0.1:18081 --mailpit-url http://127.0.0.1:18025
```

O smoke da Sprint 1 cria contas, clubes e emails sintéticos retidos. Executar numa instalação de desenvolvimento/testes. Ver [desenvolvimento](docs/09_DESENVOLVIMENTO.md), [entrega Sprint 1](docs/11_SPRINT1.md) e [OpenAPI](docs/openapi.yaml). Os documentos de produto descrevem também requisitos futuros; a presença na documentação não significa implementação.

## Próxima entrega

Sprint 2: modalidades, épocas, equipas, plantéis e atribuição de treinadores, mantendo os testes contra acesso cruzado entre clubes. O editor 2D/3D e o comércio continuam nas fases posteriores do roadmap.
