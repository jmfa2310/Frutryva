import { useId, useRef, useState } from 'react';
import { BRANDING_SWATCHES } from './colorPalettes';
import { contrastRatio } from './contrast';

export type ColorPickerProps = {
  label: string;
  value: string;
  onChange: (color: string) => void;
  id?: string;
};

export function ColorPicker({ label, value, onChange, id }: ColorPickerProps) {
  const generatedId = useId();
  const inputId = id || generatedId;
  const paletteId = inputId + '-palette';
  const [expanded, setExpanded] = useState(false);
  const valid = /^#[\da-f]{6}$/i.test(value);
  const swatches = useRef<HTMLDivElement>(null);

  return <fieldset className="branding-color-picker">
    <legend className="field-label">{label}</legend>
    <div className="swatch-control">
      <input className="color-input" type="color" aria-label={`Escolher cor ${label.toLowerCase()}`} value={valid ? value : '#000000'} onChange={event => onChange(event.target.value)} />
      <input className="input color-value" id={inputId} aria-label={label} value={value} onChange={event => onChange(event.target.value)} maxLength={7} pattern="#[0-9a-fA-F]{6}" autoComplete="off" spellCheck={false} aria-describedby={inputId + '-hint'} />
    </div>
    <span className="field-hint" id={inputId + '-hint'}>Escolha uma cor ou escreva o código hexadecimal.</span>
    <button type="button" className="branding-color-picker__toggle" aria-expanded={expanded} aria-controls={paletteId} onClick={() => setExpanded(previous => !previous)}>{expanded ? 'Fechar' : 'Ver'} paleta de {label}</button>
    {expanded && <div className="branding-color-picker__palette" id={paletteId} role="group" aria-label={`Paleta de ${label}`}>
      <div className="branding-color-picker__grid" ref={swatches} onKeyDown={event => {
        const buttons = Array.from(swatches.current?.querySelectorAll<HTMLButtonElement>('button') || []);
        const index = buttons.indexOf(event.target as HTMLButtonElement);
        if (index < 0) return;
        let target = index;
        if (event.key === 'ArrowRight') target = (index + 1) % buttons.length;
        else if (event.key === 'ArrowLeft') target = (index + buttons.length - 1) % buttons.length;
        else if (event.key === 'Home') target = 0;
        else if (event.key === 'End') target = buttons.length - 1;
        else return;
        event.preventDefault(); buttons[target]?.focus();
      }}>
        {BRANDING_SWATCHES.map(swatch => {
          const selected = value.toUpperCase() === swatch.value;
          const foreground = contrastRatio('#FFFFFF', swatch.value) > contrastRatio('#000000', swatch.value) ? '#FFFFFF' : '#000000';
          return <button key={swatch.value} type="button" className="branding-color-picker__swatch" style={{ backgroundColor: swatch.value, color: foreground }} aria-label={`${swatch.name} ${swatch.value}`} title={`${swatch.name} · ${swatch.value}`} aria-pressed={selected} onClick={() => onChange(swatch.value)}><span aria-hidden="true">{selected ? '✓' : ''}</span></button>;
        })}
      </div>
      <span className="field-hint">{BRANDING_SWATCHES.length} cores. Use também as setas do teclado para percorrer a paleta.</span>
    </div>}
  </fieldset>;
}

export default ColorPicker;
