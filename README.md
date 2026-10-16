# Pokemon Survivor

A Vampire-Survivors-style roguelite. Play at https://txhandles.github.io/pokemon-survivor/

- `index.html`: the built game the website serves (generated — don't edit by hand)
- `data/`: all content (Pokémon, moves, learnsets, abilities, balance, locations, music, sprites) — see [CONTENT.md](CONTENT.md)
- `src/`: game engine template, online co-op, friends and website build
- `tools/build.py`: builds `index.html` (and a standalone `build/monster-survivors.html`) from `data/` + `src/`
- `music/`: location music
- `firestore.rules`: Firebase Firestore security rules (paste into Firebase console → Firestore → Rules)
