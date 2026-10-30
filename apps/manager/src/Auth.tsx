import { useState, type FormEvent } from 'react';
import { api } from './api';
import { Brand, Field, Icon, Notice, useAction } from './components';
import type { User } from './types';

export default function Auth({ onAuthenticated }: { onAuthenticated: (user: User, deliveryPending?: boolean) => void }) {
  const [mode, setMode] = useState<'login' | 'register' | 'forgot' | 'reset'>('login');
  const [email, setEmail] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [password, setPassword] = useState('');
  const [token, setToken] = useState('');
  const action = useAction();
  const titles = { login: 'O seu clube começa aqui.', register: 'Dê o próximo passo.', forgot: 'Recuperar o acesso.', reset: 'Uma nova palavra-passe.' };
  const changeMode = (next: typeof mode) => { setMode(next); setPassword(''); setToken(''); action.setNotice(null); };

  function submit(event: FormEvent) {
    event.preventDefault();
    void action.run(async () => {
      if (mode === 'forgot') {
        await api('/v1/auth/forgot-password', 'POST', { email });
        action.setNotice({ type: 'success', text: 'Se existir uma conta com este email, receberá um código de recuperação. Verifique a sua caixa de correio.' });
      } else if (mode === 'reset') {
        await api('/v1/auth/reset-password', 'POST', { token: token.trim(), password });
        setMode('login'); setPassword(''); setToken('');
        action.setNotice({ type: 'success', text: 'Palavra-passe atualizada. Já pode iniciar sessão.' });
      } else {
        const result = await api<{ user: User; emailDeliveryPending?: boolean }>('/v1/auth/' + mode, 'POST', { email, password, ...(mode === 'register' ? { displayName } : {}) });
        onAuthenticated(result.user, result.emailDeliveryPending);
      }
    });
  }

  return <main className="auth-shell">
    <section className="auth-story" aria-label="FUTRYVA Manager">
      <div className="auth-story-inner"><Brand /><span className="eyebrow">CADA CLUBE. UMA IDENTIDADE.</span><h1>Mais organização.<br />Mais tempo<br />para o desporto.</h1><p>O seu acesso à FUTRYVA. Gestão do clube e administração da plataforma em espaços separados.</p><div className="step-list"><span><Icon name="check" /> Um clube por conta de cliente</span><span><Icon name="check" /> Pessoas com as permissões certas</span><span><Icon name="check" /> A identidade do clube, nas suas mãos</span></div><span className="muted">FUTRYVA · Plataforma multidesportiva em desenvolvimento</span></div>
    </section>
    <section className="auth-panel"><div className="auth-card">
      <div className="auth-brand"><Brand /></div>
      <span className="eyebrow">BEM-VINDO À FUTRYVA</span><h2>{titles[mode]}</h2>
      <p className="muted">{mode === 'register' ? 'Crie a sua conta e confirme o email para começar.' : mode === 'forgot' ? 'Enviamos um código para o email da sua conta.' : mode === 'reset' ? 'Use o código recebido por email e escolha uma nova palavra-passe.' : 'Entre no seu espaço de gestão.'}</p>
      {['login', 'register'].includes(mode) && <div className="auth-tabs" role="group" aria-label="Acesso à conta"><button className={mode === 'login' ? 'active' : ''} onClick={() => changeMode('login')}>Entrar</button><button className={mode === 'register' ? 'active' : ''} onClick={() => changeMode('register')}>Criar conta</button></div>}
      <Notice value={action.notice} />
      <form onSubmit={submit} className="stack" aria-label={mode === 'register' ? 'Criar conta' : mode === 'login' ? 'Iniciar sessão' : 'Recuperar acesso'}>
        {mode === 'register' && <Field label="Nome"><input className="input" name="displayName" autoComplete="name" value={displayName} onChange={e => setDisplayName(e.target.value)} required minLength={2} maxLength={100} /></Field>}
        {mode !== 'reset' && <Field label="Email"><input className="input" name="email" type="email" autoComplete="email" value={email} onChange={e => setEmail(e.target.value)} required maxLength={254} placeholder="nome@clube.pt" /></Field>}
        {mode === 'reset' && <Field label="Código de recuperação"><input className="input" name="token" autoComplete="off" spellCheck={false} value={token} onChange={e => setToken(e.target.value)} required /></Field>}
        {mode !== 'forgot' && <Field label="Palavra-passe" hint={mode !== 'login' ? 'Use pelo menos 12 caracteres. Evite reutilizar uma palavra-passe.' : undefined}><input className="input" name="password" type="password" autoComplete={mode === 'login' ? 'current-password' : 'new-password'} value={password} onChange={e => setPassword(e.target.value)} required minLength={mode === 'login' ? undefined : 12} maxLength={72} /></Field>}
        {mode === 'login' && <button type="button" className="link-button" onClick={() => changeMode('forgot')}>Esqueci-me da palavra-passe</button>}
        <button className="button primary" disabled={action.busy}>{action.busy ? 'Aguarde…' : mode === 'register' ? 'Criar conta' : mode === 'forgot' ? 'Enviar código de recuperação' : mode === 'reset' ? 'Guardar palavra-passe' : 'Entrar no Manager'} <Icon name="arrow" /></button>
      </form>
      {mode === 'forgot' && <button className="link-button" onClick={() => changeMode('reset')}>Já tenho um código de recuperação</button>}
      {['forgot', 'reset'].includes(mode) && <button className="link-button" onClick={() => changeMode('login')}>Voltar ao início de sessão</button>}
      <p className="field-hint form-footer">Os dados de cada clube são privados. O acesso depende das permissões atribuídas à sua conta.</p>
    </div></section>
  </main>;
}
