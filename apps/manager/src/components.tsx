import { cloneElement, isValidElement, useEffect, useId, useRef, useState, type ReactElement, type ReactNode } from 'react';
import { errorMessage } from './api';

export function Icon({ name }: { name: 'clubs' | 'brand' | 'people' | 'account' | 'plus' | 'arrow' | 'logout' | 'check' }) {
  const paths = {
    clubs: 'M3 21V7l9-4 9 4v14M8 21v-6h8v6M7 9h1m8 0h1M7 12h1m8 0h1',
    brand: 'M12 3a9 9 0 1 0 0 18h1a2 2 0 0 0 1-3.7 1.5 1.5 0 0 1 1-2.8h2A4 4 0 0 0 21 10a9 9 0 0 0-9-7ZM7 9h.01M11 7h.01M16 8h.01M6 13h.01',
    people: 'M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75M13 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0',
    account: 'M20 21a8 8 0 0 0-16 0M16 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0',
    plus: 'M12 5v14M5 12h14', arrow: 'M5 12h14m-6-6 6 6-6 6',
    logout: 'M9 3H4v18h5M10 12h11m-5-5 5 5-5 5', check: 'm5 12 4 4L19 6',
  };
  return <svg aria-hidden="true" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round"><path d={paths[name]} /></svg>;
}

export type NoticeValue = { type: 'error' | 'success' | 'info' | 'warning'; text: string } | null;
export function Notice({ value }: { value: NoticeValue }) {
  return value ? <div className={`notice ${value.type}`} role={value.type === 'error' ? 'alert' : 'status'}>{value.text}</div> : null;
}
export function useAction() {
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<NoticeValue>(null);
  async function run(action: () => Promise<void>) {
    setBusy(true); setNotice(null);
    try { await action(); }
    catch (error) { setNotice({ type: 'error', text: errorMessage(error) }); }
    finally { setBusy(false); }
  }
  return { busy, notice, setNotice, run };
}

export function Field({ label, hint, children, wide = false }: { label: string; hint?: string; children: ReactNode; wide?: boolean }) {
  const id = useId();
  const child = isValidElement(children) ? cloneElement(children as ReactElement<{ id?: string; 'aria-describedby'?: string }>, { id, 'aria-describedby': hint ? id + '-hint' : undefined }) : children;
  return <div className={`field${wide ? ' field-wide' : ''}`}><label className="field-label" htmlFor={id}>{label}</label>{child}{hint && <span className="field-hint" id={id + '-hint'}>{hint}</span>}</div>;
}

export function Brand({ kind = 'manager' }: { kind?: 'manager' | 'admin' } = {}) {
  return <div className="brand"><span className="brand-mark" aria-hidden="true">F</span><span><span className="brand-name">FUTRYVA</span><span className="brand-tag">{kind === 'admin' ? 'ADMIN' : 'MANAGER'}</span></span></div>;
}

export function Modal({ labelledBy, onClose, children }: { labelledBy: string; onClose: () => void; children: ReactNode }) {
  const container = useRef<HTMLElement>(null);
  const close = useRef(onClose);
  const previousFocus = useRef(document.activeElement as HTMLElement | null);
  close.current = onClose;
  useEffect(() => {
    const previous = previousFocus.current;
    const element = container.current;
    if (!element) return;
    const selector = 'button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), a[href], [tabindex="0"]';
    const focusable = () => Array.from(element.querySelectorAll<HTMLElement>('*')).filter(child => child.matches(selector));
    (focusable()[0] || element).focus();
    const keydown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { event.preventDefault(); close.current(); }
      if (event.key !== 'Tab') return;
      const targets = focusable();
      if (!targets.length) { event.preventDefault(); element.focus(); return; }
      const first = targets[0], last = targets[targets.length - 1];
      if (event.shiftKey && (document.activeElement === first || document.activeElement === element)) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    element.addEventListener('keydown', keydown);
    return () => { element.removeEventListener('keydown', keydown); previous?.focus(); };
  }, []);
  return <div className="modal-backdrop"><section className="modal" ref={container} tabIndex={-1} role="dialog" aria-modal="true" aria-labelledby={labelledBy}>{children}</section></div>;
}
