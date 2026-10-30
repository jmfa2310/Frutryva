# Um clube atribuído por conta

## Regra atual

Por decisão do dono do produto, cada utilizador utiliza um único clube atribuído. Esta regra substitui a seleção multiclube descrita nas versões anteriores do planeamento. A plataforma continua a alojar vários clubes, mas uma conta comum não escolhe nem cria clubes livremente.

O dono da plataforma cria o clube e atribui o respetivo gestor. Um gestor trata do clube recebido, da sua identidade visual e dos convites autorizados. A distinção é validada no backend; ocultar um botão no Manager não é a proteção de acesso.

## Preparar a conta do dono

Criar uma conta e confirmar o email através da caixa Mailpit local. Um operador com acesso administrativo ao ambiente promove essa conta pelo CLI:

```sh
docker compose exec -T api /futryva owner set email-da-conta-verificada
```

Substituir o último argumento pelo email real da conta verificada. O CLI exige acesso ao container e à base configurada; registo/login não aceitam uma opção para obter este privilégio. Atualizar a página ou voltar a entrar depois da promoção. Não promover automaticamente a primeira conta registada.

A área administrativa permite listar, criar, atribuir e arquivar clubes. O email do gestor deve corresponder a uma conta já existente e verificada. Se a conta já tiver outro clube, a operação falha com `409 user_has_club`; se outro gestor já ocupar o clube, falha com `409 club_has_manager`. Não há transferência silenciosa de utilizadores nem remoção de dados para contornar estes conflitos.

O dono não recebe pertença automática aos clubes que cria. A capacidade administrativa de atribuição não concede leitura dos dados operacionais privados de clientes.

## Contas e acesso

`GET /v1/me` devolve `isPlatformOwner` e, quando existir, `assignedClubId`. `GET /v1/clubs` devolve zero ou um clube. Para consultar ou alterar um recurso privado, a API exige simultaneamente que a conta esteja atribuída ao clube e tenha a pertença/papel necessário nesse clube.

Uma conta sem atribuição vê um estado de espera e não um seletor de clubes. As rotas antigas de criação e arquivo em `/v1/clubs` estão desativadas para contas autenticadas; as operações pertencem a `/v1/admin/clubs`. Os endpoints públicos mantêm apenas a identidade explicitamente publicada.

Os convites de treinador/assistente/diretor continuam associados a um email verificado. A aceitação permite uma conta sem clube ou já atribuída ao mesmo clube. Se a conta tiver outro clube, a API rejeita a aceitação sem consumir o código e sem mudar a atribuição existente. O gestor pode revogar esse convite e convidar uma conta adequada. Aceitar um convite não concede gestão global nem acesso a um segundo clube.

## Cores do menu e escolha de tema

O menu do Manager acompanha as cores **publicadas** do clube atribuído. Editar um rascunho altera a pré-visualização, mas não a identidade do menu que representa a versão em uso. Publicar atualiza as cores; restaurar uma versão cria nova publicação e atualiza o menu para o tema restaurado.

A seleção oferece 48 cores nomeadas, 12 combinações prontas e valores personalizados para principal, secundária, destaque, fundo e texto. O backend mantém a regra de contraste mínimo 4,5:1 entre texto e fundo; selecionar uma cor não autoriza publicar um tema ilegível. A marca FUTRYVA permanece identificável, separada do emblema e nome do clube.

## Migração de instalações anteriores

`003_managed_club_assignment.sql` acrescenta o privilégio de dono e a atribuição única sem eliminar clubes, utilizadores ou pertenças existentes. As contas que tinham exatamente um clube são atribuídas a esse clube. As contas legadas com mais de um clube mantêm o histórico e aguardam uma decisão explícita do dono; não recebem acesso multiclube por omissão.

Não substituir `.env`, alterar o nome do projeto ou apagar volumes para atualizar. As migrações são executadas pelo arranque normal e preservam o SQL histórico. Se uma instalação tiver dados importantes, fazer backup e validar a atualização numa cópia.

## Testes reproduzíveis

O teste HTTP integrado cria contas sintéticas, confirma os emails no Mailpit e promove explicitamente apenas o dono de teste:

```sh
python3 scripts/smoke_sprint1.py --bootstrap-owner
```

Por segurança, é obrigatório escolher `--bootstrap-owner` ou `--owner-binary /caminho/absoluto/futryva`. A segunda opção é destinada a CI/local com um binário confiável e configuração PostgreSQL herdada; não aceita comandos shell arbitrários. O teste não imprime emails, passwords, códigos ou cookies. Mantém os registos sintéticos e emails na instalação de desenvolvimento.

O teste de navegador usa `E2E_BOOTSTRAP_OWNER=1` para o CLI do Compose ou `E2E_OWNER_BINARY` com o mesmo objetivo em CI. A execução verifica provisionamento pelo dono, ausência de criação self-service, clube único atribuído, identidade publicada no menu, rascunho sem efeito no menu, publicação/restauro e aceitação móvel de um convite de treinador. O resultado de uma execução deve ser reportado apenas depois de observado.

## Validação de 30 outubro 2026

- Backend: 44 testes principais, 125 resultados aprovados incluindo subtestes, sem falhas nem ignorados, com PostgreSQL real e deteção de corridas. Inclui migração preservando dados, dono explícito, atribuição única, conflitos de convites e casos de pertenças históricas.
- Manager: 19 testes de componentes/contraste/paleta aprovados e build TypeScript/Vite aprovado.
- Smoke HTTP: 10 grupos funcionais aprovados, com SMTP local e bootstrap do dono pelo CLI.
- Chromium: quatro grupos de fluxos aprovados em desktop/móvel: administração, paleta por teclado, emblema, publicação/restauro, menu independente do rascunho, convites, isolamento e persistência após login. Sem erros JavaScript nem overflow horizontal móvel.
- As migrações 001 e 002 foram comparadas com a versão anterior e permanecem idênticas. A atualização é a migração aditiva 003.

Os resultados são da instância de desenvolvimento. O CI remoto e a nova atualização no Windows do utilizador não são presumidos a partir destes testes.
