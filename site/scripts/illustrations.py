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


def box(x, y, w, h, fill, text, sub='', tc=INK, size=17):
    out = f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="{min(h/2, 16)}" fill="{fill}"/>'
    if sub:
        out += f'<text x="{x+w/2}" y="{y+h/2-3}" text-anchor="middle" fill="{tc}" font-size="{size}" font-weight="700">{text}</text>'
        out += f'<text x="{x+w/2}" y="{y+h/2+16}" text-anchor="middle" fill="{tc}" fill-opacity=".72" font-size="13">{sub}</text>'
    else:
        out += f'<text x="{x+w/2}" y="{y+h/2+6}" text-anchor="middle" fill="{tc}" font-size="{size}" font-weight="700">{text}</text>'
    return out


def arrow(x1, y1, x2, y2, color='#6a6a6a'):
    import math
    ang = math.atan2(y2 - y1, x2 - x1)
    ax, ay = x2 - 9 * math.cos(ang - 0.5), y2 - 9 * math.sin(ang - 0.5)
    bx, by = x2 - 9 * math.cos(ang + 0.5), y2 - 9 * math.sin(ang + 0.5)
    return (f'<path d="M{x1},{y1} L{x2},{y2}" stroke="{color}" stroke-width="2.5" stroke-linecap="round"/>'
            f'<path d="M{ax:.1f},{ay:.1f} L{x2},{y2} L{bx:.1f},{by:.1f}" fill="none" stroke="{color}" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"/>')


steps = [
    ('BP_JVM_VERSION', 'explicit, must be supported', CORAL),
    ('.java-version · .sdkmanrc · .tool-versions', 'version managers', AMBER),
    ('pom.xml', 'release · java.version · target · source', YELLOW),
    ('build.gradle(.kts)', 'toolchain · sourceCompatibility', MINT),
    ('META-INF/MANIFEST.MF', 'Build-Jdk-Spec of a prebuilt JAR', SKY),
    ('default 21', 'when nothing else matches', '#3a3a3a'),
]
body = ''
for i, (title, sub, color) in enumerate(steps):
    y = 20 + i * 70
    tc = '#ffffff' if color == '#3a3a3a' else INK
    body += f'<circle cx="40" cy="{y+27}" r="20" fill="none" stroke="{color if color != "#3a3a3a" else "#6a6a6a"}" stroke-width="2.5"/>'
    body += f'<text x="40" y="{y+33}" text-anchor="middle" fill="#ffffff" font-size="17" font-weight="800">{i+1}</text>'
    body += box(78, y, 520, 54, color, title, sub, tc, 16)
    if i < len(steps) - 1:
        body += f'<path d="M40,{y+49} V{y+75}" stroke="#4a4a4a" stroke-width="2" stroke-dasharray="3 4"/>'
body += f'<text x="620" y="60" fill="#a6a6a6" font-size="14">first match wins</text>'
body += f'<text x="620" y="80" fill="#a6a6a6" font-size="14">hints round up to</text>'
body += f'<text x="620" y="100" fill="#a6a6a6" font-size="14">the next LTS</text>'
write('version-resolution.svg', svg(760, 450, body, 'Java version resolution order'))

left = [('SUN', '#3a3a3a'), ('SunRsaSign', '#3a3a3a'), ('SunEC', '#3a3a3a'), ('SunJSSE', '#3a3a3a'), ('SunJCE', '#3a3a3a'), ('SunJGSS · SunSASL · …', '#3a3a3a'), ('SunPKCS11', '#3a3a3a')]
right = [('1  BCFIPS', CORAL, 'approved-only'), ('2  BCJSSE', AMBER, 'TLS 1.2 / 1.3'), ('3  SUN', SKY, 'X.509 · JKS · non-crypto')]
body = f'<text x="150" y="30" text-anchor="middle" fill="#a6a6a6" font-size="15" font-weight="700">JDK DEFAULT</text>'
body += f'<text x="560" y="30" text-anchor="middle" fill="{CORAL}" font-size="15" font-weight="700">FIPS MODE</text>'
for i, (name, color) in enumerate(left):
    y = 48 + i * 46
    body += box(40, y, 220, 36, color, name, '', '#d0d0d0', 15)
    if name not in ('SUN',):
        body += f'<path d="M58,{y+18} H242" stroke="{CORAL}" stroke-width="2" stroke-opacity=".7"/>'
body += arrow(285, 205, 405, 205, CORAL)
for i, (name, color, sub) in enumerate(right):
    y = 90 + i * 76
    body += box(420, y, 280, 60, color, name, sub, INK, 18)
write('providers.svg', svg(740, 380, body, 'Security providers in FIPS mode'))

body = box(20, 40, 210, 70, SKY, 'cacerts', 'JDK trust anchors') + box(20, 150, 210, 70, MINT, 'ca-certificates', 'bindings · build time')
body += arrow(236, 75, 300, 118) + arrow(236, 185, 300, 140)
body += box(306, 95, 170, 70, '#3a3a3a', 'keytool', '+ BCFIPS provider', '#ffffff')
body += arrow(482, 130, 540, 130)
body += box(546, 95, 200, 70, CORAL, 'truststore.bcfks', 'fips layer · read-only')
body += box(546, 220, 200, 70, AMBER, '/tmp copy', 'launch-time bindings')
body += f'<path d="M646,171 V214" stroke="#6a6a6a" stroke-width="2.5" stroke-dasharray="4 5"/>'
body += f'<text x="770" y="262" fill="#a6a6a6" font-size="13">only when</text><text x="770" y="280" fill="#a6a6a6" font-size="13">certs are added</text>'
write('truststore.svg', svg(880, 310, body, 'How the BCFKS truststore is assembled'))

seq = [('launcher', '#3a3a3a', '#ffffff', 'CNB'), ('0-ca-certificates', MINT, INK, 'bindings → truststore'), ('1-memory-calculator', AMBER, INK, '-Xmx · -Xss · metaspace'), ('2-jvm-options', CORAL, INK, 'JMX · JDWP · JFR · NMT'), ('java', LIME, INK, 'your process')]
body = ''
for i, (name, color, tc, sub) in enumerate(seq):
    y = 20 + i * 82
    body += box(150, y, 340, 60, color, name, sub, tc, 17)
    if i < len(seq) - 1:
        body += arrow(320, y + 62, 320, y + 80)
body += f'<text x="20" y="128" fill="#a6a6a6" font-size="13">exec.d</text><text x="20" y="146" fill="#a6a6a6" font-size="13">static Go</text><text x="20" y="164" fill="#a6a6a6" font-size="13">no shell</text>'
body += f'<path d="M110,104 V266" stroke="#4a4a4a" stroke-width="2"/><path d="M110,104 H140 M110,266 H140" stroke="#4a4a4a" stroke-width="2"/>'
body += f'<text x="510" y="190" fill="#a6a6a6" font-size="13">each helper appends</text><text x="510" y="208" fill="#a6a6a6" font-size="13">to JAVA_TOOL_OPTIONS</text>'
write('launch.svg', svg(680, 430, body, 'Launch sequence'))

stages = [('git tag v*', '#3a3a3a', '#ffffff'), ('CI + 10 scenarios', SKY, INK), ('stacks · buildpacks', AMBER, INK), ('builder', YELLOW, INK), ('SLSA L3 provenance', LIME, INK), ('cosign · SBOM · scan', MINT, INK), ('GitHub release', CORAL, INK), ('docs', '#ffffff', INK)]
body = ''
for i, (name, color, tc) in enumerate(stages):
    col, row = i % 4, i // 4
    x = 20 + col * 215
    y = 30 + row * 130
    if row == 1:
        x = 20 + (3 - col) * 215
    body += box(x, y, 190, 62, color, name, '', tc, 16)
    if i < len(stages) - 1:
        if col < 3:
            if row == 0:
                body += arrow(x + 194, y + 31, x + 211, y + 31)
            else:
                body += arrow(x - 4, y + 31, x - 21, y + 31)
        else:
            body += arrow(x + 95, y + 66, x + 95, y + 124)
write('release-pipeline.svg', svg(880, 240, body, 'Release pipeline'))
