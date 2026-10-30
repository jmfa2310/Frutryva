export type BrandingSwatch = { name: string; value: string };

export const BRANDING_SWATCHES: BrandingSwatch[] = [
  { name: 'Branco', value: '#FFFFFF' },
  { name: 'Marfim', value: '#FFF7E6' },
  { name: 'Amarelo claro', value: '#FEF08A' },
  { name: 'Amarelo', value: '#FACC15' },
  { name: 'Dourado', value: '#D4A017' },
  { name: 'Âmbar', value: '#F59E0B' },
  { name: 'Laranja', value: '#F97316' },
  { name: 'Laranja escuro', value: '#C2410C' },
  { name: 'Vermelho', value: '#DC2626' },
  { name: 'Vermelho escuro', value: '#991B1B' },
  { name: 'Bordô', value: '#7F1D1D' },
  { name: 'Vinho', value: '#6B1635' },
  { name: 'Coral', value: '#FB7185' },
  { name: 'Rosa', value: '#EC4899' },
  { name: 'Rosa claro', value: '#F9A8D4' },
  { name: 'Magenta', value: '#C026D3' },
  { name: 'Lilás', value: '#C4B5FD' },
  { name: 'Violeta', value: '#8B5CF6' },
  { name: 'Roxo', value: '#7E22CE' },
  { name: 'Roxo escuro', value: '#581C87' },
  { name: 'Azul marinho', value: '#172554' },
  { name: 'Azul escuro', value: '#1E3A8A' },
  { name: 'Azul real', value: '#1D4ED8' },
  { name: 'Azul', value: '#2563EB' },
  { name: 'Azul celeste', value: '#38BDF8' },
  { name: 'Azul claro', value: '#BAE6FD' },
  { name: 'Ciano', value: '#06B6D4' },
  { name: 'Turquesa', value: '#14B8A6' },
  { name: 'Verde água', value: '#5EEAD4' },
  { name: 'Verde petróleo', value: '#115E59' },
  { name: 'Verde escuro', value: '#14532D' },
  { name: 'Verde', value: '#15803D' },
  { name: 'Verde esmeralda', value: '#10B981' },
  { name: 'Verde lima', value: '#84CC16' },
  { name: 'Verde claro', value: '#BBF7D0' },
  { name: 'Verde azeitona', value: '#4D7C0F' },
  { name: 'Castanho', value: '#78350F' },
  { name: 'Caramelo', value: '#A16207' },
  { name: 'Bege', value: '#D6C5A0' },
  { name: 'Areia', value: '#E7DCC3' },
  { name: 'Preto', value: '#000000' },
  { name: 'Carvão', value: '#1F2937' },
  { name: 'Cinzento escuro', value: '#4B5563' },
  { name: 'Cinzento', value: '#6B7280' },
  { name: 'Cinzento claro', value: '#D1D5DB' },
  { name: 'Prata', value: '#E5E7EB' },
  { name: 'Azul petróleo', value: '#0E7490' },
  { name: 'Rosa escuro', value: '#9D174D' },
];

export type BrandingPreset = {
  name: string;
  primaryColor: string;
  secondaryColor: string;
  accentColor: string;
  surfaceColor: string;
  textColor: string;
  kitColors: string[];
};

export const BRANDING_PRESETS: BrandingPreset[] = [
  { name: 'Vermelho e branco', primaryColor: '#DC2626', secondaryColor: '#7F1D1D', accentColor: '#FACC15', surfaceColor: '#FFFFFF', textColor: '#292524', kitColors: ['#DC2626', '#FFFFFF'] },
  { name: 'Azul e branco', primaryColor: '#1D4ED8', secondaryColor: '#172554', accentColor: '#38BDF8', surfaceColor: '#FFFFFF', textColor: '#172554', kitColors: ['#1D4ED8', '#FFFFFF'] },
  { name: 'Verde e branco', primaryColor: '#15803D', secondaryColor: '#14532D', accentColor: '#BBF7D0', surfaceColor: '#FFFFFF', textColor: '#14532D', kitColors: ['#15803D', '#FFFFFF'] },
  { name: 'Preto e dourado', primaryColor: '#D4A017', secondaryColor: '#111827', accentColor: '#FEF08A', surfaceColor: '#111827', textColor: '#F9FAFB', kitColors: ['#111827', '#D4A017'] },
  { name: 'Bordô e marfim', primaryColor: '#6B1635', secondaryColor: '#3B1020', accentColor: '#D4A017', surfaceColor: '#FFF7E6', textColor: '#591D2D', kitColors: ['#6B1635', '#FFF7E6'] },
  { name: 'Amarelo e azul', primaryColor: '#FACC15', secondaryColor: '#1E3A8A', accentColor: '#38BDF8', surfaceColor: '#FFFFFF', textColor: '#172554', kitColors: ['#FACC15', '#1E3A8A'] },
  { name: 'Laranja e preto', primaryColor: '#F97316', secondaryColor: '#1F2937', accentColor: '#FACC15', surfaceColor: '#FFF7ED', textColor: '#292524', kitColors: ['#F97316', '#1F2937'] },
  { name: 'Roxo e branco', primaryColor: '#7E22CE', secondaryColor: '#581C87', accentColor: '#C4B5FD', surfaceColor: '#FFFFFF', textColor: '#3B0764', kitColors: ['#7E22CE', '#FFFFFF'] },
  { name: 'Azul e vermelho', primaryColor: '#1E3A8A', secondaryColor: '#991B1B', accentColor: '#FACC15', surfaceColor: '#FFFFFF', textColor: '#172554', kitColors: ['#1E3A8A', '#DC2626'] },
  { name: 'Verde e amarelo', primaryColor: '#15803D', secondaryColor: '#14532D', accentColor: '#FACC15', surfaceColor: '#FFFDEB', textColor: '#14532D', kitColors: ['#15803D', '#FACC15'] },
  { name: 'Turquesa e marinho', primaryColor: '#14B8A6', secondaryColor: '#073B3A', accentColor: '#5EEAD4', surfaceColor: '#073B3A', textColor: '#ECFDF5', kitColors: ['#14B8A6', '#073B3A'] },
  { name: 'Cinzento e preto', primaryColor: '#4B5563', secondaryColor: '#111827', accentColor: '#D1D5DB', surfaceColor: '#F3F4F6', textColor: '#111827', kitColors: ['#D1D5DB', '#111827'] },
];
