"""Build the game from content data.

    python3 tools/build.py            -> build/monster-survivors.html  (single-file game)
                                       then index.html for the website (Firebase, co-op, friends)

Edit the files in data/ (see CONTENT.md); never edit build/ or index.html by hand."""
import json, os, base64, sys, subprocess
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DD = os.path.join(ROOT, 'data')
def J(n): return json.load(open(os.path.join(DD, n)))
def fail(msg): raise SystemExit('DATA ERROR: ' + msg)
pk = [p for p in J('pokemon.json')['pokemon'] if not p.get('px')];  # pixel-only Pokémon are Go-build only
mv = J('moves.json')['moves']; ls = J('learnsets.json')['learnsets']
ab = J('abilities.json')['abilities']; bal = J('balance.json'); world = J('world.json'); spr = J('sprites/sprites.json')['sprites']
TYPES = ['normal', 'fire', 'water', 'electric', 'grass', 'ice', 'fighting', 'poison', 'ground', 'flying', 'psychic', 'bug', 'rock', 'ghost', 'dragon', 'dark', 'steel', 'fairy']
ids = [p['id'] for p in pk]
if len(set(ids)) != len(ids): fail('duplicate Pokemon id')
mvids = {m['id'] for m in mv}
if len(mvids) != len(mv): fail('duplicate move id')
D = {'starters': [], 'chr': {}, 'cst': {}, 'dexnum': {}, 'sprK': {}, 'bty': {}, 'abasg': {}}
for p in pk:
    i = p['id']
    for t in p['types']:
        if t not in TYPES: fail(f'{i}: unknown type {t}')
    if 'baseStats' in p:
        if i not in TYPES: fail(f'{i}: a Pokemon with baseStats must be the base form of a type (id = type name)')
        D['cst'][i] = p['baseStats']; D['starters'].append([i, p['name'], p.get('tickets', 0)])
        if p['types'] != [i]: D['bty'][i] = p['types']
    else:
        u = p['unlock']; c = {'n': p['name'], 't': p['types'][0], 'ty': p['types'], 'base': p['statsFrom'], 'm': p['statMult'], 'tag': p['tag'], 'req': p['unlockText'], 'unlock': u}
        for a, b in (('evolvesFrom', 'from'), ('regionalOf', 'alt'), ('form', 'form'), ('enemyPool', 'pool'), ('dexNote', 'sh')):
            if a in p: c[b] = p[a]
        if p.get('legendary'): c['leg'] = 1
        if u.get('tickets'): c['tk'] = u['tickets']
        for x in ('evolvesFrom', 'regionalOf'):
            if x in p and p[x] not in ids: fail(f'{i}: {x} "{p[x]}" is not a Pokemon id')
        D['chr'][i] = c; D['starters'].append([i, p['name'], u.get('tickets', 0)])
    D['dexnum'][i] = p['dex']
    if 'spriteScale' in p: D['sprK'][i] = p['spriteScale']
    if 'abilities' in p: D['abasg'][i] = [p['abilities']['starting'], p['abilities']['hidden']]
    if i not in spr: fail(f'{i}: no sprites in data/sprites/sprites.json')
TIER = {'basic': ('B', 1.0, 1.0), 'evolved': ('E', 1.35, 1.08), 'final': ('F', 1.75, 1.2)}
D['nmv'] = [{'id': m['id'], 'n': m['name'], 't': m['type'], 'k': m['shape'], 'tier': TIER[m['tier']][0], 'd': m['desc'], 'q': m['params'], 'sy': m['heldItem'], 'ic': m['icon'],
             'dm': TIER[m['tier']][1], 'cdm': TIER[m['tier']][2]} for m in mv]
D['lrn'] = {}
for k, v in ls.items():
    for x in [v['start'], *v['moves'], *[y for l in v.get('secondType', {}).values() for y in l]]:
        if x not in mvids: fail(f'learnset {k}: unknown move {x}')
    D['lrn'][k] = {'s': v['start'], 'm': v['moves'], **({'x': v['secondType']} if v.get('secondType') else {})}
D['abil'] = ab; D['tbal'] = bal['typeStats']; D['bal'] = bal['moveDamage']; D['bale'] = bal['moveDamageEvolved']
D['locs'] = world['locations']
D['musicLoc'] = {k: v['file'] for k, v in world['music'].items()}
D['musicLoops'] = {v['file']: [v['loopStartSample'] / 44100, v['loopEndSample'] / 44100] for v in world['music'].values()}
def uri(f):
    b = open(os.path.join(DD, 'sprites', f), 'rb').read(); mt = 'image/png' if f.endswith('.png') else 'image/gif'
    return f'data:{mt};base64,' + base64.b64encode(b).decode()
D['sprites'] = {}; D['images'] = {}
for i, m in spr.items():
    o = {}
    for k, v in m.items():
        if k == 'portrait': D['images'][i] = uri(v); continue
        if isinstance(v, dict) and 'file' in v:
            vv = {kk: x for kk, x in v.items() if kk != 'file'}; o[k] = {'src': uri(v['file']), **vv}
        else: o[k] = v
    D['sprites'][i] = o
tpl = open(os.path.join(ROOT, 'src', 'game.html')).read()
if tpl.count('/*@@DATA@@*/null') != 1: fail('template marker missing')
out = tpl.replace('/*@@DATA@@*/null', json.dumps(D, ensure_ascii=False, separators=(',', ':')))
os.makedirs(os.path.join(ROOT, 'build'), exist_ok=True)
game = os.path.join(ROOT, 'build', 'monster-survivors.html'); open(game, 'w').write(out)
print('built', game, round(len(out) / 1e6, 2), 'MB ·', len(pk), 'Pokemon ·', len(mv), 'moves')
if '--no-web' not in sys.argv:
    subprocess.run([sys.executable, os.path.join(ROOT, 'src', 'web_build.py'), game, os.path.join(ROOT, 'index.html')], check=True)
