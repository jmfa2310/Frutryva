import { useEffect, useState, type FormEvent } from 'react';
import { api, errorMessage } from './api';
import { Field, Notice, useAction } from './components';
import { canManage, formatDate, roleName, type Club, type Invitation, type Member } from './types';

export default function People({ club }: { club: Club }) {
  const [members, setMembers] = useState<Member[]>([]);
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [email, setEmail] = useState('');
  const [role, setRole] = useState('coach');
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const action = useAction();
  async function reload() {
    const [memberResult, invitationResult] = await Promise.all([
      api<{ members: Member[] }>(`/v1/clubs/${club.id}/members`),
      api<{ invitations: Invitation[] }>(`/v1/clubs/${club.id}/invitations`),
    ]);
    setMembers(memberResult.members); setInvitations(invitationResult.invitations);
  }
  useEffect(() => {
    let cancelled = false;
    setLoading(true); setLoadError('');
    if (!canManage(club)) { setLoading(false); return; }
    Promise.all([
      api<{ members: Member[] }>(`/v1/clubs/${club.id}/members`),
      api<{ invitations: Invitation[] }>(`/v1/clubs/${club.id}/invitations`),
    ]).then(([memberResult, invitationResult]) => { if (!cancelled) { setMembers(memberResult.members); setInvitations(invitationResult.invitations); } })
      .catch(error => { if (!cancelled) setLoadError(errorMessage(error)); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [club.id, club.role]);
  function invite(event: FormEvent) {
    event.preventDefault();
    void action.run(async () => {
      await api(`/v1/clubs/${club.id}/invitations`, 'POST', { email, role });
      setEmail(''); await reload();
      action.setNotice({ type: 'success', text: 'Convite enviado por email. O destinatário deve aceitá-lo na sua própria conta.' });
    });
  }
  if (!canManage(club)) return <Notice value={{ type: 'info', text: 'A gestão de membros e convites está reservada à presidência e à direção deste clube.' }} />;
  if (loading) return <div className="loading-state" role="status">A carregar membros e convites…</div>;
  if (loadError) return <Notice value={{ type: 'error', text: loadError }} />;
  return <div className="stack"><Notice value={action.notice} /><div className="grid two-columns">
    <section className="card"><div className="card-header"><h2 className="card-title">Convidar uma pessoa</h2></div><div className="card-body stack"><p className="muted">O convite é pessoal, tem validade limitada e só pode ser aceite uma vez.</p><form className="stack" onSubmit={invite}><Field label="Email do destinatário"><input className="input" type="email" autoComplete="off" value={email} onChange={event => setEmail(event.target.value)} required maxLength={254} placeholder="treinador@exemplo.pt" /></Field><Field label="Função no clube"><select className="select" value={role} onChange={event => setRole(event.target.value)}><option value="coach">Treinador</option><option value="assistant">Treinador adjunto</option>{club.role === 'president' && <option value="director">Direção</option>}</select></Field><button className="button primary" disabled={action.busy}>{action.busy ? 'A enviar…' : 'Enviar convite'}</button></form></div></section>
    <section className="card"><div className="card-header"><h2 className="card-title">Quem pode fazer o quê?</h2></div><div className="card-body stack"><p><strong>Presidente</strong><br /><span className="muted">Gere o clube, a identidade e os convites. Pode arquivar o clube.</span></p><p><strong>Direção</strong><br /><span className="muted">Gere a identidade e convida treinadores e adjuntos.</span></p><p><strong>Treinadores e adjuntos</strong><br /><span className="muted">Consultam os clubes a que pertencem e a identidade publicada.</span></p><p className="field-hint">As permissões são verificadas no servidor em cada pedido. A gestão de equipas será adicionada na próxima fase.</p></div></section>
  </div>
    <section className="card"><div className="card-header"><h2 className="card-title">Membros do clube</h2><span className="badge">{members.length} {members.length === 1 ? 'membro' : 'membros'}</span></div><div className="table-wrap"><table className="table"><thead><tr><th scope="col">Nome</th><th scope="col">Email</th><th scope="col">Função</th></tr></thead><tbody>{members.map(member => <tr key={member.userId + member.role}><td><strong>{member.displayName}</strong></td><td>{member.email}</td><td><span className="role-badge">{roleName(member.role)}</span></td></tr>)}</tbody></table></div></section>
    <section className="card"><div className="card-header"><h2 className="card-title">Convites enviados</h2></div>{invitations.length === 0 ? <div className="card-body"><p className="muted">Os convites enviados aparecerão aqui.</p></div> : <div className="table-wrap"><table className="table"><thead><tr><th scope="col">Email</th><th scope="col">Função</th><th scope="col">Validade</th><th scope="col">Estado</th><th scope="col"><span className="visually-hidden">Ações</span></th></tr></thead><tbody>{invitations.map(invitation => {
      const expired = new Date(invitation.expiresAt).getTime() <= Date.now();
      const state = invitation.acceptedAt ? 'Aceite' : invitation.revokedAt ? 'Revogado' : expired ? 'Expirado' : 'Pendente';
      return <tr key={invitation.id}><td>{invitation.email}</td><td>{roleName(invitation.role)}</td><td>{formatDate(invitation.expiresAt)}</td><td><span className="badge">{state}</span></td><td>{state === 'Pendente' && <button className="button secondary small" aria-label={`Revogar convite para ${invitation.email}`} disabled={action.busy} onClick={() => void action.run(async () => { await api(`/v1/clubs/${club.id}/invitations/${invitation.id}`, 'DELETE'); await reload(); action.setNotice({ type: 'success', text: 'Convite revogado.' }); })}>Revogar</button>}</td></tr>;
    })}</tbody></table></div>}</section>
  </div>;
}
