export type User = { id: string; email: string; displayName: string; emailVerified: boolean };
export type Sponsor = { name: string; url: string };
export type Theme = {
  primaryColor: string; secondaryColor: string; accentColor: string;
  surfaceColor: string; textColor: string; logoUrl: string; coverUrl: string;
  sponsors: Sponsor[]; kitColors: string[];
};
export type Club = {
  id: string; name: string; shortName: string | null; role: string;
  branding: Theme | null; brandingVersion: number; archived: boolean;
};
export type BrandingVersion = { version: number; theme: Theme; createdAt: string };
export type Member = { userId: string; displayName: string; email: string; role: string };
export type Invitation = {
  id: string; email: string; role: string; expiresAt: string;
  acceptedAt: string | null; revokedAt: string | null;
};

export const defaultTheme: Theme = {
  primaryColor: '#087F8C', secondaryColor: '#102B30', accentColor: '#B4E66E',
  surfaceColor: '#FFFFFF', textColor: '#142B30', logoUrl: '', coverUrl: '', sponsors: [], kitColors: [],
};

export const roleName = (role: string) => ({
  president: 'Presidente', director: 'Direção', coach: 'Treinador', assistant: 'Treinador adjunto', staff: 'Equipa de apoio',
}[role] || role);
export const canManage = (club: Club) => ['president', 'director'].includes(club.role);
export const initials = (name: string) => name.trim().split(/\s+/).slice(0, 2).map(n => n[0]).join('').toUpperCase();
export const formatDate = (value: string) => new Intl.DateTimeFormat('pt-PT', {
  dateStyle: 'medium', timeStyle: 'short', timeZone: 'Europe/Lisbon',
}).format(new Date(value));
