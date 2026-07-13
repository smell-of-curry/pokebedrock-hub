# AGENTS.md

## Learned User Preferences

## Learned Workspace Facts

- gophig/`config.toml` zero-defaults: a missing `[Watchdog]` section leaves `WorldExecTimeout=0`, so `context.WithTimeout(..., 0)` fails immediately with `context deadline exceeded`. The Watchdog package may log a 10s backfill while startup `world.Call` paths (slapper/NPC/parkour summon) still see 0 — `ReadConfig` must fill zero watchdog fields from defaults, and startup summon must finish (e.g. `context.Background()`) rather than inherit a zero timeout.
- Hub startup ordering: run slapper/parkour summon before queuing spawn-chunk preload (`w.Do(Load)`); preload can saturate the world owner with hundreds of chunk loads and starve summon even when a real timeout exists.
