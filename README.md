# Pokemon Survivor

A Vampire-Survivors-style roguelite. Play at https://txhandles.github.io/pokemon-survivor/

- `index.html`: the built game the website serves (generated — don't edit by hand)
- `data/`: all content (Pokémon, moves, learnsets, abilities, balance, locations, music, sprites) — see [CONTENT.md](CONTENT.md)
- `src/`: game engine template, online co-op, friends and website build
- `tools/build.py`: builds `index.html` (and a standalone `build/monster-survivors.html`) from `data/` + `src/`
- `music/`: location music
- `firestore.rules`: Firebase Firestore security rules (paste into Firebase console → Firestore → Rules)

## Go version (in progress)

A native rebuild in Go with [Ebitengine](https://ebitengine.org), reading the same `data/` folder — content added to
`data/` shows up in both versions. Builds a Windows `.exe` and a browser (WebAssembly) version from one codebase.

| Path | What it is |
|---|---|
| `engine/content` | Loads `data/*.json` (stats, learnsets, tier rules) |
| `engine/sim` | The game simulation — pure logic, no graphics, so bots can run it headless |
| `engine/meta` | Progression: save file (same format as the web game), shop, unlocks, mastery, challenges, rewards |
| `engine/synth` | Procedural sound effects and the placeholder music loop (same recipes as the web game) |
| `engine/client` | Rendering, input (keyboard, gamepad, mouse, touch), audio, HUD and menus |
| `cmd/game` | The playable game (Windows / browser) |
| `cmd/sim` | Headless bot runner for balance checks: `go run ./cmd/sim -runs 20 -mins 5` |
| `web/index.html` | Page that loads the browser build |
| `music/` | Location and menu tracks: embedded in the Windows build, fetched by the browser build |

```
./tools/build_go.sh            # -> build/windows/PokemonSurvivor.exe and build/web/
go test ./engine/content ./engine/sim
```

Saves: the Windows build keeps its save in `%APPDATA%\PokemonSurvivor\save.json`; the browser build uses the same
browser storage key as the web game. Main menu → Save transfer copies / loads the same save codes as the web game's
"Copy save code" / "Load save code", so progress moves both ways.

The browser build accepts `?demo=<id>&biome=<type>&seed=<n>&t=<seconds>` to watch the bot play.
GitHub Actions builds both on every push (Actions → Go build → artifacts); pushing a `go-v*` tag makes a release.
