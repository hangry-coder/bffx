# ADR: Archetype CLI Syntax Design for `bffx new`

## Context

BFFX is introducing vertical archetypes (e.g., `fintech`, `superapp`, `dictation`) to accelerate project onboarding. A developer starting a new application wants to select an archetype to pre-wire batteries, directories, remote config, and pipeline schemas.

We need to decide the command-line interface (CLI) syntax for creating a new project from an archetype.

## Options Considered

### Option 1: Positional archetype as the first argument (`bffx new <archetype>`)
Under this design, the first positional argument is the archetype name:
```bash
bffx new fintech
```
* **Pros:** Extremely fast and clean to type.
* **Cons:** Makes it hard/confusing if a developer wants to name their project something else (e.g., `mybank` instead of `fintech`). Forcing the project name to match the archetype is too restrictive.

### Option 2: Explicit flag for archetype (`bffx new <name> --archetype=<archetype>`)
Under this design, the first positional argument is always the project name, and an optional flag specifies the archetype:
```bash
bffx new mybank --archetype=fintech
```
* **Pros:** Clear, explicit, and allows arbitrary project naming.
* **Cons:** Slightly more wordy for quick-start demos where matching name/archetype is fine.

### Option 3: Dual-Mode Smart Syntax (Selected)
We support both approaches seamlessly:
1. **Explicit flag mode:** `bffx new <name> --archetype=<archetype>` (e.g. `bffx new mybank --archetype=fintech`).
2. **Positional fallback mode:** `bffx new <archetype>` (e.g. `bffx new fintech`). If only one positional argument is provided, and that argument matches a known archetype preset in the registry, we treat it as both the project name *and* the chosen archetype.

## Decision

We will implement **Option 3 (Dual-Mode Smart Syntax)**.

This delivers the best of both worlds:
- **Maximum speed:** For demos or standard scaffold naming: `bffx new fintech` works out-of-the-box.
- **Production flexibility:** For specific production names: `bffx new my-bank --archetype=fintech` works perfectly.

## CLI Signature Contracts

* `bffx new --list-archetypes` -> Lists all registered archetypes and their presets.
* `bffx new <name> --archetype=<alias>` -> Scaffolds project `<name>` using `<alias>` preset.
* `bffx new <alias>` -> Scaffolds project `<alias>` using `<alias>` preset (if `<alias>` is in the registry).
