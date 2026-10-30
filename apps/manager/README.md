# FUTRYVA Manager

O teste de navegador completo está em `e2e/smoke.cjs`. Com o Compose iniciado, executar `npm ci`, `npx playwright install chromium` e `E2E_BOOTSTRAP_OWNER=1 npm run test:e2e`. Promove explicitamente apenas o dono sintético pelo CLI local e cria contas e clubes sintéticos e usa a caixa Mailpit local; não executar contra produção. `MANAGER_TEST_URL` e `MAILPIT_TEST_URL` permitem alterar os endereços. Em ambientes com Chromium do sistema, definir `CHROMIUM_PATH` para esse executável. As capturas em `test-results/` são ignoradas pelo Git.

Interface React/TypeScript da Sprint 1. Inclui registo/login, confirmação de email, recuperação de palavra-passe, conta/sessão, clubes, branding em rascunho, pré-visualização, publicação e histórico, membros e convites. Não inclui equipas, atletas, editor de treino, loja ou administração SaaS.

O frontend usa cookies HttpOnly através de pedidos na mesma origem; não guarda tokens no localStorage. O servidor continua responsável por todas as autorizações. Os temas dos clubes afetam apenas a pré-visualização; a identidade FUTRYVA do espaço de gestão mantém-se separada.

## Desenvolvimento

Usar Node.js 22.12+ ou 24 e executar nesta pasta:

```sh
npm ci
npm run dev
```

Vite encaminha `/v1` e `/health` para a API em `http://127.0.0.1:18080`. Para outra porta, definir `API_PROXY_TARGET` no processo de Vite. Em Docker, o Nginx serve a aplicação e encaminha os pedidos para `api:8080`.

```sh
npm test
npm run build
```

Os emails locais são entregues ao Mailpit configurado pelo Compose; abrir a caixa local na porta 18025 para copiar códigos de confirmação, recuperação e convite. Não partilhar os códigos. Não é uma integração de email de produção.

Os emblemas/capas aceitam ligações HTTPS ou imagens PNG, JPEG e WebP até 200 KB. O backend valida o conteúdo real. SVG não é permitido. Esta representação inicial será substituída por armazenamento de ficheiros no módulo correspondente.
