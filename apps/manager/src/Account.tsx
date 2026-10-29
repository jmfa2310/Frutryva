import { useState, type FormEvent } from 'react';
import { api } from './api';
import { Field, Notice, useAction } from './components';
import type { User } from './types';

export default function Account({ user, refreshUser, onAccepted }: { user: User; refreshUser: () => Promise<void>; onAccepted: (clubId: string) => Promise<void> }) {
  const [verificationToken, setVerificationToken] = useState('');
  const [invitationToken, setInvitationToken] = useState('');
  const verification = useAction(), acceptance = useAction(), session = useAction();
  function verify(event: FormEvent) {
    event.preventDefault();
    void verification.run(async () => {
      await api('/v1/auth/verify-email', 'POST', { token: verificationToken.trim() });
      setVerificationToken(''); await refreshUser();
      verification.setNotice({ type: 'success', text: 'Email confirmado. Já pode criar um clube ou aceitar convites.' });
    });
  }
  function accept(event: FormEvent) {
    event.preventDefault();
    void acceptance.run(async () => {
      const result = await api<{ club: { id: string } }>('/v1/invitations/accept', 'POST', { token: invitationToken.trim() });
      setInvitationToken(''); await onAccepted(result.club.id);
      acceptance.setNotice({ type: 'success', text: 'Convite aceite. O clube já está disponível na sua conta.' });
    });
  }
  return <div className="grid two-columns">
    <section className="card"><div className="card-header"><h2 className="card-title">A sua conta</h2><span className={`badge ${user.emailVerified ? 'success' : ''}`}>{user.emailVerified ? 'Email confirmado' : 'Confirmação pendente'}</span></div><div className="card-body stack">
      <div><span className="field-label">Nome</span><p>{user.displayName}</p><span className="field-label">Email</span><p>{user.email}</p></div>
      {!user.emailVerified && <><Notice value={{ type: 'info', text: 'Consulte o email de confirmação e introduza o código recebido. É necessário confirmar o email para gerir clubes.' }} /><Notice value={verification.notice} />
        <form className="stack" onSubmit={verify}><Field label="Código de confirmação"><input className="input" autoComplete="off" value={verificationToken} onChange={e => setVerificationToken(e.target.value)} required /></Field><button className="button primary" disabled={verification.busy}>{verification.busy ? 'A confirmar…' : 'Confirmar email'}</button></form>
        <button className="link-button" disabled={verification.busy} onClick={() => void verification.run(async () => { await api('/v1/auth/resend-verification', 'POST', {}); verification.setNotice({ type: 'success', text: 'Foi enviado um novo código. Consulte o seu email.' }); })}>Enviar um novo código</button>
      </>}
      <Notice value={session.notice} /><button className="button secondary" disabled={session.busy} onClick={() => void session.run(async () => { await api('/v1/auth/refresh', 'POST', {}); await refreshUser(); session.setNotice({ type: 'success', text: 'Sessão renovada em segurança.' }); })}>Renovar sessão</button>
    </div></section>
    <section className="card"><div className="card-header"><h2 className="card-title">Recebeu um convite?</h2></div><div className="card-body stack"><p className="muted">Introduza o código recebido por email. O convite deve ter sido enviado para <strong>{user.email}</strong>.</p><Notice value={acceptance.notice} /><form className="stack" onSubmit={accept}><Field label="Código do convite"><input className="input" autoComplete="off" value={invitationToken} onChange={e => setInvitationToken(e.target.value)} required disabled={!user.emailVerified} /></Field><button className="button primary" disabled={acceptance.busy || !user.emailVerified}>{acceptance.busy ? 'A aceitar…' : 'Aceitar convite'}</button></form>{!user.emailVerified && <p className="field-hint">Confirme o email da sua conta para aceitar um convite.</p>}</div></section>
  </div>;
}
