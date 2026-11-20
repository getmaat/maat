"""Generates the Ma'at logo as SVG: .github/assets/maat-logo.svg (for dark
backgrounds) and maat-logo-light.svg (for light backgrounds).

    python3 lib/logo-svg.py <DM Sans variable font (.ttf or .woff2)> <out-dir>

The icon is drawn from primitives in a 550x280 coordinate space traced from
the original raster logo; the wordmark uses DM Sans (OFL) glyph outlines, so
the SVG needs no font. Requires fontTools (and brotli for .woff2).
"""
import sys

from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.ttLib import TTFont
from fontTools.varLib import instancer

FONT, OUT = sys.argv[1], sys.argv[2]

THEMES = {
    # name: line art + wordmark, feather, gold, pink, teal pan, background
    'maat-logo.svg': dict(line='#02cdb8', feather='#09c5af', gold='#f2b33d', apos='#ffaf24',
                          pink='#ec4d9a', pan='#2ad6bd', trail='#02cdb8', bg=None),
    'maat-logo-light.svg': dict(line='#152746', feather='#09c5b3', gold='#e3a13a', apos='#f29f1d',
                                pink='#d6338a', pan='#0bb5a5', trail='#0bb5a5', bg=None),
}

# --- wordmark --------------------------------------------------------------- #
# Measured in the original: cap height 105..189 (baseline 189.5); letter left
# edges M 220, a 319, a 398, t 460; apostrophe spans x 384..401, y 104..128.
WEIGHT = 500
font = instancer.instantiateVariableFont(TTFont(FONT), {'wght': WEIGHT, 'opsz': 14}, inplace=False)
glyphs = font.getGlyphSet()
cmap = font.getBestCmap()
cap = font['OS/2'].sCapHeight
BASELINE, CAP_TOP = 189.5, 105.0
scale = (BASELINE - CAP_TOP) / cap


def glyph_path(char, left):
    name = cmap[ord(char)]
    g = glyphs[name]
    # bbox of the glyph in font units, to align its left edge with the original
    from fontTools.pens.boundsPen import BoundsPen
    bp = BoundsPen(glyphs)
    g.draw(bp)
    xmin = bp.bounds[0]
    pen = SVGPathPen(glyphs)
    # font units -> logo space: flip y, scale, translate so xmin lands on `left`
    tp = TransformPen(pen, (scale, 0, 0, -scale, left - xmin * scale, BASELINE))
    g.draw(tp)
    return pen.getCommands()


LETTERS = [('M', 220), ('a', 319), ('a', 398), ('t', 460)]
wordmark = ' '.join(glyph_path(c, x) for c, x in LETTERS)
APOSTROPHE = 'M392.6 104 L401.6 104 L389.2 128.4 L384 128.4 Z'


# --- icon ------------------------------------------------------------------- #
def icon(t):
    L, R, T, B = 40.5, 182.5, 41.5, 238.5   # outline centre lines (4px stroke)
    r = 9                                    # corner radius
    FX, FY = 141.0, 84.5                     # fold: top edge ends at FX, right edge starts at FY
    outline = (f'M{L + r} {T} H{FX} L{R} {FY} V{B - r} Q{R} {B} {R - r} {B} '
               f'H{L + r} Q{L} {B} {L} {B - r} V{T + r} Q{L} {T} {L + r} {T} Z')
    fold = f'M{FX + 2.5} {T + 3} V{FY - 4} Q{FX + 2.5} {FY + 1.5} {FX + 8} {FY + 1.5} H{R}'

    # fold texture: a triangle of small dots tucked into the dog-ear
    dots = []
    for row in range(6):
        y = 57 + row * 4.4
        for col in range(row + 1):
            dots.append(f'<circle cx="{150 + col * 4.4:.1f}" cy="{y:.1f}" r="0.95"/>')

    # feather: a full, slightly asymmetric vane around a teal shaft (x=110),
    # flanked by two thin dark gaps, with short curved barb notches cut in
    # from both edges about every 10px (measured from the original).
    feather = ('M113.4 58.5 C119.5 66 126.2 82 128.2 101 C130 119 128.6 140 124.6 158 '
               'C121.2 172 116.5 183 111.2 191.5 L109.4 191.5 '
               'C103 182 96.6 170 93.6 155 C90.4 139 90.6 118 93.4 102 '
               'C96.4 84 104 69 113.4 58.5 Z')
    slits = []
    for y in (82, 93, 104, 115, 126, 137, 148, 159, 170):
        # left notch: from the outer edge, curving down toward the shaft
        slits.append(f'M88 {y - 1} Q96.5 {y + 1.5} {101.8} {y + 6.5}')
        # right notch: mirrored, a little higher, like the original
        slits.append(f'M132 {y - 4} Q123.5 {y - 1.5} {118.2} {y + 3.5}')
    slit_paths = ''.join(f'<path d="{d}"/>' for d in slits)

    def pan(cx, color, dashed):
        # bowl: flat rim at y=172, half-ellipse down to y=188, filled with stripes
        x0, x1 = cx - 18.5, cx + 18.5
        clip_id = f'bowl{int(cx)}'
        rows = (173, 176.8, 180.6, 184.4, 188)
        offsets = (0, 3.1, 1.4, 4.6, 2.3)      # stagger dashes so rows read like lines of text
        stripes = ''.join(
            f'<line x1="{x0 - 2}" y1="{y}" x2="{x1 + 2}" y2="{y}" stroke-width="1.7"'
            + (f' stroke-dasharray="5.6 1.9 3.2 1.9" stroke-dashoffset="{o}"' if dashed else '') + '/>'
            for y, o in zip(rows, offsets))
        return (f'<clipPath id="{clip_id}"><path d="M{x0} 171.6 H{x1} A18.5 17.4 0 0 1 {x0} 171.6 Z"/></clipPath>'
                f'<g stroke="{color}" clip-path="url(#{clip_id})">{stripes}</g>'
                f'<path d="M{x0} 171.6 H{x1} A18.5 17.4 0 0 1 {x0} 171.6 Z" fill="none" stroke="{color}" stroke-width="1.6"/>')

    def scale_side(px, pan_cx, beam_to):
        return (f'<line x1="{px}" y1="123.8" x2="{beam_to}" y2="123.8" stroke="{t["gold"]}" stroke-width="2.4" stroke-linecap="round"/>'
                f'<path d="M{pan_cx - 18.5} 171.6 L{px} 124 L{pan_cx + 18.5} 171.6" fill="none" stroke="{t["gold"]}" '
                f'stroke-width="1.6" stroke-linejoin="round"/>'
                f'<circle cx="{px}" cy="123.8" r="3.9" fill="{t["gold"]}"/>')

    prompt = (f'<path d="M59.6 204.5 L69.4 213.2 L59.6 221.9" fill="none" stroke="{t["line"]}" stroke-width="2.6" '
              f'stroke-linecap="round" stroke-linejoin="round"/>'
              f'<line x1="75" y1="221.4" x2="84.6" y2="221.4" stroke="{t["line"]}" stroke-width="2.6" stroke-linecap="round"/>')
    trail = ''.join(f'<circle cx="{x}" cy="220.6" r="1.25"/>' for x in (129.8, 137.1, 144.5, 151.8, 159.1, 166.4))

    return f'''
  <defs>
    <mask id="feather-cuts" maskUnits="userSpaceOnUse" x="80" y="50" width="60" height="150">
      <rect x="80" y="50" width="60" height="150" fill="#fff"/>
      <g stroke="#000" stroke-width="1.7" stroke-linecap="round" fill="none">{slit_paths}</g>
      <g stroke="#000" fill="none">
        <line x1="108.3" y1="76" x2="108.3" y2="189" stroke-width="1.8"/>
        <line x1="111.9" y1="76" x2="111.9" y2="189" stroke-width="1.1"/>
      </g>
    </mask>
  </defs>
  <path d="{outline}" fill="none" stroke="{t["line"]}" stroke-width="4" stroke-linejoin="round"/>
  <path d="{fold}" fill="none" stroke="{t["line"]}" stroke-width="3" stroke-linejoin="round" stroke-linecap="round"/>
  <g fill="{t["pink"]}">{''.join(dots)}</g>
  <line x1="110" y1="96" x2="110" y2="237" stroke="{t["line"]}" stroke-width="2.6"/>
  <path d="{feather}" fill="{t["feather"]}" mask="url(#feather-cuts)"/>
  {scale_side(67.6, 68.5, 88)}
  {scale_side(154.6, 154, 132)}
  {pan(68.5, t["pink"], False)}
  {pan(154, t["pan"], True)}
  {prompt}
  <g fill="{t["trail"]}">{trail}</g>'''


for filename, t in THEMES.items():
    bg = f'<rect width="550" height="280" fill="{t["bg"]}"/>' if t['bg'] else ''
    svg = f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 550 280" width="550" height="280" role="img" aria-label="Ma'at">
  <title>Ma'at</title>{bg}{icon(t)}
  <path d="{wordmark}" fill="{t["line"]}"/>
  <path d="{APOSTROPHE}" fill="{t["apos"]}"/>
</svg>
'''
    open(f'{OUT}/{filename}', 'w').write(svg)
    print('wrote', filename, len(svg) // 1024, 'KB')
