# Sprint 1 — identidade, clubes e Manager

## Âmbito implementado

A Sprint 1 acrescenta um fluxo local integrado: criar conta, confirmar email, entrar, recuperar acesso, criar/alternar clubes, preparar identidade visual e publicá-la, convidar utilizadores e aceitar convites por email. A primeira interface é Manager React/TypeScript; a API e PostgreSQL mantêm a fundação e as modalidades da Sprint 0.

O schema novo é aditivo. Contas, clubes e catálogo existentes não são eliminados. O nome e a identidade da plataforma permanecem separados do tema de cada clube.

## Fluxo de utilização

1. Arrancar o Compose e abrir Manager na porta local 18081.
2. Registar conta com nome, email e palavra-passe (12–72 bytes).
3. Abrir Mailpit na porta 18025, copiar o código recebido e confirmar o email no Manager.
4. Criar clube; o criador recebe a função de presidente. Um utilizador pode pertencer a vários clubes e selecioná-los na mesma conta.
5. Editar nome e identidade visual: cores, emblema, capa, patrocinadores e equipamento. Guardar o rascunho e verificar a pré-visualização antes de publicar.
6. Publicar para tornar a identidade visível no endpoint público. Um novo rascunho permanece privado até nova publicação. Reverter para uma versão anterior cria um novo registo no histórico.
7. Convidar treinador/assistente/diretor; o destinatário regista-se ou entra, verifica o email e aceita o código na sua conta. A aceitação exige email igual ao do convite.
8. Usar recuperação de acesso através de outro código Mailpit; redefinir a palavra-passe invalida sessões anteriores.

A caixa de email pertence à instalação de desenvolvimento: não existe envio para destinatários externos. O adapter SMTP fornecido não aceita hosts externos. Não expor a caixa na Internet.

## Permissões e limites

| Operação | Presidente | Diretor | Treinador/assistente |
|---|---|---|---|
| Consultar o clube a que pertence | Sim | Sim | Sim |
| Alterar nome e branding | Sim | Sim | Não |
| Ver rascunho/histórico, membros e convites | Sim | Sim | Não |
| Convidar treinador/assistente | Sim | Sim | Não |
| Convidar/revogar convite de diretor | Sim | Não | Não |
| Arquivar clube | Sim | Não | Não |

Criar clube e executar escritas exige email verificado. Uma conta de outro clube não obtém acesso por conhecer um UUID. Dados de membros, rascunhos e convites não aparecem nos endpoints públicos. Arquivar não elimina dados nem concede acesso a outros clubes.

Sessões duram 24 horas e são revogáveis. Confirmação de email dura 24 horas; recuperação, 30 minutos; convites, sete dias. Reutilização, expiração e destinatário errado são rejeitados. Convites pendentes não são reenviados implicitamente; revogar o anterior antes de emitir outro.

O branding exige cores hexadecimais e contraste texto/superfície de pelo menos 4,5:1. Emblema/capa aceitam URLs HTTPS ou PNG/JPEG/WebP estático embutido, até 200 KiB e 16 megapixels por imagem. SVG não é aceite; não há ainda biblioteca de media S3/MinIO.

## Ficheiros e contratos

- `database/002_identity_and_clubs.sql`: contas verificadas, sessões, códigos, drafts/versões de branding, arquivo e convites.
- `backend/internal/identity/`: autenticação, cookies, códigos e SMTP local.
- `backend/internal/organizations/`: clubes, pertenças, branding e convites.
- `backend/internal/httpapi/`: composição HTTP e proteção de pedidos.
- `apps/manager/`: interface, cliente API, testes, build e proxy para a API.
- `docker-compose.yml` e `.env.example`: API 18080, Manager 18081 e Mailpit 18025, configuráveis e só em loopback.
- `scripts/update-windows.ps1`: atualização na pasta existente, preservando configuração e dados.
- `scripts/smoke_sprint1.py`, suites Go/React, workflow CI e `docs/openapi.yaml`: validação e contrato dos endpoints implementados.

O relatório final da execução deve listar os comandos e resultados observados. Este documento descreve o comportamento implementado e não presume que o CI remoto foi executado.

## Validação desta entrega — 29 outubro 2026

- Backend: `go vet -mod=readonly ./...` e build aprovados; suite com PostgreSQL 16 real, `go test -mod=readonly -race -count=1 -json ./...`: **37 testes principais, 118 resultados aprovados incluindo subtestes, zero falhas e zero ignorados**.
- Manager: `npm ci`, `npm test` (**7 testes**) e `npm run build` (TypeScript/Vite) aprovados.
- API real com PostgreSQL e Mailpit: `python3 scripts/smoke.py` e `python3 scripts/smoke_sprint1.py` aprovados; o segundo verifica **9 grupos funcionais** incluindo recuperação, isolamento, publicação e convites.
- Chromium real: `CHROMIUM_PATH=/usr/bin/chromium npm run test:e2e`, em `apps/manager`, aprovado. Verifica registo/confirmacão, criação de dois clubes, emblema, publicação/restauro, convites, aceitação em viewport móvel de 390 px, leitura por treinador, logout/login e ausência de erros JavaScript. O ecrã móvel não apresentou overflow horizontal.
- Atualizador PowerShell: `pwsh -NoProfile -File scripts/test-update-windows.ps1` aprovado com ficheiros sintéticos e Docker simulado. Verificou repetição, backups distintos, preservação de `.env`, `COMPOSE_PROJECT_NAME` e ficheiros do utilizador. Foi executado com PowerShell 7.5.4 em Linux; o seletor gráfico e Docker Desktop em Windows ainda dependem da confirmação local do utilizador.
- Upgrade no volume existente do ambiente: migrações 001/002 registadas, catálogo preservado (cinco modalidades/sete variantes), quatro serviços saudáveis e SQL inicial idêntico ao da Sprint 0. Preparação completa repetida sem reset.

O workflow de CI está preparado, mas estes resultados são locais; uma execução no GitHub só pode ser afirmada após observar o resultado remoto.

## Atualizar sem perder a instalação existente

Para instalações obtidas por ZIP, descarregar a Sprint 1 pelo GitHub com sessão iniciada, extrair para outra pasta e executar o script de atualização indicando a pasta existente. Preservar `.env`, `DB_PASSWORD` e `COMPOSE_PROJECT_NAME`: mudar a identidade do projeto pode selecionar um novo volume. Seguir [as instruções PowerShell](07_INSTALACAO_WINDOWS.md).

A porta API passa a ter valor configurável com omissão 18080, evitando o conflito com outros projetos na 8080. Manager e Mailpit têm portas próprias. Não parar containers de outras aplicações para libertar portas sem uma decisão explícita do utilizador.

## Ainda por desenvolver

Sprint 2 acrescentará épocas, equipas, plantéis e atribuição de treinadores por equipa. Treinos, jogos/estatísticas, editor 2D/3D, loja/pagamentos/subscrições e as aplicações Coach/Club/Admin completas seguem o roadmap. A publicação atual é da identidade do clube, não de resultados, atletas ou conteúdo editorial.

MFA, email de produção, políticas operacionais de auditoria/retensão, backups e restauro, testes de carga e revisão RGPD/jurídica continuam requisitos antes de comercialização. Não há nesta entrega uma garantia de segurança ou prontidão de produção.

O limitador de autenticação é local ao processo (60 pedidos/minuto por IP do transporte); clientes via Nginx partilham esse limite no desenvolvimento. Antes de exposição pública, configurar proxies confiáveis e limites distribuídos, quotas de convites e entrega de email desacoplada, incluindo mitigação de diferenças temporais na recuperação de contas.
