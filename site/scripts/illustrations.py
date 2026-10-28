import sys, os
OUT = sys.argv[1]

def shade(hexc, f):
    h = hexc.lstrip('#'); r, g, b = (int(h[i:i+2], 16) for i in (0, 2, 4))
    return '#%02x%02x%02x' % tuple(max(0, min(255, int(c * f))) for c in (r, g, b))

def slab(cx, y, a, t, color, cls=''):
    b = a / 2
    top = f'M{cx},{y-b} L{cx+a},{y} L{cx},{y+b} L{cx-a},{y} Z'
    left = f'M{cx-a},{y} L{cx},{y+b} L{cx},{y+b+t} L{cx-a},{y+t} Z'
    right = f'M{cx},{y+b} L{cx+a},{y} L{cx+a},{y+t} L{cx},{y+b+t} Z'
    c = f' class="{cls}"' if cls else ''
    return (f'<g{c}><path d="{left}" fill="{shade(color,0.72)}"/><path d="{right}" fill="{shade(color,0.55)}"/>'
            f'<path d="{top}" fill="{color}"/><path d="{top}" fill="none" stroke="#ffffff" stroke-opacity=".35" stroke-width="1.2"/></g>')

def svg(w, h, body, label):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {w} {h}" role="img" aria-label="{label}">'
            f'<style>text{{font-family:Outfit,Inter,system-ui,sans-serif}}</style>{body}</svg>\n')

def write(name, content):
    with open(os.path.join(OUT, name), 'w') as f:
        f.write(content)

CORAL, AMBER, YELLOW, MINT, LIME, SKY, INK = '#ff9371', '#ffc06c', '#ffedba', '#95ffec', '#b6ff9f', '#c5eaff', '#1a1a1a'

layers = [
    ('Application', 'your code, slices per dependency', LIME),
    ('Launch helpers', 'memory · CA certs · JVM options', MINT),
    ('FIPS 140-3', 'BC-FJA 2.1.1 · BCFKS truststore', CORAL),
    ('Eclipse Temurin', 'JRE or JDK · Java 8 to 25', AMBER),
    ('Wolfi run image', 'glibc · no shell · non-root', SKY),
]
body = ''
cx, a, t, gap = 230, 170, 26, 78
y0 = 120
for i, (title, sub, color) in enumerate(reversed(layers)):
    y = y0 + (len(layers) - 1 - i) * gap
    body += slab(cx, y, a, t, color, f'layer layer-{len(layers)-1-i}')
for i, (title, sub, color) in enumerate(layers):
    y = y0 + i * gap
    body += (f'<g class="label label-{i}"><path d="M{cx+a+10},{y+t/2} H{cx+a+46}" stroke="{color}" stroke-width="2" stroke-dasharray="3 4"/>'
             f'<circle cx="{cx+a+50}" cy="{y+t/2}" r="4" fill="{color}"/>'
             f'<text x="{cx+a+64}" y="{y+t/2-2}" fill="#ffffff" font-size="22" font-weight="700">{title}</text>'
             f'<text x="{cx+a+64}" y="{y+t/2+20}" fill="#a6a6a6" font-size="15">{sub}</text></g>')
write('stack.svg', svg(720, 560, body, 'Layers of an image built by the Wolfi Java Buildpack'))

def lock(cx, cy, s, color):
    return (f'<g transform="translate({cx-s/2},{cy-s/2})"><path d="M{s*0.28},{s*0.46} V{s*0.32} a{s*0.22},{s*0.22} 0 0 1 {s*0.44},0 V{s*0.46}" fill="none" stroke="{color}" stroke-width="{s*0.09}" stroke-linecap="round"/>'
            f'<rect x="{s*0.16}" y="{s*0.44}" width="{s*0.68}" height="{s*0.5}" rx="{s*0.1}" fill="{color}"/>'
            f'<circle cx="{s*0.5}" cy="{s*0.66}" r="{s*0.07}" fill="{INK}"/><rect x="{s*0.47}" y="{s*0.68}" width="{s*0.06}" height="{s*0.14}" rx="{s*0.03}" fill="{INK}"/></g>')

body = slab(200, 250, 120, 22, SKY) + slab(200, 196, 120, 22, AMBER) + slab(200, 142, 120, 22, CORAL, 'pulse')
body += f'<circle cx="300" cy="96" r="58" fill="{INK}" stroke="{CORAL}" stroke-width="4"/>' + lock(300, 96, 70, CORAL)
body += f'<text x="200" y="340" text-anchor="middle" fill="#ffffff" font-size="20" font-weight="700">approved-only</text>'
write('fips.svg', svg(400, 360, body, 'FIPS 140-3 mode'))

bars = [('heap', 0.52, LIME), ('code cache', 0.14, AMBER), ('threads', 0.14, MINT), ('metaspace', 0.08, CORAL), ('headroom', 0.12, SKY)]
body = ''
y = 370
for name, frac, color in bars:
    t = max(10, frac * 240)
    body += slab(170, y - t, 120, t, color)
    y -= t + 6
y = 370
for name, frac, color in bars:
    t = max(10, frac * 240)
    body += f'<text x="306" y="{y - t/2 + 10}" fill="#ffffff" font-size="16" font-weight="600">{name}</text>'
    y -= t + 6
write('memory.svg', svg(420, 450, body, 'Memory budget of the JVM'))

def check(cx, cy, r, color):
    return (f'<circle cx="{cx}" cy="{cy}" r="{r}" fill="{color}"/>'
            f'<path d="M{cx-r*0.42},{cy+r*0.02} L{cx-r*0.1},{cy+r*0.34} L{cx+r*0.46},{cy-r*0.3}" fill="none" stroke="{INK}" stroke-width="{r*0.18}" stroke-linecap="round" stroke-linejoin="round"/>')

body = ''
for i, color in enumerate([SKY, AMBER, MINT]):
    x = 80 + i * 120
    body += slab(x, 200, 46, 14, color) + slab(x, 172, 46, 14, shade(color, 0.9)) + slab(x, 144, 46, 14, color)
    if i < 2:
        body += (f'<g stroke="#5a5a5a" stroke-width="5" fill="none"><rect x="{x+44}" y="186" width="34" height="16" rx="8"/>'
                 f'<rect x="{x+62}" y="186" width="34" height="16" rx="8"/></g>')
body += check(320, 84, 34, LIME)
body += f'<text x="200" y="290" text-anchor="middle" fill="#ffffff" font-size="20" font-weight="700">SLSA L3 · signed · SBOM</text>'
write('supply-chain.svg', svg(400, 320, body, 'Verified supply chain'))

steps = [('detect', SKY), ('build', AMBER), ('export', CORAL), ('launch', MINT)]
body = ''
for i, (name, color) in enumerate(steps):
    x = 70 + i * 130
    body += slab(x, 130, 48, 18, color, 'hop')
    body += f'<text x="{x}" y="210" text-anchor="middle" fill="#ffffff" font-size="18" font-weight="700">{name}</text>'
    if i < len(steps) - 1:
        body += f'<path d="M{x+58},140 H{x+70}" stroke="#5a5a5a" stroke-width="3" stroke-linecap="round"/><path d="M{x+66},134 L{x+74},140 L{x+66},146" fill="none" stroke="#5a5a5a" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/>'
write('lifecycle.svg', svg(560, 240, body, 'Buildpack lifecycle'))

body = f'<rect width="1200" height="630" fill="{INK}"/>'
body += f'<circle cx="960" cy="140" r="360" fill="{CORAL}" opacity=".10"/>'
for i, color in enumerate([SKY, AMBER, CORAL, MINT, LIME]):
    body += slab(900, 470 - i * 70, 170, 26, color)
body += (f'<g transform="translate(80,90)"><rect width="88" height="88" rx="22" fill="#262626"/>'
         f'<g transform="translate(12,12) scale(1)"><path d="M32 10 52 20.5 32 31 12 20.5Z" fill="{CORAL}"/><path d="M12 28.5 32 39l20-10.5v5.5L32 44.5 12 34Z" fill="{AMBER}"/><path d="M12 40.5 32 51l20-10.5V46L32 56.5 12 46Z" fill="{MINT}"/></g></g>')
body += f'<text x="80" y="290" fill="#ffffff" font-size="76" font-weight="800">Wolfi Java</text>'
body += f'<text x="80" y="370" fill="#ffffff" font-size="76" font-weight="800">Buildpack</text>'
body += f'<text x="80" y="440" fill="#cfcfcf" font-size="30">Small, signed, FIPS-ready Java images</text>'
body += f'<g font-size="22" font-weight="700"><rect x="80" y="490" width="150" height="44" rx="22" fill="{CORAL}"/><text x="155" y="519" text-anchor="middle" fill="{INK}">FIPS 140-3</text>'
body += f'<rect x="244" y="490" width="150" height="44" rx="22" fill="{MINT}"/><text x="319" y="519" text-anchor="middle" fill="{INK}">SLSA L3</text>'
body += f'<rect x="408" y="490" width="170" height="44" rx="22" fill="{YELLOW}"/><text x="493" y="519" text-anchor="middle" fill="{INK}">Java 8 to 25</text></g>'
write('og.svg', svg(1200, 630, body, 'Wolfi Java Buildpack'))
