import { useEffect, useState, type FormEvent } from 'react';
import { api, ApiError, errorMessage } from './api';
import { Field, Modal, Notice, type NoticeValue } from './components';
import type { PlatformClub } from './types';

const ownerMessages: Record<string, string> = {
  user_has_club: 'Esta conta já tem um clube atribuído. Cada pessoa só pode ter um clube ativo.',
  club_has_manager: 'Este clube já tem um responsável atribuído.',
  manager_not_found: 'Não existe uma conta com esse email. Peça ao responsável que se registe primeiro.',
  manager_not_verified: 'O responsável precisa de confirmar o email da sua conta antes da atribuição.',
  platform_owner_required: 'Esta operação está reservada ao dono da plataforma.',
  invalid_request: 'Reveja o nome, nome curto e email do responsável.',
};

function ownerError(error: unknown) {
  return error instanceof ApiError && ownerMessages[error.code] ? ownerMessages[error.code] : errorMessage(error);
}

export default function PlatformClubs({ onChanged }: { onChanged: () => Promise<void> }) {
  const [clubs, setClubs] = useState<PlatformClub[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<NoticeValue>(null);
  const [name, setName] = useState('');
  const [shortName, setShortName] = useState('');
  const [managerEmail, setManagerEmail] = useState('');
  const [assignmentEmails, setAssignmentEmails] = useState<Record<string, string>>({});
  const [archiving, setArchiving] = useState<PlatformClub | null>(null);

  async function reload() {
    const result = await api<{ clubs: PlatformClub[] }>('/v1/admin/clubs');
    setClubs(result.clubs);
  }
  useEffect(() => {
    let cancelled = false;
    api<{ clubs: PlatformClub[] }>('/v1/admin/clubs')
      .then(result => { if (!cancelled) setClubs(result.clubs); })
      .catch(error => { if (!cancelled) setLoadError(ownerError(error)); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, []);

  async function perform(task: () => Promise<void>) {
    setBusy(true); setNotice(null);
    try { await task(); }
    catch (error) { setNotice({ type: 'error', text: ownerError(error) }); }
    finally { setBusy(false); }
  }
  function createClub(event: FormEvent) {
    event.preventDefault();
    void perform(async () => {
      await api('/v1/admin/clubs', 'POST', { name, shortName, managerEmail });
      setName(''); setShortName(''); setManagerEmail('');
      await reload(); await onChanged();
      setNotice({ type: 'success', text: 'Clube criado e atribuído ao responsável indicado.' });
    });
  }
  function assignClub(event: FormEvent, club: PlatformClub) {
    event.preventDefault();
    void perform(async () => {
      await api(`/v1/admin/clubs/${club.id}/assign`, 'POST', { email: assignmentEmails[club.id] || '' });
      setAssignmentEmails(previous => ({ ...previous, [club.id]: '' }));
      await reload(); await onChanged();
      setNotice({ type: 'success', text: 'Clube atribuído à conta indicada.' });
    });
  }

  if (loading) return <div className="loading-state" role="status">A carregar os clubes da plataforma…</div>;
  if (loadError) return <div className="stack"><Notice value={{ type: 'error', text: loadError }} /><button className="button secondary" onClick={() => { setLoading(true); setLoadError(''); void reload().catch(error => setLoadError(ownerError(error))).finally(() => setLoading(false)); }}>Tentar novamente</button></div>;

  return <div className="stack">
    <Notice value={notice} />
    <section className="card"><div className="card-header"><h2 className="card-title">Criar e atribuir um clube</h2><span className="badge">Dono da plataforma</span></div><div className="card-body stack">
      <p className="muted">O responsável deve ter uma conta registada e o email confirmado. Cada pessoa só pode ter um clube ativo. A atribuição não dá acesso aos dados privados de outros clubes.</p>
      <form className="stack" aria-label="Criar e atribuir clube" onSubmit={createClub}>
        <div className="form-grid"><Field label="Nome do novo clube"><input className="input" value={name} onChange={event => setName(event.target.value)} required minLength={2} maxLength={120} /></Field><Field label="Nome curto do novo clube (opcional)"><input className="input" value={shortName} onChange={event => setShortName(event.target.value)} maxLength={30} /></Field></div>
        <Field label="Email do responsável" hint="A conta indicada passa a gerir este clube como presidente."><input className="input" type="email" autoComplete="off" value={managerEmail} onChange={event => setManagerEmail(event.target.value)} required maxLength={254} /></Field>
        <div><button className="button primary" disabled={busy}>{busy ? 'A guardar…' : 'Criar e atribuir clube'}</button></div>
      </form>
    </div></section>
    <section className="stack platform-club-list" aria-label="Clubes da plataforma"><div className="section-heading"><h2>Clubes da plataforma</h2><span className="badge">{clubs.length}</span></div>
      {clubs.length === 0 ? <section className="card empty-state"><h3>Ainda não existem clubes.</h3><p className="muted">Crie o primeiro clube e indique a conta do responsável.</p></section> : clubs.map(club => <article className="card" key={club.id}><div className="card-header"><div><h3 className="card-title">{club.name}</h3>{club.shortName && <p>{club.shortName}</p>}</div><button className="button danger small" aria-label={`Arquivar clube ${club.name}`} disabled={busy} onClick={() => setArchiving(club)}>Arquivar</button></div><div className="card-body stack">
        {club.manager ? <div><span className="field-label">Responsável atribuído</span><p><strong>{club.manager.displayName}</strong><br /><span className="muted">{club.manager.email}</span></p></div> : <><Notice value={{ type: 'info', text: 'Este clube ainda precisa de um responsável. Os dados existentes são preservados.' }} /><form className="stack" aria-label={`Atribuir ${club.name}`} onSubmit={event => assignClub(event, club)}><Field label={`Email do responsável de ${club.name}`}><input className="input" type="email" autoComplete="off" value={assignmentEmails[club.id] || ''} onChange={event => setAssignmentEmails(previous => ({ ...previous, [club.id]: event.target.value }))} required maxLength={254} /></Field><div><button className="button primary" disabled={busy}>Atribuir clube</button></div></form></>}
      </div></article>)}
    </section>
    {archiving && <Modal labelledBy="platform-archive-title" onClose={() => { if (!busy) setArchiving(null); }}><h2 id="platform-archive-title">Arquivar {archiving.name}?</h2><p>O clube deixará de estar disponível na aplicação. Os dados e o histórico serão preservados.</p><Notice value={notice} /><div className="modal-actions"><button className="button secondary" disabled={busy} onClick={() => setArchiving(null)}>Cancelar</button><button className="button danger" disabled={busy} onClick={() => void perform(async () => { await api(`/v1/admin/clubs/${archiving.id}`, 'DELETE'); setArchiving(null); await reload(); await onChanged(); setNotice({ type: 'success', text: 'Clube arquivado. Os dados foram preservados.' }); })}>Confirmar arquivo</button></div></Modal>}
  </div>;
}
