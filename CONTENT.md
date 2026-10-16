# Adding content

All game content lives in `data/`. The engine (`src/game.html`) reads it at build time, so adding a Pokémon, move,
ability or music track means editing data files — not game code.

```
python3 tools/build.py        # data/ + src/ -> build/monster-survivors.html and index.html
```

The build checks the data and stops with a `DATA ERROR:` message if something doesn't line up (unknown type,
missing sprites, a learnset naming a move that doesn't exist, an evolution pointing at a missing Pokémon…).

## Layout

| Path | What it is |
|---|---|
| `data/pokemon.json` | Every playable Pokémon, in roster order |
| `data/learnsets.json` | Moves each evolution line can learn |
| `data/moves.json` | All 345 moves |
| `data/abilities.json` | Abilities per type (3 starting, 3 hidden, 1 legendary) |
| `data/balance.json` | Numbers from the bot balance sweeps (type stat multipliers, per-move damage multipliers) |
| `data/world.json` | Location names (3 per biome) and location music with loop points |
| `data/sprites/` | Portrait (`<id>.gif`) and animation sheets (`<id>_front.png`, `<id>_back.png`) + `sprites.json` frame data |
| `music/` | Location music (MP3), loaded on demand |
| `src/game.html` | The game engine (template) |
| `src/net.js`, `src/friends.js`, `src/web_build.py` | Online co-op, friends, Firebase website build |
| `tools/build.py` | The build |

## Add a Pokémon

1. **Sprites** — drop `<id>.gif` (portrait) and `<id>_front.png` / `<id>_back.png` (animation sheets) into
   `data/sprites/` and add an entry to `sprites.json` (frame size, offsets, idle/attack frame lists and durations).
   The sheets are generated from the front/back GIFs you supply; the existing entries show the format.
2. **pokemon.json** — add an entry. An evolution:

```json
{
 "id": "growl2", "name": "Arcanine", "dex": 59, "types": ["fire"],
 "evolvesFrom": "growl", "statsFrom": "fire", "statMult": 1.07,
 "unlock": {"bosses": 5, "counter": "growlb", "as": ["growl"]},
 "unlockText": "Defeat 5 bosses as Growlithe",
 "enemyPool": ["boss"], "tag": "Evolution 2 · +7% stats",
 "dexNote": "✨ Beat 5 bosses<br>as Growlithe", "spriteScale": 0.6
}
```

   A new first form bought with tickets uses `"unlock": {"tickets": 2}` and needs `"abilities"` (below).

3. **learnsets.json** — first forms of a new line get a learnset (13 moves, or 10 + 10 for dual types):

```json
"growl": {"start": "fire_firefang", "moves": ["fire_firefang", "fire_ember", "..."]}
```
   Dual-type lines add `"secondType": {"flying": ["flying_gust", "..."]}`; those moves appear only on forms that
   actually have that type. Lines without a learnset fall back to every move of their type.

### Pokémon fields

| Field | Meaning |
|---|---|
| `id` | Internal id (never rename — saves use it). Base forms of a type use the type name (`fire` = Charmander) |
| `name`, `dex` | Display name and National Dex number |
| `types` | One or two types; the first is the primary type (abilities, stats) |
| `evolvesFrom` / `regionalOf` + `form` | Evolution parent / regional form of another Pokémon (`"form": "Alolan"`) |
| `statsFrom`, `statMult` | Stats copied from this base type, times this multiplier (1.07 = +7%) |
| `baseStats` | Only for type base forms: `{hp, atk, def, spd}` multipliers |
| `tickets` | Only for type base forms: ticket price (0 = free) |
| `unlock`, `unlockText` | Unlock rule (below) and the text shown in the Dex |
| `enemyPool` | Where it appears as an enemy: `reg`, `mini`, `boss` |
| `tag`, `dexNote` | Subtitle in menus; highlight note in the Dex |
| `abilities` | First forms only: `{"starting": 0-2, "hidden": 0-2}` — index into its primary type's abilities |
| `spriteScale` | Size multiplier for big evolutions |
| `legendary` | `true` for legendaries |

### Unlock rules

| Rule | Example |
|---|---|
| Ticket price | `{"tickets": 2}` |
| Mastery rank of a line | `{"mastery": 25, "line": "poli"}` |
| Beat N bosses | `{"bosses": 5, "counter": "growlb", "as": ["growl"]}` |
| Beat one boss | `{"boss": true, "flag": "avul", "as": ["vul"], "biome": "ice"}` |
| Clear a biome tier | `{"clearTier": 1, "biome": "fire"}` |
| Several at once | `{"all": [ {...}, {...} ]}` |

Boss rules can be narrowed with `as` (playing as these Pokémon), `asLine` (any Pokémon of that line),
`bossType` (the boss's type) and `biome` (where it was beaten). `counter` / `flag` is the save-file key that stores
progress — pick a new unique name for each new rule and never reuse or rename an existing one.
`doneFlag` (optional) is set when the rule completes.

## Add a move

Add it to `moves.json`, then put its id in the learnsets that should have it.

| Field | Meaning |
|---|---|
| `id` | `type_name`, e.g. `fire_flamethrower` (never rename) |
| `tier` | `basic` (any stage), `evolved` (stage 2+), `final` (fully evolved / legendary) |
| `shape` | Attack archetype: `proj nova arc beam dash strike erupt homing boom whirl aura wave screen clone jab burst twin sludge cone sweep chain spin mine rupture orbit trail` |
| `desc`, `icon` | Card text and HUD emoji |
| `heldItem` | Held item that evolves it at Lv 10 |
| `params` | Overrides of the shape's defaults: `d`/`dp` damage (+per level), `cd` cooldown, `n0` count, `r` radius, `st` status `[kind, chance, seconds]` (`bn` burn, `ps` poison, `su` stun, `fz` freeze, `sl` slow, `cf` confuse, `fe` fear, `rt` root), `cr` extra crit, `ls`/`lsx` lifesteal, `hpc` HP cost, `pct` % HP damage, `vps`/`vst`/`vlow` bonus vs poisoned/statused/low-HP foes, `coin` coin drops, `sk` wave status, `dmg` wave deals damage |

Dash moves are fired by the player (Shift / RB / R1). New moves start untuned — the bot benchmark sets their
`moveDamage` multipliers in `balance.json`.

## Add an ability

Abilities are per type in `abilities.json` (`t1` starting, `t2` hidden, `leg` legendary). Each ability is
`{n, d, p}`; `p` holds effect parameters read by the engine (see existing entries). New first-form Pokémon pick
one starting and one hidden ability by index.

## Add location music

Put the MP3 in `music/` and add it to `world.json`:

```json
"music": { "fire:1": {"file": "mt_ember.mp3", "loopStartSample": 2454097, "loopEndSample": 5135776} }
```

The key is `biome:locationIndex` (index into that biome's three `locations`). Loop points are sample positions at
44.1 kHz; the intro plays once, then the game loops between them. Seamless loop points are found by analysing the
track.
