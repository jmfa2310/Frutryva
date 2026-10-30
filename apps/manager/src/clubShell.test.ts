import { describe, expect, it } from 'vitest';
import { accessibleColor, clubShellStyle } from './clubShell';
import { contrastRatio } from './contrast';
import { defaultTheme } from './types';

describe('identidade publicada na navegação', () => {
  it('preserva a identidade padrão quando o clube ainda não publicou', () => {
    expect(clubShellStyle(null)).toEqual({});
    expect(clubShellStyle(undefined)).toEqual({});
  });

  it('usa as cores do clube e mantém texto legível em cores claras e escuras', () => {
    for (const background of ['#FFFFFF', '#000000', '#FFD700', '#AF3321', '#777777', '#006400', '#FFA500', '#3344CC']) {
      const style = clubShellStyle({ ...defaultTheme, primaryColor: background, secondaryColor: background }) as Record<string, string>;
      expect(style['--platform']).toBe(background);
      expect(style['--sidebar']).toBe(background);
      expect(contrastRatio(style['--platform-button-text'], background)).toBeGreaterThanOrEqual(4.5);
      expect(contrastRatio(style['--sidebar-text'], background)).toBeGreaterThanOrEqual(4.5);
      expect(contrastRatio(style['--platform-button-text'], style['--platform-hover'])).toBeGreaterThanOrEqual(4.5);
    }
    expect(accessibleColor('#FFFFFF')).toBe('#000000');
    expect(accessibleColor('#000000')).toBe('#FFFFFF');
  });

  it('não aplica valores CSS inválidos ao menu', () => {
    const style = clubShellStyle({ ...defaultTheme, primaryColor: 'url(evil)', secondaryColor: 'red; color:white' }) as Record<string, string>;
    expect(style['--platform']).toBe(defaultTheme.primaryColor);
    expect(style['--sidebar']).toBe(defaultTheme.secondaryColor);
  });
});
