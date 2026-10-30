import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import ColorPicker from './ColorPicker';
import { BRANDING_PRESETS, BRANDING_SWATCHES } from './colorPalettes';
import { contrastRatio } from './contrast';

function Pickers() {
  const [primary, setPrimary] = useState('#123456');
  const [secondary, setSecondary] = useState('#654321');
  return <><ColorPicker label="Principal" value={primary} onChange={setPrimary} /><ColorPicker label="Secundária" value={secondary} onChange={setSecondary} /></>;
}

describe('paleta de cores do clube', () => {
  it('começa recolhida e escolher vermelho altera apenas a cor principal', async () => {
    const user = userEvent.setup();
    render(<Pickers />);
    expect(screen.queryByRole('button', { name: 'Vermelho #DC2626' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Ver paleta de Principal' }));
    const palette = screen.getByRole('group', { name: 'Paleta de Principal' });
    expect(within(palette).getAllByRole('button')).toHaveLength(BRANDING_SWATCHES.length);
    await user.click(within(palette).getByRole('button', { name: 'Vermelho #DC2626' }));
    expect(screen.getByRole('textbox', { name: 'Principal' })).toHaveValue('#DC2626');
    expect(screen.getByRole('textbox', { name: 'Secundária' })).toHaveValue('#654321');
    expect(within(palette).getByRole('button', { name: 'Vermelho #DC2626' })).toHaveAttribute('aria-pressed', 'true');
    await user.click(screen.getByRole('button', { name: 'Fechar paleta de Principal' }));
    expect(screen.queryByRole('group', { name: 'Paleta de Principal' })).not.toBeInTheDocument();
  });

  it('preserva códigos personalizados durante a escrita sem passar valores inválidos ao seletor nativo', async () => {
    const user = userEvent.setup();
    render(<Pickers />);
    const text = screen.getByRole('textbox', { name: 'Principal' });
    const native = screen.getByLabelText('Escolher cor principal');
    await user.clear(text);
    await user.type(text, '#12');
    expect(text).toHaveValue('#12');
    expect(native).toHaveValue('#000000');
    await user.type(text, 'aBcD');
    expect(text).toHaveValue('#12aBcD');
    expect(native).toHaveValue('#12abcd');
    expect(screen.getByRole('textbox', { name: 'Secundária' })).toHaveValue('#654321');
  });

  it('permite percorrer cores pelo teclado e selecionar com Enter', async () => {
    const user = userEvent.setup();
    render(<Pickers />);
    await user.click(screen.getByRole('button', { name: 'Ver paleta de Principal' }));
    const white = screen.getByRole('button', { name: 'Branco #FFFFFF' });
    white.focus();
    await user.keyboard('{ArrowRight}{Enter}');
    expect(screen.getByRole('textbox', { name: 'Principal' })).toHaveValue('#FFF7E6');
  });

  it('oferece cores únicas e combinações com contraste de texto WCAG AA', () => {
    expect(BRANDING_SWATCHES.length).toBeGreaterThanOrEqual(40);
    expect(new Set(BRANDING_SWATCHES.map(color => color.value)).size).toBe(BRANDING_SWATCHES.length);
    expect(BRANDING_PRESETS).toHaveLength(12);
    expect(new Set(BRANDING_PRESETS.map(preset => preset.name)).size).toBe(12);
    for (const preset of BRANDING_PRESETS) {
      expect(contrastRatio(preset.textColor, preset.surfaceColor), preset.name).toBeGreaterThanOrEqual(4.5);
      for (const color of [preset.primaryColor, preset.secondaryColor, preset.accentColor, preset.surfaceColor, preset.textColor, ...preset.kitColors]) {
        expect(color).toMatch(/^#[\dA-F]{6}$/);
      }
    }
  });
});
