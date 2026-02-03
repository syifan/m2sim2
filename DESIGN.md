# DESIGN.md - Design Philosophy

## Independence from MGPUSim

While M2Sim uses Akita (like MGPUSim) and draws inspiration from MGPUSim's architecture, **M2Sim is not bound to follow MGPUSim's structure**. Make design decisions that best fit an ARM64 CPU simulator.

### Guidelines

1. **Choose meaningful names**: If a different name is more appropriate, use it
   - Example: `driver/` → consider `os/` (clearer for OS/syscall emulation)
   
2. **Adapt to CPU semantics**: GPU and CPU have different abstractions
   - No wavefronts, warps, or GPU-specific concepts
   - Pipeline stages, branch prediction, and caches are CPU-native

3. **Keep it simple**: M2Sim targets single-core initially
   - Avoid over-engineering for multi-core until M5+
   - Don't add GPU-style memory hierarchies (e.g., no LDS)

4. **Diverge when it makes sense**: If MGPUSim does something that doesn't fit:
   - Document why you're doing it differently
   - Choose clarity over consistency with MGPUSim

### What to Keep from MGPUSim

- Akita component/port patterns (they work well)
- Separation of concerns (functional vs timing)
- Testing practices (Ginkgo/Gomega)

### When in Doubt

Ask: "What would make this clearest for a CPU simulator?" — not "What does MGPUSim do?"

## Package Naming Suggestions

| Current | Alternative | Rationale |
|---------|-------------|-----------|
| `driver/` | `os/` | Handles OS services and syscalls, not a device driver |
| `emu/` | `emu/` | Good as-is: "emulator" is clear |
| `timing/` | `timing/` | Good as-is: describes cycle-accurate mode |

*These are suggestions, not mandates. Rename when it improves clarity.*
