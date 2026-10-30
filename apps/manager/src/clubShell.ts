import type { CSSProperties } from 'react';
import { contrastRatio } from './contrast';
import { defaultTheme, type Theme } from './types';

const color = (value: string, fallback: string) => /^#[\da-f]{6}$/i.test(value) ? value : fallback;

// Both candidates always provide at least AA contrast for a solid RGB surface.
export function accessibleColor(background: string): '#000000' | '#FFFFFF' {
  return contrastRatio('#FFFFFF', background) >= contrastRatio('#000000', background) ? '#FFFFFF' : '#000000';
}

function blend(background: string, foreground: string, amount: number) {
  return '#' + [1, 3, 5].map(index => {
    const start = parseInt(background.slice(index, index + 2), 16);
    const end = parseInt(foreground.slice(index, index + 2), 16);
    return Math.round(start + (end - start) * amount).toString(16).padStart(2, '0');
  }).join('');
}

// Only call this with the club's published theme. Draft editing stays local to
// the preview component and cannot change the surrounding application shell.
export function clubShellStyle(theme: Theme | null | undefined): CSSProperties {
  if (!theme) return {};
  const primary = color(theme.primaryColor, defaultTheme.primaryColor);
  const secondary = color(theme.secondaryColor, defaultTheme.secondaryColor);
  const accent = color(theme.accentColor, defaultTheme.accentColor);
  const surface = color(theme.surfaceColor, defaultTheme.surfaceColor);
  const text = color(theme.textColor, defaultTheme.textColor);
  const primaryText = accessibleColor(primary);
  const sidebarText = accessibleColor(secondary);
  const hover = blend(primary, primaryText === '#FFFFFF' ? '#000000' : '#FFFFFF', 0.12);
  return {
    '--platform': primary,
    '--platform-hover': hover,
    '--platform-button-text': primaryText,
    '--platform-hover-text': primaryText,
    '--platform-ink': contrastRatio(primary, '#F4F6F5') >= 4.5 ? primary : '#182C32',
    '--sidebar': secondary,
    '--sidebar-text': sidebarText,
    '--sidebar-muted': sidebarText,
    '--sidebar-active': primary,
    '--sidebar-active-text': primaryText,
    '--sidebar-line': blend(secondary, sidebarText, 0.25),
    '--sidebar-avatar': primary,
    '--sidebar-avatar-text': primaryText,
    '--club-primary': primary,
    '--club-secondary': secondary,
    '--club-accent': accent,
    '--club-surface': surface,
    '--club-text': text,
  } as CSSProperties;
}
