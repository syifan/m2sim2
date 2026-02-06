# CLAUDE.md

## Build, Test, Lint

```bash
go build ./...            # Build
ginkgo -r                 # Test (or: go test ./...)
golangci-lint run ./...   # Lint
```

## Conventions

- **Reuse Akita components** — Use Akita's cache/memory controllers, don't reinvent them. Raise issues upstream if needed.
- **Separate functional and timing logic** — `emu/` is for emulation, `timing/` is for cycle-accurate simulation.
- **Track instruction support** — Update `insts/SUPPORTED.md` when adding instructions.
- **Follow Go best practices** — Use Akita component/port patterns for timing model.
- **See SPEC.md** — For architecture decisions, milestones, and design philosophy.
