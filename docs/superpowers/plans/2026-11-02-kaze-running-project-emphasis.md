# Implementation plan: running mark and active project (#107)

Design: ../specs/2026-11-02-kaze-running-project-emphasis-design.md

Status: both parts are complete locally. The owner approved the active-header
design and this continuation on 2026-11-03 after the localhost mockup. Bounded
regression tests, all theme/scale pixels/renders, contrast and zero-allocation
cache checks, full tests/vet, focused races, macOS build and Windows cross-build
pass. Native installed-app checks remain open. No release, commit or push.

1. Add bounded failing coverage for the shared running mark's closed ring/core,
   in-box geometry, all states and 1x/2x scale; cover active repository selection,
   switching, folding/filtering, worktrees and absent repo, plus cache behavior.
2. Replace the shared open-arc drawing with one ring/core primitive. Route every
   current running mark through it and remove legacy overflow compensation while
   preserving positions and spacing of surrounding text and controls.
3. Carry the active repo key in sidebar drawing state/cache and tint just its
   existing group header. Preserve session selection and waiting counts.
4. Inspect actual headless theme/scale renders, confirm no idle invalidations,
   run full tests/vet/focused races/macOS and Windows builds, and update the
   README, HANDOFF, issue #107 scope and audit evidence. No release or push.
