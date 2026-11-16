# Folded K terminal branding rollout

Status: owner approved rollout on 2026-11-16; implemented for a review PR; no merge or release requested.

The owner selected the folded K and accepted the developer refinement on 2026-11-16, then requested actual SVG artwork usable throughout the product. Canonical hand-built vector masters are in `assets/branding/`. They contain paths and gradients only, including outlined wordmarks, without raster images or external dependencies.

1. Promote the new mark, full dark/light logos, compact lockups and icon SVGs into `assets/branding/`. Preserve the existing outlined wordmark and byline. Use enhanced violet on dark backgrounds and a deeper purple contrast variant on light backgrounds. Keep the platform-specific macOS icon margins.
2. Update the embedded logo parser to identify the mark independently of its old fill color. Update the Gio mark drawing to preserve the violet gradient in About, the title bar and Settings footer. Add bounded regression checks for the new geometry, terminal underscore, gradient and light-theme contrast before changing behavior; watch failures first.
3. Export Windows icon PNG and all macOS icon sizes from the SVG masters. Generate ICNS and both Windows resource objects using the existing README command, retaining version metadata. Document the export procedure so vector changes remain reproducible.
4. Replace website header/footer lockups and favicon, bump favicon cache keys, update README branding and check other current production references. Historical designs, changelog entries and screenshots retain their historical artwork. Record required native screenshot retakes.
5. Render the app branding in dark/light themes and narrow layouts; inspect desktop/mobile website and icon exports. Run branding and UI tests, full Go tests and vet, native macOS build and Windows cross-builds. Do not replace or launch over the installed app.
6. Update HANDOFF with results and outstanding native checks. No commits, pushes or merges without owner instruction. Once an app branding PR is merged, follow CLAUDE.md for its release, including version, changelog and download artifacts.
