# Um clube atribuído por conta

## Regra atual

Por decisão do dono do produto, cada conta de cliente utiliza um único clube atribuído. A conta do dono da plataforma é uma conta administrativa separada, sem clube. Esta regra substitui a seleção multiclube descrita nas versões anteriores do planeamento. A plataforma continua a alojar vários clubes, mas uma conta comum não escolhe nem cria clubes livremente.

O dono da plataforma cria o clube e atribui o respetivo gestor. Um gestor trata do clube recebido, da sua identidade visual e dos convites autorizados. A distinção é validada no backend; ocultar um botão no Manager não é a proteção de acesso.

## Preparar a conta do dono

Criar uma conta e confirmar o email através da caixa Mailpit local. Um operador com acesso administrativo ao ambiente promove essa conta pelo CLI:

```sh
docker compose exec -T api /futryva owner set email-da-conta-verificada
```

Substituir o último argumento pelo email real da conta verificada. O CLI exige acesso ao container e à base configurada; registo/login não aceitam uma opção para obter este privilégio. Atualizar a página ou voltar a entrar depois da promoção. Não promover automaticamente a primeira conta registada.

A área administrativa permite listar, criar, atribuir e arquivar clubes. O email do gestor deve corresponder a uma conta já existente e verificada. Se a conta já tiver outro clube, a operação falha com `409 user_has_club`; se outro gestor já ocupar o clube, falha com `409 club_has_manager`. Não há transferência silenciosa de utilizadores nem remoção de dados para contornar estes conflitos.

O dono não pode acumular o papel de cliente ou gestor de clube na mesma conta. A atribuição administrativa de um dono como gestor é rejeitada com `409 platform_owner_cannot_manage_club`, mesmo quando esse dono usa a própria API administrativa. Para gerir um clube como cliente é necessária outra conta. A capacidade administrativa de atribuição não concede leitura dos dados operacionais privados de clientes.

## Contas e acesso

`GET /v1/me` devolve `isPlatformOwner` e, para um cliente atribuído, `assignedClubId`. Para o dono, `assignedClubId` é omitido e `GET /v1/clubs` devolve sempre uma lista vazia, incluindo quando existe uma pertença histórica. Para clientes, `GET /v1/clubs` devolve zero ou um clube. Para consultar ou alterar um recurso privado, a API exige simultaneamente que a conta esteja atribuída ao clube e tenha a pertença/papel necessário nesse clube.

Uma conta de cliente sem atribuição vê um estado de espera e não um seletor de clubes. O dono entra diretamente numa aplicação administrativa neutra, com a marca FUTRYVA ADMIN, navegação de clubes da plataforma e conta administrativa. Não vê “O meu clube”, identidade visual, emblema, convites ou mensagem de clube por atribuir. A aplicação administrativa não consulta `/v1/clubs`. As rotas antigas de criação e arquivo em `/v1/clubs` estão desativadas para contas autenticadas; as operações pertencem a `/v1/admin/clubs`. Os endpoints públicos mantêm apenas a identidade explicitamente publicada.

Os convites de treinador/assistente/diretor continuam associados a um email verificado de cliente. O dono não pode aceitar convites para clubes: a API devolve `409 platform_owner_cannot_manage_club` sem consumir o código. A aceitação permite uma conta sem clube ou já atribuída ao mesmo clube. Se a conta tiver outro clube, a API rejeita a aceitação sem consumir o código e sem mudar a atribuição existente. O gestor pode revogar esse convite e convidar uma conta adequada. Aceitar um convite não concede gestão global nem acesso a um segundo clube.

## Cores do menu e escolha de tema

O menu do Manager **dos clientes** acompanha as cores **publicadas** do clube atribuído. O FUTRYVA ADMIN conserva sempre as cores e a marca da plataforma, mesmo depois de criar clubes com temas diferentes ou de voltar a entrar. Editar um rascunho altera a pré-visualização, mas não a identidade do menu que representa a versão em uso. Publicar atualiza as cores; restaurar uma versão cria nova publicação e atualiza o menu para o tema restaurado.

A seleção oferece 48 cores nomeadas, 12 combinações prontas e valores personalizados para principal, secundária, destaque, fundo e texto. O backend mantém a regra de contraste mínimo 4,5:1 entre texto e fundo; selecionar uma cor não autoriza publicar um tema ilegível. A marca FUTRYVA permanece identificável, separada do emblema e nome do clube.

## Migração de instalações anteriores

`003_managed_club_assignment.sql` acrescenta o privilégio de dono e a atribuição única sem eliminar clubes, utilizadores ou pertenças existentes. As contas que tinham exatamente um clube são atribuídas a esse clube. As contas legadas com mais de um clube mantêm o histórico e aguardam uma decisão explícita do dono; não recebem acesso multiclube por omissão.

`004_platform_owner_separation.sql` remove apenas a atribuição de clube das contas já promovidas a dono e regista essa separação no histórico administrativo. Mantém clubes, temas e pertenças históricas; estas pertenças deixam de conferir acesso privado ao dono. O CLI `owner set` aplica a mesma separação a promoções novas. Se um clube antigo ficar sem responsável cliente, o dono pode atribuí-lo a outra conta verificada pela administração. Não é necessário apagar ou recriar esse clube.

Não substituir `.env`, alterar o nome do projeto ou apagar volumes para atualizar. As migrações são executadas pelo arranque normal e preservam o SQL histórico. Se uma instalação tiver dados importantes, fazer backup e validar a atualização numa cópia.

## Testes reproduzíveis

O teste HTTP integrado cria contas sintéticas, confirma os emails no Mailpit e promove explicitamente apenas o dono de teste:

```sh
python3 scripts/smoke_sprint1.py --bootstrap-owner
```

Por segurança, é obrigatório escolher `--bootstrap-owner` ou `--owner-binary /caminho/absoluto/futryva`. A segunda opção é destinada a CI/local com um binário confiável e configuração PostgreSQL herdada; não aceita comandos shell arbitrários. O teste não imprime emails, passwords, códigos ou cookies. Mantém os registos sintéticos e emails na instalação de desenvolvimento.

O teste de navegador usa `E2E_BOOTSTRAP_OWNER=1` para o CLI do Compose ou `E2E_OWNER_BINARY` com o mesmo objetivo em CI. A execução verifica administração neutra e separada do dono, ausência de pedidos ao contexto privado do cliente nessa aplicação, provisionamento, ausência de criação self-service, clube único por cliente, identidade publicada no menu do cliente, rascunho sem efeito no menu, publicação/restauro e aceitação móvel de um convite de treinador. O resultado de uma execução deve ser reportado apenas depois de observado.

## Validação anterior — antes da separação administrativa 004

Os resultados abaixo correspondem à revisão anterior com a migração 003. Não comprovam por si só a nova separação administrativa; esta revisão exige repetir os testes HTTP e de navegador com os novos casos de dono sem clube.

- Backend: 44 testes principais, 125 resultados aprovados incluindo subtestes, sem falhas nem ignorados, com PostgreSQL real e deteção de corridas. Inclui migração preservando dados, dono explícito, atribuição única, conflitos de convites e casos de pertenças históricas.
- Manager: 19 testes de componentes/contraste/paleta aprovados e build TypeScript/Vite aprovado.
- Smoke HTTP: 10 grupos funcionais aprovados, com SMTP local e bootstrap do dono pelo CLI.
- Chromium: quatro grupos de fluxos aprovados em desktop/móvel: administração, paleta por teclado, emblema, publicação/restauro, menu independente do rascunho, convites, isolamento e persistência após login. Sem erros JavaScript nem overflow horizontal móvel.
- As migrações 001 e 002 foram comparadas com a versão anterior e permanecem idênticas. A atualização então validada era a migração aditiva 003.

Os resultados são da instância de desenvolvimento. O CI remoto e a nova atualização no Windows do utilizador não são presumidos a partir destes testes.

## Validação da separação Admin/Manager — migração 004

- Backend com PostgreSQL real e deteção de corridas: **48 testes principais, 131 resultados aprovados incluindo subtestes, zero falhas e zero ignorados**. Inclui preservação de clube/branding/histórico, auditoria idempotente, rejeição de atribuições e convites ao dono, ponteiros antigos e promoção concorrente com atribuição.
- Manager/Admin: **21 testes de componentes aprovados** e build TypeScript/Vite aprovado. O teste de owner com dados antigos de clube confirma que não há consulta de rotas de cliente nem aplicação de tema.
- API real: **11 grupos funcionais aprovados**, com email local e bootstrap administrativo explícito.
- Chromium real: quatro grupos de fluxos aprovados, incluindo Admin neutro sem O meu clube, sem convites e sem pedidos a /v1/clubs, mesmo após publicação de temas distintos por clientes. Clientes mantêm os temas e o fluxo móvel sem erros JavaScript ou overflow horizontal.
- Migrações 001, 002 e 003 permanecem idênticas à versão anterior; a única nova migração é a 004.

A atualização no Windows do utilizador e o CI remoto continuam a ser verificados separadamente.
