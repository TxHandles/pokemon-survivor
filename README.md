# Pokemon Survivor

A Vampire-Survivors-style roguelite. Play at https://txhandles.github.io/pokemon-survivor/

- `index.html`: the built game the website serves (generated — don't edit by hand)
- `data/`: all content (Pokémon, moves, learnsets, abilities, balance, locations, music, sprites) — see [CONTENT.md](CONTENT.md)
- `src/`: game engine template, online co-op, friends and website build
- `tools/build.py`: builds `index.html` (and a standalone `build/monster-survivors.html`) from `data/` + `src/`
- `music/`: location music
- `firestore.rules`: Firebase Firestore security rules (paste into Firebase console → Firestore → Rules)

## Go version (in progress)

**Play it:** browser → https://txhandles.github.io/pokemon-survivor/go/ · Windows →
https://github.com/TxHandles/pokemon-survivor/releases/latest/download/PokemonSurvivor.exe (Windows may warn about an
unknown publisher the first time: More info → Run anyway). The browser build at `/go/` shares the website's save with
the web game (same browser storage), so progress carries over between the two.

A native rebuild in Go with [Ebitengine](https://ebitengine.org), reading the same `data/` folder — content added to
`data/` shows up in both versions. Builds a Windows `.exe` and a browser (WebAssembly) version from one codebase.

| Path | What it is |
|---|---|
| `engine/content` | Loads `data/*.json` (stats, learnsets, tier rules) |
| `engine/sim` | The game simulation — pure logic, no graphics, so bots can run it headless |
| `engine/meta` | Progression: save file (same format as the web game), shop, unlocks, mastery, challenges, rewards |
| `engine/synth` | Procedural sound effects and the placeholder music loop (same recipes as the web game) |
| `engine/lockstep` | Deterministic lockstep for online co-op: input frames, menu picks as commands, state-hash desync checks, dropped players |
| `engine/net` | Online rooms: host / join by code, lobby protocol, host relays inputs; WebRTC (browser API / pion on desktop) signalled through Firestore |
| `cmd/netpeer` | Headless online player over the desktop transport, for testing native ↔ browser play |
| `engine/client` | Rendering, input (keyboard, gamepad, mouse, touch), audio, HUD and menus |
| `cmd/game` | The playable game (Windows / browser) |
| `cmd/simhash` | Prints a state-hash trace of a fixed run; `tools/crosscheck.sh` compares native vs wasm to prove the sim is bit-identical across builds |
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

Online: the browser build signs in with Google against the same Firebase project as the web game, so it shares one
account, cloud save and leaderboards with it (Daily run → Leaderboards). The Windows build has no sign-in; it moves
progress with save codes. Both builds have the Pokédex, run history and the daily run (same biome / modifiers / world
as the web game for that day; daily records sync when signed in).

Local co-op (main menu → Local co-op): up to 4 players on one screen — keyboard (WASD / arrows), gamepads, and bot
teammates. In the lobby: Enter / A joins, B (or the + key) adds a bot, left / right picks a Pokémon, E / Y toggles the
ability, Enter / A readies up, Esc / B leaves. Level-ups come from a shared XP pool and each player picks their own
moves; fainted players revive at the Pokémon Center that appears on the map.

Online co-op (main menu → Online co-op): one player hosts a room and shares its 5-character code, up to three
others join with it; each picks a different Pokémon with their own upgrades, the host picks the run setup and
starts. Every player's game runs the same simulation in lockstep (only inputs travel, peer to peer over WebRTC;
rooms are signalled through the same Firestore `lobbies/{code}` documents as the web game), so nobody has a host
advantage; the games compare state hashes every second and end the run if they ever disagree. A player who
disconnects is dropped and the others carry on. The Windows build and the browser build play together: the
simulation is bit-identical across them (portable float math instead of the platform-specific `math.Exp` / `Pow` /
`Hypot`; `tools/crosscheck.sh` proves it in CI). In the browser you sign in with Google; the Windows build signs in
anonymously, which needs **Anonymous** enabled once under Firebase → Authentication → Sign-in method. Not yet
checked: other CPU architectures (arm64), and connections that need a relay (some strict NATs cannot connect peer to
peer — the guest then sees "could not reach the host").

Testing online co-op headless (all through the test relay `tools/sigserver.py`, no Firebase needed):
`cd build/web && python3 ../../tools/netcoop.py out 60 400` runs a host and a guest in two Chromium instances;
`go build ./cmd/netpeer && python3 ../../tools/crossplay.py CODE 60` has the native peer host for a browser guest;
two `netpeer`s (`-host CODE` / `-join CODE`, with `-signal http://localhost:8773`) test the desktop transport alone.

The browser build accepts `?demo=<id>&biome=<type>&seed=<n>&t=<seconds>` to watch the bot play, and
`?coop=fire,water,grass` for a bot co-op demo.
GitHub Actions builds both on every push and publishes the browser build to the site (`go/`, gzipped wasm);
pushing a `go-v*` tag makes a GitHub release with the Windows exe (`git tag go-v0.2.0 && git push origin go-v0.2.0`).
