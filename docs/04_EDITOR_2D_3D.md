# Editor de exercícios 2D/3D — especificação

## Princípio
**Uma única cena e timeline, dois renderizadores.** O utilizador cria/edita em 2D e visualiza em 3D; edição 3D avançada só depois. Coordenadas normalizadas no plano do campo, escala adaptada à modalidade.

## Elementos
Jogadores (equipa, número, cor), bolas, cones, estacas, barreiras, balizas, zonas, linhas, setas, rótulos e trajetórias. Ações de adicionar, selecionar, mover, rodar, duplicar, agrupar, eliminar, desfazer/refazer, zoom, pan, snap, camadas e guardar automático.

## Timeline
Keyframes por objeto com `timeMs`, posição, rotação, visibilidade, animação/ação; interpolação determinística; passes, remates, deslocações simultâneas, pausa, velocidade, loop e etapas. Caminhos e setas podem aparecer/desaparecer ao longo do tempo. Validar tempos e colisões visuais básicas.

## Renderizadores
2D: React Konva ou Canvas/SVG. 3D: Three.js + React Three Fiber, modelos GLB leves e animações básicas. Para desempenho mobile: LOD, instancing, texturas compactas e fallback 2D.

## Modelo JSON ilustrativo (não contrato definitivo)
```json
{
  "schemaVersion": 1,
  "sportVariant": "futsal",
  "field": {"width": 40, "height": 20},
  "objects": [{"id":"p7","type":"player","team":"blue","number":7,"x":0.3,"y":0.5}],
  "tracks": [{"objectId":"p7","keyframes":[{"timeMs":0,"x":0.3,"y":0.5},{"timeMs":2000,"x":0.6,"y":0.5}]}],
  "durationMs": 2000
}
```

## Modalidades
Futebol/futsal: linhas e balizas, passes, pressão e rotações. Voleibol: rede, seis posições, rotações, serviço e bloco. Basquetebol: cestos, drible, lançamento e marcações. Andebol: balizas, área, passes, remates e bloqueios. Os movimentos são representações táticas, não simulação física regulamentar.

## Aceitação do protótipo
Criar exercício futsal com 5 jogadores, 1 bola e 4 cones; desenhar trajetórias; reproduzir; alternar 2D↔3D no mesmo instante sem divergência; guardar; recarregar; reproduzir com resultados consistentes em desktop e mobile recente.

## Exportações
Primeiro: imagem PNG/SVG 2D e PDF de sessão. Depois: vídeo WebM/MP4 quando houver pipeline e compatibilidade testados. Licenciar modelos, fontes e texturas antes de distribuir.
