import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import App from './App';
import Auth from './Auth';
import { contrastRatio } from './contrast';
import { api, errorMessage, ApiError } from './api';
import { defaultTheme, type User } from './types';

const manager: User = { id: 'user-1', email: 'manager@example.invalid', displayName: 'Ana Silva', emailVerified: true, assignedClubId: 'club-a', isPlatformOwner: false };
const clubA = { id: 'club-a', name: 'Clube Azul', shortName: 'CA', role: 'president', branding: null, brandingVersion: 0, archived: false };
const clubB = { ...clubA, id: 'club-b', name: 'Clube Verde', shortName: 'CV' };
const json = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } });
function server(handler: (path: string, init: RequestInit) => Response | Promise<Response>) {
  const fetcher = vi.fn((path: string, init: RequestInit = {}) => handler(path, init));
  vi.stubGlobal('fetch', fetcher);
  return fetcher;
}
beforeEach(() => { vi.unstubAllGlobals(); });

describe('acesso à conta', () => {
  it('regista e pede confirmação de email sem guardar tokens no browser', async () => {
    const user = userEvent.setup();
    const fetcher = server(path => {
      if (path === '/v1/me') return json({ error: { code: 'unauthorized' } }, 401);
      if (path === '/v1/auth/register') return json({ user: { ...manager, emailVerified: false, assignedClubId: undefined } }, 201);
      if (path === '/v1/clubs') return json({ clubs: [] });
      throw new Error('unexpected request');
    });
    const storageSpy = vi.spyOn(Storage.prototype, 'setItem');
    render(<App />);
    await user.click(await screen.findByRole('button', { name: 'Criar conta' }));
    await user.type(screen.getByLabelText('Nome'), 'Ana Silva');
    await user.type(screen.getByLabelText('Email'), manager.email);
    await user.type(screen.getByLabelText('Palavra-passe'), 'Uma frase sintética longa');
    await user.click(within(screen.getByRole('form', { name: 'Criar conta' })).getByRole('button', { name: 'Criar conta' }));
    expect(await screen.findByRole('button', { name: 'Confirmar email' })).toBeVisible();
    const call = fetcher.mock.calls.find(([path]) => path === '/v1/auth/register');
    expect(call?.[1]?.credentials).toBe('same-origin');
    expect(JSON.parse(String(call?.[1]?.body))).toMatchObject({ displayName: 'Ana Silva', email: manager.email });
    expect(storageSpy).not.toHaveBeenCalled();
  });

  it('recupera acesso por código com resposta genérica', async () => {
    const user = userEvent.setup();
    server(path => {
      if (path === '/v1/auth/forgot-password') return json({}, 202);
      if (path === '/v1/auth/reset-password') return new Response(null, { status: 204 });
      throw new Error('unexpected request');
    });
    render(<Auth onAuthenticated={vi.fn()} />);
    await user.click(screen.getByRole('button', { name: 'Esqueci-me da palavra-passe' }));
    await user.type(screen.getByLabelText('Email'), 'unknown@example.invalid');
    await user.click(screen.getByRole('button', { name: 'Enviar código de recuperação' }));
    expect(await screen.findByText(/Se existir uma conta com este email/)).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Já tenho um código de recuperação' }));
    await user.type(screen.getByLabelText('Código de recuperação'), 'synthetic-one-use-code');
    await user.type(screen.getByLabelText('Palavra-passe'), 'Uma nova frase sintética');
    await user.click(screen.getByRole('button', { name: 'Guardar palavra-passe' }));
    expect(await screen.findByText(/Palavra-passe atualizada/)).toBeVisible();
    expect(screen.getByRole('button', { name: 'Entrar no Manager' })).toBeVisible();
  });
});

describe('clube único atribuído pela plataforma', () => {
  it('mostra espera pela atribuição sem criar nem selecionar clubes', async () => {
    const user = userEvent.setup();
    const fetcher = server(path => path === '/v1/me' ? json({ user: { ...manager, assignedClubId: undefined } }) : json({ clubs: [] }));
    render(<App />);
    expect(await screen.findByRole('heading', { name: 'O seu clube ainda não foi atribuído.' })).toBeVisible();
    expect(screen.getByText(/Contacte o responsável pela plataforma/)).toBeVisible();
    expect(screen.queryByRole('button', { name: /Criar clube|Criar o primeiro clube|Administração/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('combobox', { name: 'Clube ativo' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Verificar atribuição' }));
    await waitFor(() => expect(fetcher.mock.calls.filter(([path]) => path === '/v1/clubs').length).toBeGreaterThan(1));
    expect(fetcher.mock.calls.every(([, init]) => !init?.method || init?.method === 'GET')).toBe(true);
  });

  it('apresenta só o clube atribuído sem criação ou arquivo pelo presidente', async () => {
    server(path => path === '/v1/me' ? json({ user: { ...manager, assignedClubId: clubB.id } }) : json({ clubs: [clubA, clubB] }));
    render(<App />);
    expect(await screen.findByRole('heading', { name: 'Clube Verde', level: 2 })).toBeVisible();
    expect(screen.queryByRole('heading', { name: 'Clube Azul' })).not.toBeInTheDocument();
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Arquivar|Criar clube|Administração/ })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Editar dados' })).toBeVisible();
  });

  it('obtém a atribuição nova sem recarregar e preserva-a com respostas fora de ordem', async () => {
    const user = userEvent.setup();
    let profiles = 0, clubRequests = 0;
    let resolveManual!: (response: Response) => void;
    const delayedManual = new Promise<Response>(resolve => { resolveManual = resolve; });
    server(path => {
      if (path === '/v1/me') return json({ user: { ...manager, assignedClubId: ++profiles === 1 ? undefined : clubA.id } });
      if (path === '/v1/clubs') {
        clubRequests++;
        if (clubRequests === 1) return json({ clubs: [] });
        if (clubRequests === 2) return delayedManual;
        return json({ clubs: [clubA] });
      }
      throw new Error('unexpected request');
    });
    render(<App />);
    await screen.findByRole('heading', { name: 'O seu clube ainda não foi atribuído.' });
    await user.click(screen.getByRole('button', { name: 'Verificar atribuição' }));
    expect(await screen.findByRole('heading', { name: clubA.name, level: 2 })).toBeVisible();
    await act(async () => { resolveManual(json({ clubs: [clubA] })); await delayedManual; });
    expect(screen.getByRole('heading', { name: clubA.name, level: 2 })).toBeVisible();
    expect(screen.queryByRole('heading', { name: 'O seu clube ainda não foi atribuído.' })).not.toBeInTheDocument();
  });

  it('mantém cores publicadas no menu durante o rascunho e muda só ao publicar', async () => {
    const user = userEvent.setup();
    let published = { ...defaultTheme, primaryColor: '#AA2211', secondaryColor: '#FFD700' };
    let draft = { ...defaultTheme, primaryColor: '#227744' };
    let version = 1;
    server((path, init) => {
      if (path === '/v1/me') return json({ user: manager });
      if (path === '/v1/clubs') return json({ clubs: [{ ...clubA, branding: published, brandingVersion: version }] });
      if (path.endsWith('/branding/draft') && init?.method === 'PUT') { draft = JSON.parse(String(init.body)).theme; return json({ theme: draft }); }
      if (path.endsWith('/branding/draft')) return json({ theme: draft });
      if (path.endsWith('/branding/versions')) return json({ versions: [] });
      if (path.endsWith('/branding/publish')) { published = draft; version++; return json({ version }); }
      throw new Error('unexpected request: ' + path);
    });
    const { container } = render(<App />);
    await screen.findByRole('heading', { name: 'Clube Azul', level: 2 });
    const shell = container.querySelector<HTMLElement>('.app-shell')!;
    expect(shell.style.getPropertyValue('--platform')).toBe('#AA2211');
    expect(shell.style.getPropertyValue('--sidebar')).toBe('#FFD700');
    expect(screen.getByText('FUTRYVA')).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Identidade visual' }));
    const primary = await screen.findByLabelText('Principal', { exact: true });
    expect(primary).toHaveValue('#227744');
    fireEvent.change(primary, { target: { value: '#4433CC' } });
    await user.click(screen.getByRole('button', { name: 'Guardar rascunho' }));
    expect(await screen.findByText(/Rascunho guardado/)).toBeVisible();
    expect(shell.style.getPropertyValue('--platform')).toBe('#AA2211');
    await user.click(screen.getByRole('button', { name: 'Guardar e publicar' }));
    await waitFor(() => expect(shell.style.getPropertyValue('--platform')).toBe('#4433CC'));
  });

  it('não permite editar branding nem consultar membros a um treinador', async () => {
    const user = userEvent.setup();
    const fetcher = server(path => {
      if (path === '/v1/me') return json({ user: manager });
      if (path === '/v1/clubs') return json({ clubs: [{ ...clubA, role: 'coach', branding: defaultTheme, brandingVersion: 1 }] });
      throw new Error('private endpoint must not be requested');
    });
    render(<App />);
    await screen.findByRole('heading', { name: 'Clube Azul', level: 2 });
    await user.click(screen.getByRole('button', { name: 'Identidade visual' }));
    expect(await screen.findByText('Só de leitura')).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Guardar e publicar' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Pessoas e convites' }));
    expect(await screen.findByText(/gestão de membros e convites está reservada/)).toBeVisible();
    expect(fetcher.mock.calls.every(([path]) => path === '/v1/me' || path === '/v1/clubs')).toBe(true);
  });

  it('edita o clube atribuído com foco contido e fecho por Escape', async () => {
    const user = userEvent.setup();
    server(path => path === '/v1/me' ? json({ user: manager }) : json({ clubs: [clubA] }));
    render(<App />);
    await user.click(await screen.findByRole('button', { name: 'Editar dados' }));
    const dialog = screen.getByRole('dialog', { name: 'Editar dados do clube' });
    expect(within(dialog).getByLabelText('Nome do clube')).toHaveFocus();
    within(dialog).getByRole('button', { name: 'Guardar alterações' }).focus();
    await user.tab();
    expect(within(dialog).getByLabelText('Nome do clube')).toHaveFocus();
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Editar dados' })).toHaveFocus();
  });
});

describe('dono da plataforma', () => {
  it('cria e atribui pelo painel reservado sem criar clube pessoal', async () => {
    const user = userEvent.setup();
    const assigned = { id: clubA.id, name: clubA.name, shortName: clubA.shortName, manager: { id: manager.id, email: manager.email, displayName: manager.displayName } };
    let clubs: typeof assigned[] = [];
    const fetcher = server((path, init) => {
      if (path === '/v1/me') return json({ user: { ...manager, assignedClubId: undefined, isPlatformOwner: true } });
      if (path === '/v1/clubs') return json({ clubs: [] });
      if (path === '/v1/admin/clubs' && init?.method === 'POST') { clubs = [assigned]; return json({ club: assigned }, 201); }
      if (path === '/v1/admin/clubs') return json({ clubs });
      throw new Error('unexpected request');
    });
    render(<App />);
    await user.click(await screen.findByRole('button', { name: 'Administração' }));
    const form = await screen.findByRole('form', { name: 'Criar e atribuir clube' });
    await user.type(within(form).getByLabelText('Nome do novo clube'), clubA.name);
    await user.type(within(form).getByLabelText('Email do responsável'), manager.email);
    await user.click(within(form).getByRole('button', { name: 'Criar e atribuir clube' }));
    expect(await screen.findByText('Clube criado e atribuído ao responsável indicado.')).toBeVisible();
    expect(screen.getByRole('heading', { name: clubA.name })).toBeVisible();
    const create = fetcher.mock.calls.find(([path, init]) => path === '/v1/admin/clubs' && init?.method === 'POST');
    expect(JSON.parse(String(create?.[1]?.body))).toMatchObject({ name: clubA.name, managerEmail: manager.email });
    expect(fetcher.mock.calls.some(([path, init]) => path === '/v1/clubs' && init?.method === 'POST')).toBe(false);
  });

  it('explica a recusa de atribuir outro clube a uma conta já atribuída', async () => {
    const user = userEvent.setup();
    server((path, init) => {
      if (path === '/v1/me') return json({ user: { ...manager, isPlatformOwner: true } });
      if (path === '/v1/clubs') return json({ clubs: [clubA] });
      if (path === '/v1/admin/clubs' && init?.method === 'POST') return json({ error: { code: 'user_has_club' } }, 409);
      if (path === '/v1/admin/clubs') return json({ clubs: [] });
      throw new Error('unexpected request');
    });
    render(<App />);
    await user.click(await screen.findByRole('button', { name: 'Administração' }));
    const form = await screen.findByRole('form', { name: 'Criar e atribuir clube' });
    await user.type(within(form).getByLabelText('Nome do novo clube'), 'Outro clube');
    await user.type(within(form).getByLabelText('Email do responsável'), manager.email);
    await user.click(within(form).getByRole('button', { name: 'Criar e atribuir clube' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Esta conta já tem um clube atribuído.');
    expect(screen.getByLabelText('Nome do novo clube')).toHaveValue('Outro clube');
  });
});

describe('contratos e acessibilidade', () => {
  it('trata 204 e traduz falhas de email e convites', async () => {
    server(() => new Response(null, { status: 204 }));
    expect(await api('/v1/auth/logout', 'POST', {})).toBeUndefined();
    expect(errorMessage(new ApiError(503, 'email_unavailable', 'internal'))).toContain('enviar o email');
    expect(errorMessage(new ApiError(400, 'invalid_invitation', 'internal'))).toContain('convite é inválido');
  });
  it('calcula contraste WCAG', () => {
    expect(contrastRatio('#000000', '#FFFFFF')).toBeCloseTo(21);
    expect(contrastRatio('#FFFFFF', '#FFFFFF')).toBeCloseTo(1);
    expect(contrastRatio(defaultTheme.textColor, defaultTheme.surfaceColor)).toBeGreaterThanOrEqual(4.5);
  });
});
