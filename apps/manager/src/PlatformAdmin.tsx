import { useEffect, useState } from 'react';
import Account from './Account';
import { Brand, Icon, Notice, useAction } from './components';
import PlatformClubs from './PlatformClubs';
import { initials, type User } from './types';

// The platform operator has its own shell. It never receives a club theme or
// loads tenant routes, even if an old session still contains an assignment.
export default function PlatformAdmin({ user, refreshUser, onLogout }: {
  user: User;
  refreshUser: () => Promise<void>;
  onLogout: () => Promise<void>;
}) {
  const [page, setPage] = useState<'clubs' | 'account'>('clubs');
  const action = useAction();
  const title = page === 'clubs' ? 'Clubes da plataforma' : 'A minha conta';
  useEffect(() => {
    const previous = document.title;
    document.title = 'FUTRYVA Admin';
    return () => { document.title = previous; };
  }, []);

  return <div className="app-shell platform-admin">
    <a className="skip-link" href="#main-content">Saltar para o conteúdo</a>
    <aside className="sidebar">
      <Brand kind="admin" />
      <div className="sidebar-caption">ADMINISTRAÇÃO</div>
      <nav aria-label="Navegação da administração">
        <button className={`nav-button ${page === 'clubs' ? 'active' : ''}`} aria-current={page === 'clubs' ? 'page' : undefined} onClick={() => { setPage('clubs'); action.setNotice(null); }}><Icon name="clubs" />Clubes da plataforma</button>
        <button className={`nav-button ${page === 'account' ? 'active' : ''}`} aria-current={page === 'account' ? 'page' : undefined} onClick={() => { setPage('account'); action.setNotice(null); }}><Icon name="account" />A minha conta</button>
      </nav>
      <div className="sidebar-foot"><span className="user-avatar">{initials(user.displayName)}</span><div className="user-details"><strong>{user.displayName}</strong><span>Dono da plataforma</span></div><button className="icon-button" aria-label="Terminar sessão" disabled={action.busy} onClick={() => void action.run(onLogout)}><Icon name="logout" /></button></div>
    </aside>
    <div className="workspace">
      <header className="topbar"><div className="breadcrumb">Administração <span>/</span> <strong>{page === 'clubs' ? 'Clubes' : 'Conta'}</strong></div><div className="topbar-actions"><button className="button secondary small" onClick={() => setPage('account')}>Conta da plataforma</button></div></header>
      <main className="page" id="main-content">
        <div className="page-heading"><div><span className="eyebrow">FUTRYVA ADMIN</span><h1 className="page-title">{title}</h1><p className="page-description">{page === 'clubs' ? 'Crie os clubes e atribua cada um à conta do respetivo responsável.' : 'Gira a identidade e a sessão da sua conta de administração.'}</p></div></div>
        <Notice value={action.notice} />
        {!user.emailVerified && page !== 'account' && <div className="notice warning" role="status">Confirme o email da sua conta para administrar a plataforma. <button className="link-button" onClick={() => setPage('account')}>Confirmar agora</button></div>}
        {page === 'clubs' ? <PlatformClubs onChanged={refreshUser} /> : <Account user={user} refreshUser={refreshUser} onAccepted={async () => {}} />}
      </main>
    </div>
  </div>;
}
