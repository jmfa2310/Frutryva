import { useEffect, useState, type FormEvent } from 'react';
import { api, ApiError, errorMessage } from './api';
import { contrastRatio } from './contrast';
import Account from './Account';
import Auth from './Auth';
import BrandingEditor, { ClubPreview } from './BrandingEditor';
import { Brand, Field, Icon, Modal, Notice, useAction } from './components';
import People from './People';
import { canManage, initials, roleName, type Club, type User } from './types';

type Page = 'clubs' | 'branding' | 'people' | 'account';

export default function App() {
  const [user, setUser] = useState<User | null>(null);
  const [booting, setBooting] = useState(true);
  const [bootError, setBootError] = useState('');
  const [clubs, setClubs] = useState<Club[]>([]);
  const [clubId, setClubId] = useState('');
  const [clubsLoading, setClubsLoading] = useState(false);
  const [clubsError, setClubsError] = useState('');
  const [page, setPage] = useState<Page>('clubs');
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<Club | null>(null);
  const [archiving, setArchiving] = useState<Club | null>(null);
  const [name, setName] = useState('');
  const [shortName, setShortName] = useState('');
  const [loadRevision, setLoadRevision] = useState(0);
  const action = useAction();

  useEffect(() => {
    let cancelled = false;
    api<{ user: User }>('/v1/me').then(result => { if (!cancelled) setUser(result.user); }).catch(error => {
      if (!cancelled && !(error instanceof ApiError && error.status === 401)) setBootError(errorMessage(error));
    }).finally(() => { if (!cancelled) setBooting(false); });
    const unauthorized = () => { setUser(null); setClubs([]); setClubId(''); };
    window.addEventListener('futryva:unauthorized', unauthorized);
    return () => { cancelled = true; window.removeEventListener('futryva:unauthorized', unauthorized); };
  }, []);

  useEffect(() => {
    let cancelled = false;
    if (!user) { setClubs([]); setClubId(''); return; }
    setClubsLoading(true); setClubsError('');
    api<{ clubs: Club[] }>('/v1/clubs').then(result => {
      if (cancelled) return;
      setClubs(result.clubs); setClubId(previous => result.clubs.some(club => club.id === previous) ? previous : result.clubs[0]?.id || '');
    }).catch(error => { if (!cancelled) setClubsError(errorMessage(error)); }).finally(() => { if (!cancelled) setClubsLoading(false); });
    return () => { cancelled = true; };
  }, [user?.id, loadRevision]);

  async function refreshUser() { const result = await api<{ user: User }>('/v1/me'); setUser(result.user); action.setNotice(null); }
  async function refreshClubs(preferred?: string) {
    const result = await api<{ clubs: Club[] }>('/v1/clubs');
    setClubs(result.clubs);
    setClubId(previous => preferred && result.clubs.some(club => club.id === preferred) ? preferred : result.clubs.some(club => club.id === previous) ? previous : result.clubs[0]?.id || '');
  }
  const activeClub = clubs.find(club => club.id === clubId);
  function openCreate() { setEditing(null); setCreating(true); setName(''); setShortName(''); action.setNotice(null); }
  function openEdit(club: Club) { setCreating(false); setEditing(club); setName(club.name); setShortName(club.shortName || ''); action.setNotice(null); }
  function saveClub(event: FormEvent) {
    event.preventDefault();
    void action.run(async () => {
      const result = await api<{ club: Club }>(editing ? `/v1/clubs/${editing.id}` : '/v1/clubs', editing ? 'PATCH' : 'POST', { name, shortName });
      await refreshClubs(result.club.id); setCreating(false); setEditing(null);
      action.setNotice({ type: 'success', text: editing ? 'Dados do clube atualizados.' : 'Clube criado. Já pode preparar a identidade visual e convidar a sua equipa.' });
    });
  }
  if (booting) return <main className="loading-state" role="status">A abrir o seu espaço FUTRYVA…</main>;
  if (bootError) return <main className="auth-panel"><section className="auth-card"><Brand /><h1>Não foi possível abrir o serviço.</h1><Notice value={{ type: 'error', text: bootError }} /><button className="button primary" onClick={() => window.location.reload()}>Tentar novamente</button></section></main>;
  if (!user) return <Auth onAuthenticated={(current, deliveryPending) => { setUser(current); setPage(current.emailVerified ? 'clubs' : 'account'); if (deliveryPending) action.setNotice({ type: 'warning', text: 'A conta foi criada, mas o email não pôde ser enviado. Use Enviar um novo código para tentar novamente.' }); }} />;
  const titles = { clubs: 'Os seus clubes', branding: 'Identidade do clube', people: 'Pessoas e convites', account: 'A sua conta' };
  const subtitles = { clubs: 'Cada clube tem a sua identidade. A sua conta liga-os num único espaço.', branding: 'Prepare, pré-visualize e publique a identidade que representa o seu clube.', people: 'Convide as pessoas certas e acompanhe o acesso ao clube.', account: 'Confirme o seu email, renove a sessão e aceite convites.' };
  return <div className="app-shell"><a className="skip-link" href="#main-content">Saltar para o conteúdo</a><aside className="sidebar"><Brand /><div className="sidebar-caption">ESPAÇO DE GESTÃO</div><nav aria-label="Navegação principal">{([['clubs', 'clubs', 'Os meus clubes'], ['branding', 'brand', 'Identidade visual'], ['people', 'people', 'Pessoas e convites'], ['account', 'account', 'A minha conta']] as const).map(([value, icon, label]) => <button key={value} className={`nav-button ${page === value ? 'active' : ''}`} aria-current={page === value ? 'page' : undefined} onClick={() => { setPage(value); action.setNotice(null); }}><Icon name={icon} />{label}</button>)}</nav><div className="sidebar-foot"><span className="user-avatar">{initials(user.displayName)}</span><div className="user-details"><strong>{user.displayName}</strong><span>{user.emailVerified ? 'Email confirmado' : 'Confirmação pendente'}</span></div><button className="icon-button" aria-label="Terminar sessão" disabled={action.busy} onClick={() => void action.run(async () => { await api('/v1/auth/logout', 'POST', {}); setUser(null); setClubs([]); setClubId(''); })}><Icon name="logout" /></button></div></aside><div className="workspace"><header className="topbar"><div className="breadcrumb">Manager <span>/</span> <strong>{titles[page]}</strong></div><div className="topbar-actions">{clubs.length > 0 && <select className="select" aria-label="Clube ativo" value={clubId} onChange={event => { setClubId(event.target.value); action.setNotice(null); }}><option value="" disabled>Escolher clube</option>{clubs.map(club => <option key={club.id} value={club.id}>{club.name}</option>)}</select>}<button className="button secondary small" onClick={() => setPage('account')}>Aceitar convite</button></div></header><main className="page" id="main-content"><div className="page-heading"><div><span className="eyebrow">FUTRYVA MANAGER</span><h1 className="page-title">{titles[page]}</h1><p className="page-description">{subtitles[page]}</p></div>{page === 'clubs' && <button className="button primary" disabled={!user.emailVerified} onClick={openCreate}><Icon name="plus" /> Criar clube</button>}</div>
    {!user.emailVerified && page !== 'account' && <div className="notice warning" role="status">Confirme o seu email para gerir clubes. <button className="link-button" onClick={() => setPage('account')}>Confirmar agora</button></div>}
    <Notice value={action.notice} />
    {page === 'account' ? <Account user={user} refreshUser={refreshUser} onAccepted={async id => { await refreshClubs(id); setPage('clubs'); }} /> : clubsLoading ? <div className="loading-state" role="status">A carregar os seus clubes…</div> : clubsError ? <div className="stack"><Notice value={{ type: 'error', text: clubsError }} /><button className="button secondary" onClick={() => setLoadRevision(value => value + 1)}>Tentar novamente</button></div> : page === 'clubs' ? <>
      <div className="metric-strip"><div className="metric"><span>Clubes na sua conta</span><strong>{clubs.length}</strong></div><div className="metric"><span>Clubes que pode gerir</span><strong>{clubs.filter(canManage).length}</strong></div><div className="metric"><span>Identidades publicadas</span><strong>{clubs.filter(club => club.brandingVersion > 0).length}</strong></div></div>
      {clubs.length === 0 ? <section className="card empty-state"><span className="empty-icon"><Icon name="clubs" /></span><h2>Um espaço para o seu clube.</h2><p className="muted">Crie o primeiro clube ou aceite um convite enviado para o seu email.</p><div className="inline-actions"><button className="button primary" disabled={!user.emailVerified} onClick={openCreate}>Criar o primeiro clube</button><button className="button secondary" onClick={() => setPage('account')}>Tenho um convite</button></div></section> : <div className="club-grid">{clubs.map(club => <article className="club-card" key={club.id}><div className="club-card-header"><span className="club-mark" style={{ backgroundColor: club.branding?.primaryColor || '#087F8C', color: contrastRatio('#FFFFFF', club.branding?.primaryColor || '#087F8C') > contrastRatio('#000000', club.branding?.primaryColor || '#087F8C') ? '#FFFFFF' : '#000000' }}>{club.branding?.logoUrl ? <img src={club.branding.logoUrl} alt="" referrerPolicy="no-referrer" /> : initials(club.shortName || club.name)}</span><span className="role-badge">{roleName(club.role)}</span></div><h2>{club.name}</h2><p className="club-meta">{club.shortName || 'Clube multidesportivo'}</p><div className="club-colors">{club.branding && [club.branding.primaryColor, club.branding.secondaryColor, club.branding.accentColor].map((color, index) => <span className="color-dot" key={index} style={{ backgroundColor: color }} title={color} />)}<span className="field-hint">{club.brandingVersion > 0 ? `Identidade publicada · v${club.brandingVersion}` : 'Identidade por publicar'}</span></div><div className="divider" /><div className="toolbar"><button className="button secondary small" onClick={() => { setClubId(club.id); setPage('branding'); }}>Abrir clube <Icon name="arrow" /></button>{canManage(club) && <button className="link-button" onClick={() => openEdit(club)}>Editar dados</button>}</div>{club.role === 'president' && <button className="link-button" onClick={() => { setArchiving(club); action.setNotice(null); }}>Arquivar clube</button>}</article>)}</div>}
      {activeClub?.branding && <section className="card"><div className="card-header"><h2 className="card-title">Identidade publicada · {activeClub.name}</h2><span className="badge">Versão {activeClub.brandingVersion}</span></div><div className="card-body"><ClubPreview club={activeClub} theme={activeClub.branding} label="Publicado" /></div></section>}
    </> : activeClub ? page === 'branding' ? <BrandingEditor key={activeClub.id} club={activeClub} onChanged={() => refreshClubs()} /> : <People key={activeClub.id} club={activeClub} /> : <section className="card empty-state"><h2>Escolha ou crie um clube.</h2><p className="muted">Esta área fica disponível quando pertencer a um clube.</p><button className="button primary" onClick={() => setPage('clubs')}>Ver os meus clubes</button></section>}
  </main></div>
    {(creating || editing) && <Modal labelledBy="club-form-title" onClose={() => { if (!action.busy) { setCreating(false); setEditing(null); } }}><h2 id="club-form-title">{editing ? 'Editar dados do clube' : 'Criar um clube'}</h2><p className="muted">{editing ? 'Atualize os dados de identificação do clube.' : 'Ficará responsável pelo clube como presidente. Pode convidar a direção e os treinadores a seguir.'}</p><Notice value={action.notice} /><form className="stack" onSubmit={saveClub}><Field label="Nome do clube"><input className="input" value={name} onChange={event => setName(event.target.value)} required minLength={2} maxLength={120} autoFocus /></Field><Field label="Nome curto (opcional)"><input className="input" value={shortName} onChange={event => setShortName(event.target.value)} maxLength={30} /></Field><div className="modal-actions"><button type="button" className="button secondary" disabled={action.busy} onClick={() => { setCreating(false); setEditing(null); }}>Cancelar</button><button className="button primary" disabled={action.busy}>{action.busy ? 'A guardar…' : editing ? 'Guardar alterações' : 'Criar clube'}</button></div></form></Modal>}
    {archiving && <Modal labelledBy="archive-title" onClose={() => { if (!action.busy) setArchiving(null); }}><h2 id="archive-title">Arquivar {archiving.name}?</h2><p>O clube deixará de estar disponível na aplicação. Os dados existentes são preservados.</p><Notice value={action.notice} /><div className="modal-actions"><button className="button secondary" disabled={action.busy} onClick={() => setArchiving(null)}>Cancelar</button><button className="button danger" disabled={action.busy} onClick={() => void action.run(async () => { await api(`/v1/clubs/${archiving.id}`, 'DELETE'); setArchiving(null); await refreshClubs(); action.setNotice({ type: 'success', text: 'Clube arquivado. Os dados foram preservados.' }); })}>Confirmar arquivo</button></div></Modal>}
  </div>;
}
