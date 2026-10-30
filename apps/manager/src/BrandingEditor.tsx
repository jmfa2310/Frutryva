import { useEffect, useState, type CSSProperties, type ChangeEvent } from 'react';
import { api, errorMessage } from './api';
import { contrastRatio } from './contrast';
import ColorPicker from './ColorPicker';
import { BRANDING_PRESETS } from './colorPalettes';
import { Field, Modal, Notice, useAction } from './components';
import { canManage, defaultTheme, formatDate, initials, type BrandingVersion, type Club, type Theme } from './types';

export function ClubPreview({ club, theme, label }: { club: Club; theme: Theme; label: string }) {
  const style = { '--club-primary': theme.primaryColor, '--club-secondary': theme.secondaryColor, '--club-accent': theme.accentColor, '--club-surface': theme.surfaceColor, '--club-text': theme.textColor } as CSSProperties;
  return <div className="preview-panel" style={style}><div className="preview-cover" style={theme.coverUrl ? { backgroundImage: `url("${theme.coverUrl.replace(/["\\\n\r]/g, '')}")` } : undefined}><span className="badge">{label}</span></div><div className="preview-content"><div className="preview-logo">{theme.logoUrl ? <img src={theme.logoUrl} alt="Emblema do clube" referrerPolicy="no-referrer" /> : initials(club.shortName || club.name)}</div><span className="eyebrow">O SEU CLUBE</span><h3>{club.name}</h3><p>{club.shortName || 'A identidade que nos une.'}</p><div className="preview-nav"><span>Clube</span><span>Identidade</span></div><div className="club-colors" aria-label="Cores do equipamento">{theme.kitColors.map((color, index) => <span key={index} className="color-dot" style={{ backgroundColor: color }} title={color} />)}</div>{theme.sponsors.length > 0 && <div className="preview-sponsors"><span className="field-hint">Patrocinadores</span>{theme.sponsors.map((sponsor, index) => <span className="sponsor-tag" key={index}>{sponsor.name}</span>)}</div>}</div></div>;
}

export default function BrandingEditor({ club, onChanged }: { club: Club; onChanged: () => Promise<void> }) {
  const [theme, setTheme] = useState<Theme>({ ...defaultTheme });
  const [saved, setSaved] = useState('');
  const [versions, setVersions] = useState<BrandingVersion[]>([]);
  const [loading, setLoading] = useState(true);
  const [kitInput, setKitInput] = useState('');
  const [loadError, setLoadError] = useState('');
  const [rollbackVersion, setRollbackVersion] = useState<number | null>(null);
  const action = useAction();
  const manage = canManage(club);
  useEffect(() => {
    let cancelled = false;
    setLoading(true); setLoadError('');
    if (!manage) { setLoading(false); return; }
    Promise.all([
      api<{ theme: Theme }>(`/v1/clubs/${club.id}/branding/draft`),
      api<{ versions: BrandingVersion[] }>(`/v1/clubs/${club.id}/branding/versions`),
    ]).then(([draft, history]) => {
      if (cancelled) return;
      const loaded = { ...defaultTheme, ...draft.theme, sponsors: draft.theme.sponsors || [], kitColors: draft.theme.kitColors || [] };
      setTheme(loaded); setSaved(JSON.stringify(loaded)); setKitInput(loaded.kitColors.join(', ')); setVersions(history.versions);
    }).catch(error => { if (!cancelled) setLoadError(errorMessage(error)); }).finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [club.id, manage]);
  const dirty = JSON.stringify(theme) !== saved;
  const contrast = contrastRatio(theme.textColor, theme.surfaceColor);
  const set = (key: keyof Theme, value: string) => setTheme(previous => ({ ...previous, [key]: value }));
  async function refreshHistory() {
    const history = await api<{ versions: BrandingVersion[] }>(`/v1/clubs/${club.id}/branding/versions`);
    setVersions(history.versions);
  }
  async function save() {
    const result = await api<{ theme: Theme }>(`/v1/clubs/${club.id}/branding/draft`, 'PUT', { theme });
    const resultTheme = { ...defaultTheme, ...result.theme, sponsors: result.theme.sponsors || [], kitColors: result.theme.kitColors || [] };
    setTheme(resultTheme); setSaved(JSON.stringify(resultTheme));
    return resultTheme;
  }
  function upload(key: 'logoUrl' | 'coverUrl', event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file) return;
    if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) {
      action.setNotice({ type: 'error', text: 'Este formato não é suportado. Escolha uma imagem PNG, JPEG ou WebP.' }); event.target.value = ''; return;
    }
    if (file.size > 200 * 1024) {
      action.setNotice({ type: 'error', text: `A imagem tem ${Math.ceil(file.size / 1024)} KB. O limite é 200 KB; reduza a imagem antes de a carregar.` }); event.target.value = ''; return;
    }
    const reader = new FileReader();
    reader.onload = () => { set(key, String(reader.result)); action.setNotice(null); };
    reader.onerror = () => action.setNotice({ type: 'error', text: 'Não foi possível ler a imagem.' });
    reader.readAsDataURL(file);
  }
  if (loading) return <div className="loading-state" role="status">A carregar a identidade do clube…</div>;
  if (loadError) return <Notice value={{ type: 'error', text: loadError }} />;
  if (!manage) return <section className="card"><div className="card-header"><h2 className="card-title">Identidade publicada</h2><span className="badge">Só de leitura</span></div><div className="card-body">{club.branding ? <ClubPreview club={club} theme={club.branding} label={`Versão ${club.brandingVersion}`} /> : <p className="muted">A direção ainda não publicou a identidade deste clube.</p>}</div></section>;
  const colors: [keyof Theme, string][] = [['primaryColor', 'Principal'], ['secondaryColor', 'Secundária'], ['accentColor', 'Destaque'], ['surfaceColor', 'Fundo'], ['textColor', 'Texto']];
  return <div className="stack"><Notice value={action.notice} /><div className="grid two-columns"><div className="stack">
    <section className="card"><div className="card-header"><h2 className="card-title">Identidade em rascunho</h2><span className="badge">{dirty ? 'Alterações por guardar' : 'Guardado'}</span></div><div className="card-body stack"><p className="muted">Experimente as cores e imagens. As alterações só ficam públicas depois de publicar.</p><section className="branding-presets" aria-label="Combinações de cores"><h3>Combinações prontas</h3><p className="field-hint">Escolha uma combinação para começar ou personalize cada cor abaixo. Emblema, capa e patrocinadores são mantidos.</p><div className="branding-preset-grid">{BRANDING_PRESETS.map(preset => <button type="button" className="branding-preset" key={preset.name} aria-label={`Aplicar ${preset.name}`} aria-pressed={colors.every(([key]) => String(theme[key]).toLowerCase() === String(preset[key as keyof typeof preset]).toLowerCase())} onClick={() => { const { name: _name, ...values } = preset; setTheme(previous => ({ ...previous, ...values, kitColors: [...values.kitColors] })); setKitInput(values.kitColors.join(', ')); action.setNotice({ type: 'info', text: 'Combinação aplicada ao rascunho. Guarde e publique quando estiver pronta.' }); }}><span className="branding-preset-colors" aria-hidden="true">{[preset.primaryColor, preset.secondaryColor, preset.accentColor].map((color, index) => <span key={index} style={{ backgroundColor: color }} />)}</span><span>{preset.name}</span></button>)}</div></section><div className="form-grid">{colors.map(([key, label]) => <ColorPicker key={key} id={`color-${key}`} label={label} value={String(theme[key])} onChange={value => set(key, value)} />)}</div><Notice value={{ type: contrast >= 4.5 ? 'success' : 'warning', text: `Contraste entre texto e fundo: ${contrast.toFixed(2)}:1. ${contrast >= 4.5 ? 'Cumpre o mínimo WCAG AA.' : 'Ajuste as cores para atingir pelo menos 4,5:1.'}` }} />
      {(['logoUrl', 'coverUrl'] as const).map((key, index) => <div className="stack" key={key}><Field label={index === 0 ? 'Emblema' : 'Imagem de capa'} hint="Carregue PNG, JPEG ou WebP até 200 KB, ou use uma ligação HTTPS."><input className="input" type="file" accept="image/png,image/jpeg,image/webp" onChange={event => upload(key, event)} /></Field><Field label={index === 0 ? 'Ligação do emblema (opcional)' : 'Ligação da capa (opcional)'}><input className="input" type="url" value={theme[key].startsWith('data:') ? '' : theme[key]} placeholder="https://…" onChange={event => set(key, event.target.value)} /></Field>{theme[key] && <button className="link-button" onClick={() => set(key, '')}>Remover {index === 0 ? 'emblema' : 'capa'}</button>}</div>)}
      <Field label="Cores do equipamento" hint="Separe as cores por vírgulas, por exemplo: #087F8C, #FFFFFF."><input className="input" value={kitInput} onChange={event => { setKitInput(event.target.value); setTheme(previous => ({ ...previous, kitColors: event.target.value.split(',').map(color => color.trim()).filter(Boolean) })); }} /></Field>
      <div className="section-heading"><h3>Patrocinadores</h3><button className="button secondary small" onClick={() => setTheme(previous => ({ ...previous, sponsors: [...previous.sponsors, { name: '', url: '' }] }))}>Adicionar</button></div>{theme.sponsors.length === 0 && <p className="field-hint">Ainda não adicionou patrocinadores.</p>}{theme.sponsors.map((sponsor, index) => <div className="stack" key={index}><div className="form-grid"><Field label={`Nome do patrocinador ${index + 1}`}><input className="input" value={sponsor.name} maxLength={100} onChange={event => setTheme(previous => ({ ...previous, sponsors: previous.sponsors.map((item, i) => i === index ? { ...item, name: event.target.value } : item) }))} /></Field><Field label={`Ligação do patrocinador ${index + 1}`}><input className="input" type="url" value={sponsor.url} placeholder="https://…" onChange={event => setTheme(previous => ({ ...previous, sponsors: previous.sponsors.map((item, i) => i === index ? { ...item, url: event.target.value } : item) }))} /></Field></div><button className="link-button" onClick={() => setTheme(previous => ({ ...previous, sponsors: previous.sponsors.filter((_, i) => i !== index) }))}>Remover patrocinador {index + 1}</button></div>)}
      <div className="toolbar"><button className="button secondary" disabled={action.busy || contrast < 4.5} onClick={() => void action.run(async () => { await save(); action.setNotice({ type: 'success', text: 'Rascunho guardado. A versão pública mantém-se até publicar.' }); })}>{action.busy ? 'A guardar…' : 'Guardar rascunho'}</button><button className="button primary" disabled={action.busy || contrast < 4.5} onClick={() => void action.run(async () => { await save(); await api(`/v1/clubs/${club.id}/branding/publish`, 'POST', {}); await onChanged(); await refreshHistory(); action.setNotice({ type: 'success', text: 'Identidade publicada. A nova versão está disponível.' }); })}>Guardar e publicar</button></div>
    </div></section>
  </div><div className="stack"><section className="card"><div className="card-header"><h2 className="card-title">Pré-visualização</h2><span className="badge">Rascunho privado</span></div><div className="card-body"><ClubPreview club={club} theme={theme} label="Pré-visualização" /></div></section><section className="card"><div className="card-header"><h2 className="card-title">Histórico de publicações</h2></div><div className="card-body">{versions.length === 0 ? <p className="muted">Ainda não existem versões publicadas.</p> : <div className="version-list">{versions.map(version => <div className="version-item" key={version.version}><div><strong>Versão {version.version}</strong><p className="field-hint">{formatDate(version.createdAt)}</p></div>{version.version === club.brandingVersion ? <span className="badge">Atual</span> : <button className="button secondary small" disabled={action.busy} onClick={() => setRollbackVersion(version.version)}>Restaurar</button>}</div>)}</div>}<p className="field-hint">Restaurar cria uma nova publicação. O histórico é preservado.</p></div></section></div></div>
    {rollbackVersion !== null && <Modal labelledBy="rollback-title" onClose={() => { if (!action.busy) setRollbackVersion(null); }}><h2 id="rollback-title">Restaurar a versão {rollbackVersion}?</h2><p>Será criada uma nova publicação com esta identidade. O rascunho também será atualizado para esta versão.</p><div className="modal-actions"><button className="button secondary" onClick={() => setRollbackVersion(null)}>Cancelar</button><button className="button primary" disabled={action.busy} onClick={() => void action.run(async () => { const result = await api<{ club: Club }>(`/v1/clubs/${club.id}/branding/rollback`, 'POST', { version: rollbackVersion }); if (result.club.branding) { const restored = { ...result.club.branding, sponsors: result.club.branding.sponsors || [], kitColors: result.club.branding.kitColors || [] }; setTheme(restored); setSaved(JSON.stringify(restored)); setKitInput(restored.kitColors.join(', ')); } setRollbackVersion(null); await onChanged(); await refreshHistory(); action.setNotice({ type: 'success', text: 'Versão restaurada numa nova publicação.' }); })}>Confirmar restauro</button></div></Modal>}
  </div>;
}
