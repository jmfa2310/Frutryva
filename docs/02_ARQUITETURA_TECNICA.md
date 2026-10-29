# Arquitetura e stack

## Escolha inicial
Monorepo com quatro frontends React/TypeScript (PWA responsiva), backend modular Go, PostgreSQL, armazenamento S3 compatível (MinIO local), Docker Compose em desenvolvimento, CI e testes. Considerar React Native/Expo após validar PWA e necessidades nativas. Não iniciar com microsserviços.

## Diagrama lógico
```text
Coach / Manager / Club / Admin (React)
               | HTTPS
         API Go modular
               |
     PostgreSQL + S3/MinIO
               |
        workers/filas (futuro)
```

## Módulos do backend
identity, organizations, memberships, sports, teams, athletes, training, tactics, matches, publishing, commerce, billing, notifications, audit. Dependências explícitas; APIs versionadas `/v1`.

## Multitenancy
ClubId em recursos do clube; validação do clube ativo e pertença em TODAS as queries/escritas; constraints compostas e testes contra acesso cruzado. Para conteúdo pessoal de treinadores usar ownership independente. Evitar confiar em clubId enviado pelo cliente. Possível RLS PostgreSQL como defesa adicional.

## Branding
Tema publicado e rascunho por clube: `primaryColor`, `secondaryColor`, `accentColor`, `surfaceColor`, `textColor`, `logoAssetId`, `coverAssetId`, `sponsors`, `kitColors`, `version`. CSS variables no frontend; acessibilidade WCAG; cache invalidada por versão; rollback.

## Segurança
TLS; hash de palavra-passe Argon2id ou bcrypt com parâmetros atuais; tokens de curta duração e refresh revogável, idealmente cookies seguros HttpOnly no web; MFA para dirigentes/admin; rate limiting; validação e sanitização; uploads com limites e varrimento; segregação de ambientes; gestão de segredos; auditoria; backups cifrados e restauro testado.

## Dados pessoais
Minimização; perfis e consentimentos adequados; proteção reforçada de menores; autorização de imagem; política de retenção; exportação/apagamento; contratos de tratamento e RGPD; dados médicos restritos, se vierem a existir. Revisão jurídica antes de comercialização.

## Operação
Dev: Docker Compose. Produção: HTTPS, backups automáticos, logs sem dados sensíveis, métricas, alertas, migrações versionadas, ambiente staging, CI/CD e rollback. Nunca usar credenciais de desenvolvimento em produção.
