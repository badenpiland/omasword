#!/usr/bin/env python3
"""Generate the omasword launcher icon: a pixel-art sword (pointing down,
like the TUI's "†" mark) on a Bible cover, in Omarchy's flat, blocky style.

    python3 assets/make-icon.py > assets/omasword.svg
    rsvg-convert -w 256 -h 256 assets/omasword.svg -o assets/omasword.png

Pass a hex color to recolor it (default: Tokyo Night blue, Omarchy's
default accent).
"""
import sys
N, cell = 25, 12   # 25x25 grid of 12px cells = 300x300

def grid():
    g = [[0]*N for _ in range(N)]
    def h(r, c0, c1):
        for c in range(c0, c1+1): g[r][c] = 1
    def v(c, r0, r1):
        for r in range(r0, r1+1): g[r][c] = 1

    # --- Bible: closed book seen from the front ---
    L, R, T, B = 2, 22, 1, 23      # outer edge
    h(T, L+1, R-1)                 # top (corners notched, Omarchy-style)
    v(L, T+1, B-1)                 # outer spine edge
    v(R, T+1, B-3)                 # right cover edge
    v(L+2, T+1, B-1)               # spine/binding line
    h(B-2, L+2, R)                 # bottom of the cover
    h(B, L+1, R-1)                 # page block bottom
    v(R, B-1, B-1)                 # page block right edge
    g[B-1][R-1] = 0
    h(B, L+1, R-1)

    # --- sword (pointing down, like the TUI's dagger mark) on the cover ---
    c = (L + 3 + R - 1) // 2       # centre of the cover area
    h(3, c-1, c+1)                 # pommel
    v(c, 4, 6)                     # grip
    g[7][c-5] = g[7][c+5] = 1      # quillon tips
    h(8, c-5, c+5)                 # crossguard
    for r in range(9, 18): h(r, c-1, c+1)   # blade
    v(c, 18, 19)                   # point
    return g

def svg(g, color):
    rects = []
    for y, row in enumerate(g):
        x = 0
        while x < N:
            if row[x]:
                s = x
                while x < N and row[x]: x += 1
                rects.append(f'<rect x="{s*cell}" y="{y*cell}" width="{(x-s)*cell}" height="{cell}"/>')
            else: x += 1
    return ('<svg xmlns="http://www.w3.org/2000/svg" width="300" height="300" viewBox="0 0 300 300">'
            f'<g fill="{color}" shape-rendering="crispEdges">' + "".join(rects) + "</g></svg>")

print(svg(grid(), sys.argv[1] if len(sys.argv) > 1 else "#7aa2f7"))
