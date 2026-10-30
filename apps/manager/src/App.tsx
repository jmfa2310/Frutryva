import { useEffect, useState, type FormEvent } from 'react';
import { api, ApiError, errorMessage } from './api';
import Account from './Account';
import Auth from './Auth';
import BrandingEditor, { ClubPreview } from './BrandingEditor';
import { accessibleColor, clubShellStyle } from './clubShell';
import { Brand, Field, Icon, Modal, Notice, useAction } from './components';
import People from './People';
import PlatformClubs from './PlatformClubs';
import { canManage, initials, roleName, type Club, type User } from './types';

type Page = 'club' | 'branding' | 'people' | 'account' | 'platform';

export default function App() {
  const [user, setUser] = useState<User | null>(null);
  const [booting, setBooting] = useState(true);
  const [bootError, setBootError] = useState('');
  const [club, setClub] = useState<Club | null>(null);
  const [clubLoading, setClubLoading] = useState(false);
  const [clubError, setClubError] = useState('');
  const [page, setPage] = useState<Page>('club');
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState('');
  const [shortName, setShortName] = useState('');
  const [loadRevision, setLoadRevision] = useState(0);
  const action = useAction();

  useEffect(() => {
    let cancelled = false;
    api<{ user: User }>('/v1/me').then(result => {
      if (!cancelled) setUser(result.user);
    }).catch(error => {
      if (!cancelled && !(error instanceof ApiError && error.status === 401)) setBootError(errorMessage(error));
    }).finally(() => { if (!cancelled) setBooting(false); });
    const unauthorized = () => { setUser(null); setClub(null); setEditing(false); };
    window.addEventListener('futryva:unauthorized', unauthorized);
    return () => { cancelled = true; window.removeEventListener('futryva:unauthorized', unauthorized); };
  }, []);

  useEffect(() => {
    let cancelled = false;
    if (!user) { setClub(null); return; }
    setClubLoading(true); setClubError('');
    api<{ clubs: Club[] }>('/v1/clubs').then(result => {
      if (!cancelled) setClub(result.clubs.find(item => item.id === user.assignedClubId) || null);
    }).catch(error => { if (!cancelled) setClubError(errorMessage(error)); })
      .finally(() => { if (!cancelled) setClubLoading(false); });
    return () => { cancelled = true; };
  }, [user?.id, user?.assignedClubId, loadRevision]);

  async function refreshUser() {
    const result = await api<{ user: User }>('/v1/me');
    setUser(result.user); action.setNotice(null);
  }
  async function refreshClub() {
    const result = await api<{ clubs: Club[] }>('/v1/clubs');
    setClub(result.clubs.find(item => item.id === user?.assignedClubId) || null);
  }
  async function refreshAssignment() {
    // Use the fresh assignment in both responses; the current render may still
    // hold the previous user while React commits the profile update.
    const profile = await api<{ user: User }>('/v1/me');
    setUser(profile.user); action.setNotice(null);
    const result = await api<{ clubs: Club[] }>('/v1/clubs');
    setClub(result.clubs.find(item => item.id === profile.user.assignedClubId) || null);
  }
  function navigate(next: Page) { setPage(next); action.setNotice(null); }
  function openEdit() {
    if (!club) return;
    setName(club.name); setShortName(club.shortName || ''); setEditing(true); action.setNotice(null);
  }
  function saveClub(event: FormEvent) {
    event.preventDefault();
    if (!club) return;
    void action.run(async () => {
      await api(`/v1/clubs/${club.id}`, 'PATCH', { name, shortName });
      await refreshClub(); setEditing(false);
      action.setNotice({ type: 'success', text: 'Dados do clube atualizados.' });
    });
  }

  if (booting) return <main className="loading-state" role="status">A abrir o seu espaço FUTRYVA…</main>;
  if (bootError) return <main className="auth-panel"><section className="auth-card"><Brand /><h1>Não foi possível abrir o serviço.</h1><Notice value={{ type: 'error', text: bootError }} /><button className="button primary" onClick={() => window.location.reload()}>Tentar novamente</button></section></main>;
  if (!user) return <Auth onAuthenticated={(current, deliveryPending) => {
    setUser(current); setPage(current.emailVerified ? 'club' : 'account');
    if (deliveryPending) action.setNotice({ type: 'warning', text: 'A conta foi criada, mas o email não pôde ser enviado. Use Enviar um novo código para tentar novamente.' });
  }} />;

  const titles = { club: 'O meu clube', branding: 'Identidade do clube', people: 'Pessoas e convites', account: 'A sua conta', platform: 'Administração da plataforma' };
  const subtitles = {
    club: 'O clube atribuído à sua conta pelo responsável pela plataforma.',
    branding: 'Prepare, pré-visualize e publique a identidade que representa o seu clube.',
    people: 'Convide as pessoas certas e acompanhe o acesso ao clube.',
    account: 'Confirme o seu email, renove a sessão e consulte os seus convites.',
    platform: 'Crie os clubes e atribua cada um à conta do respetivo responsável.',
  };
  const navigation: { page: Page; icon: 'clubs' | 'brand' | 'people' | 'account'; label: string }[] = [
    { page: 'club', icon: 'clubs', label: 'O meu clube' },
    { page: 'branding', icon: 'brand', label: 'Identidade visual' },
    { page: 'people', icon: 'people', label: 'Pessoas e convites' },
    { page: 'account', icon: 'account', label: 'A minha conta' },
  ];
  if (user.isPlatformOwner) navigation.push({ page: 'platform', icon: 'clubs', label: 'Administração' });

  const noClub = <section className="card empty-state">
    <span className="empty-icon"><Icon name="clubs" /></span>
    <h2>O seu clube ainda não foi atribuído.</h2>
    <p className="muted">Contacte o responsável pela plataforma. Cada conta de gestão está associada a um único clube.</p>
    {!user.emailVerified && <p className="field-hint">Confirme primeiro o seu email para poder receber a atribuição.</p>}
    <div className="inline-actions">
      <button className="button secondary" disabled={action.busy} onClick={() => void action.run(refreshAssignment)}>Verificar atribuição</button>
      {!user.emailVerified && <button className="button primary" onClick={() => navigate('account')}>Confirmar email</button>}
      {user.isPlatformOwner && <button className="button primary" onClick={() => navigate('platform')}>Gerir clubes da plataforma</button>}
    </div>
  </section>;

  return <div className="app-shell" style={clubShellStyle(club?.branding)}>
    <a className="skip-link" href="#main-content">Saltar para o conteúdo</a>
    <aside className="sidebar">
      <Brand />
      {club && <div className="sidebar-club">
        <span className="sidebar-club-logo">{club.branding?.logoUrl ? <img src={club.branding.logoUrl} alt="" referrerPolicy="no-referrer" /> : initials(club.shortName || club.name)}</span>
        <div className="sidebar-club-name"><strong>{club.name}</strong><span>{roleName(club.role)}</span></div>
      </div>}
      <div className="sidebar-caption">{user.isPlatformOwner ? 'GESTÃO E ADMINISTRAÇÃO' : 'ESPAÇO DE GESTÃO'}</div>
      <nav aria-label="Navegação principal">{navigation.map(item => <button key={item.page} className={`nav-button ${page === item.page ? 'active' : ''}`} aria-current={page === item.page ? 'page' : undefined} onClick={() => navigate(item.page)}><Icon name={item.icon} />{item.label}</button>)}</nav>
      <div className="sidebar-foot"><span className="user-avatar">{initials(user.displayName)}</span><div className="user-details"><strong>{user.displayName}</strong><span>{user.isPlatformOwner ? 'Dono da plataforma' : user.emailVerified ? 'Email confirmado' : 'Confirmação pendente'}</span></div><button className="icon-button" aria-label="Terminar sessão" disabled={action.busy} onClick={() => void action.run(async () => { await api('/v1/auth/logout', 'POST', {}); setUser(null); setClub(null); setEditing(false); })}><Icon name="logout" /></button></div>
    </aside>
    <div className="workspace">
      <header className="topbar"><div className="breadcrumb">Manager <span>/</span> <strong>{titles[page]}</strong></div><div className="topbar-actions">{club && <span className="assigned-club-name">{club.shortName || club.name}</span>}<button className="button secondary small" onClick={() => navigate('account')}>Conta e convites</button></div></header>
      <main className="page" id="main-content"><div className="page-heading"><div><span className="eyebrow">FUTRYVA MANAGER</span><h1 className="page-title">{titles[page]}</h1><p className="page-description">{subtitles[page]}</p></div></div>
        {!user.emailVerified && page !== 'account' && <div className="notice warning" role="status">Confirme o seu email para gerir o clube. <button className="link-button" onClick={() => navigate('account')}>Confirmar agora</button></div>}
        <Notice value={action.notice} />
        {page === 'account' ? <Account user={user} refreshUser={refreshUser} onAccepted={async () => { await refreshAssignment(); setPage('club'); }} />
          : page === 'platform' && user.isPlatformOwner ? <PlatformClubs onChanged={refreshAssignment} />
            : clubLoading ? <div className="loading-state" role="status">A carregar o seu clube…</div>
              : clubError ? <div className="stack"><Notice value={{ type: 'error', text: clubError }} /><button className="button secondary" onClick={() => setLoadRevision(value => value + 1)}>Tentar novamente</button></div>
                : !club ? noClub
                  : page === 'club' ? <div className="stack">
                    <article className="club-card assigned-club-summary"><div className="club-card-header"><span className="club-mark" style={{ backgroundColor: club.branding?.primaryColor || '#087F8C', color: accessibleColor(club.branding?.primaryColor || '#087F8C') }}>{club.branding?.logoUrl ? <img src={club.branding.logoUrl} alt="" referrerPolicy="no-referrer" /> : initials(club.shortName || club.name)}</span><span className="role-badge">{roleName(club.role)}</span></div><h2>{club.name}</h2><p className="club-meta">{club.shortName || 'Clube multidesportivo'}</p><p className="field-hint">Clube atribuído pela plataforma. A sua conta não pode criar ou escolher outro clube.</p><div className="club-colors">{club.branding && [club.branding.primaryColor, club.branding.secondaryColor, club.branding.accentColor].map((value, index) => <span className="color-dot" key={index} style={{ backgroundColor: value }} title={value} />)}<span className="field-hint">{club.brandingVersion > 0 ? `Identidade publicada · v${club.brandingVersion}` : 'Identidade por publicar'}</span></div><div className="divider" /><div className="toolbar"><button className="button secondary small" onClick={() => navigate('branding')}>Ver identidade visual <Icon name="arrow" /></button>{canManage(club) && <button className="link-button" disabled={!user.emailVerified} onClick={openEdit}>Editar dados</button>}</div></article>
                    {club.branding && <section className="card"><div className="card-header"><h2 className="card-title">Identidade publicada · {club.name}</h2><span className="badge">Versão {club.brandingVersion}</span></div><div className="card-body"><ClubPreview club={club} theme={club.branding} label="Publicado" /></div></section>}
                  </div>
                    : page === 'branding' ? <BrandingEditor key={club.id} club={club} onChanged={refreshClub} />
                      : <People key={club.id} club={club} />}
      </main>
    </div>
    {editing && club && <Modal labelledBy="club-form-title" onClose={() => { if (!action.busy) setEditing(false); }}><h2 id="club-form-title">Editar dados do clube</h2><p className="muted">Atualize os dados do clube que lhe foi atribuído.</p><Notice value={action.notice} /><form className="stack" onSubmit={saveClub}><Field label="Nome do clube"><input className="input" value={name} onChange={event => setName(event.target.value)} required minLength={2} maxLength={120} autoFocus /></Field><Field label="Nome curto (opcional)"><input className="input" value={shortName} onChange={event => setShortName(event.target.value)} maxLength={30} /></Field><div className="modal-actions"><button type="button" className="button secondary" disabled={action.busy} onClick={() => setEditing(false)}>Cancelar</button><button className="button primary" disabled={action.busy}>{action.busy ? 'A guardar…' : 'Guardar alterações'}</button></div></form></Modal>}
  </div>;
}
