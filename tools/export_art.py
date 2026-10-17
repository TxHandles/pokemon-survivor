#!/usr/bin/env python3
"""Exports the web version's hand-drawn canvas art into sprite atlases for the Go build.

The web game draws its scenery, landmarks, hazard pools and move art with canvas code (original
drawings). The Go build uses the same pictures: this renders each one in headless Chromium from
build/monster-survivors.html and packs them into data/art/*.png with an index in data/art/art.json.
It also writes the collision shapes and per-location scenery tables the simulation needs
(engine/sim/scenery_gen.go).

Run after tools/build.py whenever the web art changes:  python3 tools/export_art.py
Needs: playwright (Chromium) and Pillow.
"""
import base64, io, json, math, os, subprocess, sys, time
from PIL import Image
from playwright.sync_api import sync_playwright

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUT = os.path.join(ROOT, 'data', 'art')

JS = r"""
() => {
 const out = {types:{}, scen:{}, lm:{}, pools:{}, meta:{}};
 const png = c => c.toDataURL('image/png');
 // move art: ART[type][p|o|b|d] on 96px canvases
 for (const t of Object.keys(ART)) for (const v of ['p','o','b','d']) out.types[t+'/'+v] = png(asp(t, v));
 out.meta.artCols = Object.fromEntries(Object.keys(ART).map(t => [t, ART[t].c]));
 // scenery: per biome & location, tree / rock styles, three variants each (same seeds as scnSpr)
 const loc = {}, seen = {};
 for (const bi of Object.keys(SCB)) {
  loc[bi] = [];
  for (let li = 0; li < 3; li++) {
   const d = ((SCV[bi] || [])[li]) || SCB[bi] || SCB.normal, keys = {t: [], r: []};
   for (const kind of ['t', 'r']) for (let v = 0; v < 3; v++) {
    const seed = v*977 + bi.length*31 + (kind === 't' ? 7 : 3), style = kind === 't' ? d[0] : d[2], pal = kind === 't' ? d[1] : d[3];
    const id = kind + ':' + style + ':' + JSON.stringify(pal) + ':' + seed;
    if (!seen[id]) {
     const c = scnC(128, 128), x = c.getContext('2d'), R = srng(seed);
     try { if (kind === 't') (TREE[style] || TREE.oak)(x, R, pal); else (ROCK[style] || ROCK.boulder)(x, R, pal) } catch (e) {}
     seen[id] = 's' + Object.keys(seen).length; out.scen[seen[id]] = png(c);
    }
    keys[kind].push(seen[id]);
   }
   loc[bi].push({tree: d[0], rock: d[2], t: keys.t, r: keys.r});
  }
 }
 out.meta.loc = loc;
 out.meta.shp = SHP; out.meta.rscm = RSCM;
 // landmarks: two per location + the location sign (160px canvases anchored at 80,150)
 const lmLoc = {}, lseen = {};
 const lmOne = (kind, p) => { const id = kind + ':' + JSON.stringify(p || {}); if (!lseen[id]) { const c = scnC(160, 160), x = c.getContext('2d'); try { LMK[kind].d(x, p || {}) } catch (e) {} lseen[id] = 'l' + Object.keys(lseen).length; out.lm[lseen[id]] = png(c) } return lseen[id] };
 for (const bi of Object.keys(LMS)) lmLoc[bi] = LMS[bi].map(pair => pair.map(([kind, p]) => ({kind, img: lmOne(kind, p)})));
 out.meta.lmLoc = lmLoc; out.meta.sign = lmOne('sign', {});
 out.meta.lmk = Object.fromEntries(Object.keys(LMK).map(k => [k, {cs: LMK[k].cs || [], fx: LMK[k].fx || null}]));
 // hazard pools: 256px, three variants
 for (const k of ['lava', 'deep', 'ice', 'toxic', 'sand', 'web']) for (let v = 0; v < 3; v++) out.pools[k + '/' + v] = png(poolSpr(k, v));
 // ground palettes per biome & location (FLB merged with the FLV location variants), ambient life, location tints
 const floors = {};
 for (const bi of Object.keys(FLB)) { floors[bi] = []; for (let li = 0; li < 3; li++) { const v = (FLV[bi] || [])[li]; floors[bi].push(v ? Object.assign({}, FLB[bi], v) : FLB[bi]) } }
 out.floors = {floors, amb: AMB_K, lfx: LFX};
 return out;
}
"""


def img(u):
    return Image.open(io.BytesIO(base64.b64decode(u.split(',', 1)[1]))).convert('RGBA')


def atlas(name, items, cell, cols):
    """items: list of (key, dataURL); returns {key: [x, y, w, h]}"""
    keys = sorted(items)
    rows = math.ceil(len(keys) / cols)
    A = Image.new('RGBA', (cols * cell, rows * cell), (0, 0, 0, 0))
    idx = {}
    for i, k in enumerate(keys):
        x, y = (i % cols) * cell, (i // cols) * cell
        A.paste(img(items[k]), (x, y))
        idx[k] = [x, y, cell, cell]
    A.save(os.path.join(OUT, name + '.png'), optimize=True)
    return idx


def main():
    os.makedirs(OUT, exist_ok=True)
    srv = subprocess.Popen([sys.executable, '-m', 'http.server', '8793', '-d', os.path.join(ROOT, 'build')],
                           stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    time.sleep(1)
    try:
        with sync_playwright() as p:
            b = p.chromium.launch(args=['--no-sandbox'])
            pg = b.new_page()
            pg.goto('http://localhost:8793/monster-survivors.html')
            pg.wait_for_timeout(2500)
            d = pg.evaluate(JS)
            b.close()
    finally:
        srv.terminate()
    meta = d['meta']
    meta['atlas'] = {
        'types': atlas('types', d['types'], 96, 12),
        'scen': atlas('scenery', d['scen'], 128, 16),
        'lm': atlas('landmarks', d['lm'], 160, 12),
        'pools': atlas('pools', d['pools'], 256, 6),
    }
    with open(os.path.join(OUT, 'floors.json'), 'w') as f:
        json.dump(d['floors'], f, separators=(',', ':'), sort_keys=True)
    with open(os.path.join(OUT, 'art.json'), 'w') as f:
        json.dump(meta, f, separators=(',', ':'), sort_keys=True)
    write_sim(meta)
    print('wrote', OUT, {k: len(v) for k, v in meta['atlas'].items()})


def write_sim(m):
    """engine/sim/scenery_gen.go: per-location scenery styles and collision circles."""
    def circles(l):
        return '{' + ', '.join('{%g, %g, %g}' % tuple(c) for c in (l or [])) + '}'
    o = ['// Code generated by tools/export_art.py from the web scenery tables; DO NOT EDIT.', '', 'package sim', '']
    o.append('// locScenery[biome][location] = {tree style, rock style}')
    o.append('var locScenery = map[string][3][2]string{')
    for bi in sorted(m['loc']):
        o.append('\t%s: {%s},' % (json.dumps(bi), ', '.join('{%s, %s}' % (json.dumps(l['tree']), json.dumps(l['rock'])) for l in m['loc'][bi])))
    o.append('}\n')
    for kind, name in (('t', 'shpTree'), ('r', 'shpRock')):
        o.append('var %s = map[string][][3]float64{' % name)
        for k in sorted(m['shp'][kind]):
            if m['shp'][kind][k] is not None:
                o.append('\t%s: %s,' % (json.dumps(k), circles(m['shp'][kind][k])))
        o.append('}\n')
    o.append('var rockScale = map[string]float64{%s}\n' % ', '.join('%s: %g' % (json.dumps(k), v) for k, v in sorted(m['rscm'].items())))
    o.append('// landmark kinds of each location (two per location) and their collision circles (160px canvas, base 80,150)')
    o.append('var locLandmarks = map[string][3][2]string{')
    for bi in sorted(m['lmLoc']):
        o.append('\t%s: {%s},' % (json.dumps(bi), ', '.join('{%s, %s}' % (json.dumps(a['kind']), json.dumps(b['kind'])) for a, b in m['lmLoc'][bi])))
    o.append('}\n')
    o.append('var lmShapes = map[string][][3]float64{')
    for k in sorted(m['lmk']):
        o.append('\t%s: %s,' % (json.dumps(k), circles(m['lmk'][k]['cs'])))
    o.append('}')
    with open(os.path.join(ROOT, 'engine', 'sim', 'scenery_gen.go'), 'w') as f:
        f.write('\n'.join(o) + '\n')
    subprocess.run(['gofmt', '-w', os.path.join(ROOT, 'engine', 'sim', 'scenery_gen.go')])


if __name__ == '__main__':
    main()
