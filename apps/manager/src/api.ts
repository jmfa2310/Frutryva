export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string) { super(message); }
}

const messages: Record<string, string> = {
  invalid_credentials: 'O email ou a palavra-passe não estão corretos.',
  email_taken: 'Já existe uma conta com este email. Tente iniciar sessão.',
  email_exists: 'Já existe uma conta com este email. Tente iniciar sessão.',
  email_in_use: 'Já existe uma conta com este email. Tente iniciar sessão.',
  email_not_verified: 'Confirme primeiro o seu email para continuar.',
  forbidden: 'A sua função não permite realizar esta ação.',
  platform_owner_required: 'Apenas o dono da plataforma pode criar, atribuir ou arquivar clubes.',
  user_has_club: 'Esta conta já tem outro clube atribuído. Contacte o responsável pela plataforma.',
  club_has_manager: 'Este clube já tem um responsável atribuído.',
  manager_not_found: 'O responsável precisa de criar uma conta antes da atribuição.',
  manager_not_verified: 'O responsável precisa de confirmar o email antes da atribuição.',
  not_found: 'Este recurso já não está disponível ou não tem acesso.',
  invalid_token: 'O código é inválido, já foi utilizado ou expirou.',
  token_expired: 'Este código expirou. Peça um novo para continuar.',
  rate_limited: 'Foram feitas muitas tentativas. Aguarde alguns minutos.',
  mail_unavailable: 'Não foi possível enviar o email. Tente novamente dentro de momentos.',
  email_unavailable: 'Não foi possível enviar o email. Tente novamente dentro de momentos.',
  invalid_theme: 'Verifique as cores, o contraste e as imagens do clube. As ligações devem usar HTTPS e as imagens devem ter até 200 KB.',
  invalid_request: 'Reveja os campos preenchidos e tente novamente.',
  unsupported_media_type: 'O formato enviado não é suportado.',
  already_member: 'Esta pessoa já pertence ao clube.',
  invitation_pending: 'Já existe um convite pendente para este email.',
  invalid_invitation: 'O convite é inválido, expirou, já foi utilizado ou pertence a outro email.',
};

export function errorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 401) return messages[error.code] || 'A sessão terminou. Inicie sessão novamente.';
    if (messages[error.code]) return messages[error.code];
    if (error.status === 400 || error.status === 422) return error.message || 'Reveja os campos preenchidos.';
    if (error.status === 403) return 'Não tem permissão para realizar esta ação.';
    if (error.status === 409) return 'Existe um registo em conflito. Atualize a página e tente novamente.';
    return 'Não foi possível concluir o pedido. Tente novamente.';
  }
  return 'Não foi possível contactar o serviço. Verifique a ligação e tente novamente.';
}

export async function api<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
  const response = await fetch(path, {
    method, credentials: 'same-origin',
    headers: body === undefined ? { Accept: 'application/json' } : { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (response.status === 204) return undefined as T;
  const value = await response.json().catch(() => ({}));
  if (!response.ok) {
    if (response.status === 401 && !['/v1/auth/login', '/v1/me'].includes(path)) {
      window.dispatchEvent(new Event('futryva:unauthorized'));
    }
    throw new ApiError(response.status, value.error?.code || 'request_failed', value.error?.message || '');
  }
  return value as T;
}
