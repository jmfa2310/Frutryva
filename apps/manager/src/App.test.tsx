import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import App from './App';
import Auth from './Auth';
import { contrastRatio } from './contrast';
import { api, errorMessage, ApiError } from './api';
import { defaultTheme, type User } from './types';

const manager: User = { id: 'user-1', email: 'manager@example.invalid', displayName: 'Ana Silva', emailVerified: true };
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
  it('regista uma conta e leva a pessoa à confirmação de email, sem guardar tokens no browser', async () => {
    const user = userEvent.setup();
    const fetcher = server(path => {
      if (path === '/v1/me') return json({ error: { code: 'unauthorized' } }, 401);
      if (path === '/v1/auth/register') return json({ user: { ...manager, emailVerified: false } }, 201);
      if (path === '/v1/clubs') return json({ clubs: [] });
      throw new Error('unexpected request: ' + path);
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

  it('mostra confirmação genérica para recuperação e permite introduzir o código recebido', async () => {
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

describe('gestão e isolamento visual', () => {
  it('mantém o clube selecionado quando chega atrasado um rascunho de outro clube', async () => {
    const user = userEvent.setup();
    let resolveOldDraft!: (response: Response) => void;
    const oldDraft = new Promise<Response>(resolve => { resolveOldDraft = resolve; });
    const fetcher = server(path => {
      if (path === '/v1/me') return json({ user: manager });
      if (path === '/v1/clubs') return json({ clubs: [clubA, clubB] });
      if (path === '/v1/clubs/club-a/branding/draft') return oldDraft;
      if (path === '/v1/clubs/club-b/branding/draft') return json({ theme: { ...defaultTheme, primaryColor: '#227744' } });
      if (path.endsWith('/branding/versions')) return json({ versions: [] });
      throw new Error('unexpected request: ' + path);
    });
    render(<App />);
    await screen.findByRole('heading', { name: 'Clube Azul' });
    await user.click(screen.getByRole('button', { name: 'Identidade visual' }));
    await waitFor(() => expect(fetcher.mock.calls.some(([path]) => path === '/v1/clubs/club-a/branding/draft')).toBe(true));
    await user.selectOptions(screen.getByLabelText('Clube ativo'), 'club-b');
    expect(await screen.findByLabelText('Principal')).toHaveValue('#227744');
    await act(async () => { resolveOldDraft(json({ theme: { ...defaultTheme, primaryColor: '#CC1122' } })); await oldDraft; });
    expect(screen.getByLabelText('Principal')).toHaveValue('#227744');
    expect(screen.getByRole('heading', { name: 'Clube Verde' })).toBeVisible();
    expect(screen.queryByRole('heading', { name: 'Clube Azul' })).not.toBeInTheDocument();
  });

  it('não apresenta edição ou consulta dados de membros a um treinador', async () => {
    const user = userEvent.setup();
    const fetcher = server(path => {
      if (path === '/v1/me') return json({ user: manager });
      if (path === '/v1/clubs') return json({ clubs: [{ ...clubA, role: 'coach', branding: defaultTheme, brandingVersion: 1 }] });
      throw new Error('private endpoint must not be requested: ' + path);
    });
    render(<App />);
    await screen.findByLabelText('Clube ativo');
    await user.click(screen.getByRole('button', { name: 'Identidade visual' }));
    expect(await screen.findByText('Só de leitura')).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Guardar e publicar' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Pessoas e convites' }));
    expect(await screen.findByText(/gestão de membros e convites está reservada/)).toBeVisible();
    expect(fetcher.mock.calls.every(([path]) => path === '/v1/me' || path === '/v1/clubs')).toBe(true);
  });

  it('oferece um formulário de criação com foco contido e fecha com Escape', async () => {
    const user = userEvent.setup();
    server(path => path === '/v1/me' ? json({ user: manager }) : json({ clubs: [] }));
    render(<App />);
    await user.click(await screen.findByRole('button', { name: 'Criar clube' }));
    const dialog = screen.getByRole('dialog', { name: 'Criar um clube' });
    expect(within(dialog).getByLabelText('Nome do clube')).toHaveFocus();
    const lastButton = within(dialog).getByRole('button', { name: 'Criar clube' });
    lastButton.focus();
    await user.tab();
    expect(within(dialog).getByLabelText('Nome do clube')).toHaveFocus();
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Criar clube' })).toHaveFocus();
  });
});

describe('contratos e acessibilidade', () => {
  it('trata 204 sem tentar ler JSON e traduz falhas de email e convites', async () => {
    server(() => new Response(null, { status: 204 }));
    expect(await api('/v1/auth/logout', 'POST', {})).toBeUndefined();
    expect(errorMessage(new ApiError(503, 'email_unavailable', 'internal'))).toContain('enviar o email');
    expect(errorMessage(new ApiError(400, 'invalid_invitation', 'internal'))).toContain('convite é inválido');
  });

  it('calcula o contraste WCAG e distingue combinações legíveis', () => {
    expect(contrastRatio('#000000', '#FFFFFF')).toBeCloseTo(21);
    expect(contrastRatio('#FFFFFF', '#FFFFFF')).toBeCloseTo(1);
    expect(contrastRatio(defaultTheme.textColor, defaultTheme.surfaceColor)).toBeGreaterThanOrEqual(4.5);
  });
});
