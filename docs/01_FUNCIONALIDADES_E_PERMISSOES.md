# Funcionalidades, papéis e regras

## Coach
Conta própria ou convite por clube; equipas e épocas; plantéis; presenças; calendário de treinos; biblioteca de exercícios; editor com campo e material; animação por keyframes; vista 2D/3D; formações, convocatórias, jogos, eventos e estatísticas; relatórios PDF; notas privadas e partilha seletiva.

## Manager (presidente/direção)
Criar/configurar clube; identidade visual; modalidades, escalões, equipas, treinadores e permissões; publicar conteúdo; gerir produtos, stock, encomendas, reembolsos e indicadores de vendas; seguidores e métricas agregadas; sócios/quotas como módulo futuro; dashboard desportivo. O presidente não obtém por defeito acesso a notas privadas ou informação médica.

## Club (adeptos)
Seguir vários clubes; navegar modalidades/equipas; jogos, resultados, plantéis e estatísticas aprovadas; onze inicial/convocatória quando publicado; notícias, notificações opt-in; loja e histórico de compras; votação e sócios como evoluções. Adeptos sem conta podem ver conteúdo público, se o clube permitir.

## Admin (operador SaaS)
Clientes, planos, subscrições, períodos de teste, limites, suporte, métricas, auditoria e incidentes. Acesso de suporte a dados de clientes só por processo autorizado, temporário e auditado.

## Identidades e convites
- Email + palavra-passe ou login social opcional; email verificado; recuperação de acesso; sessões revogáveis.
- Presidente envia convite de uso único com validade e função/equipa definida; treinador aceita na sua conta existente ou cria uma.
- Utilizador pode alternar clube/equipa; permissões validadas no servidor em cada pedido.
- Treinador independente pode criar biblioteca privada e equipa pessoal; ao mudar de clube, exercícios pessoais continuam seus, dados do clube não.

## Matriz resumida
| Ação | Admin SaaS | Presidente | Treinador | Adepto |
|---|---|---|---|---|
| Criar clube/gerir subscrição | condicionado | sim | não | não |
| Alterar branding do clube | não por defeito | sim | não | não |
| Convidar treinadores | não por defeito | sim | conforme delegação | não |
| Gerir treinos da equipa | não | leitura autorizada | sim | não |
| Publicar resultados | não | sim/delegação | se autorizado | não |
| Ver conteúdo público | sim | sim | sim | sim |
| Comprar na loja | sim | sim | sim | sim |

## Regras de publicação
Dados operacionais de jogo são privados por defeito; resultados, alinhamentos, fotos e estatísticas só entram na app pública após publicação e verificação das autorizações adequadas, sobretudo para menores.
