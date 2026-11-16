# Folded terminal K branding verification

Owner-approved implementation, 2026-11-16. Local and unreleased.

## Result

The folded K and terminal underscore replace the current production logo in the canonical SVGs,
README dark/light logos, website header/footer/favicon/install icon, app title bar, Settings footer,
About, macOS icon and Windows executable resources. Wordmark lettering and attribution are retained.
The app parses semantic SVG groups once and draws the matching violet gradient. Footer spacing
accommodates the wider mark; the title mark scales within its available width.

`assets/branding/README.md` documents sources and exports. `tools/branding/` pins resvg 2.6.2,
exports the 512px Windows PNG and eleven ICNS PNG representations (16 to 1024px), and copies
website vectors. Both Windows resource files were rebuilt with go-winres 0.3.3.

## Verification

- RED: updated platform/website checks rejected old lilac exports; geometry checks rejected the
  old mark bounds and wordmark placement; actual Gio rendering rejected the missing underscore
  and flat fill. GREEN: these pass with the new artwork.
- `go test ./...` passes. `go vet ./...` passes. A later focused contrast check passes in all ten themes.
- Actual headless Gio gradient/underscore checks pass for both light/dark palettes. Both gradient
  endpoints meet 3:1 on every relevant page, navigation and title background in all ten themes.
- About renders generated for ten themes, wide/narrow; title renders for ten themes, Mac/Windows,
  sidebar visible/hidden. Inspected Midnight wide and Washi narrow representative app captures.
- Separate headless Chrome checked index and changelog pages at 1440px and 390px. Every logo
  loaded; no horizontal overflow. Inspected desktop/mobile landing page screenshots.
- Native macOS build, `tools/package-macos.sh`, and strict deep signature validation pass.
  Existing native notification deprecation and duplicate linker-library warnings remain.
- Windows amd64 and arm64 cross-builds pass. Resource version metadata remains 0.41.1.
- ICNS is recognised by macOS sips at 1024px; platform tests decode/check its required PNG entries.
- `git diff --check` passes. Unrelated pre-existing untracked designs were preserved.

Selected captures in `branding/` are development evidence, not replacements for native product captures.

## Remaining external checks

The installed app and live GitHub Pages have not been replaced or published. The owner requested a review PR; no merge or release was requested. Retake product screenshots after native owner review. Check Dock/Finder/taskbar and
native Windows display after release. The Discord Rich Presence artwork is an external upload
under the `kaze` key; it needs a separate developer-portal replacement using the new icon.
The artwork SVGs are fully vector; the conceptual PNG and earlier designs remain historical.

Deferred Discord artwork and product retakes: [issue #205](https://github.com/beefysalad/kaze-terminal/issues/205).
