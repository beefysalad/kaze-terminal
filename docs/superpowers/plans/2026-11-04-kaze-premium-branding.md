# Premium Kaze branding across app and website

Status: logo design approved; expanded implementation plan awaiting approval.

## Scope

Use the approved tapered wind K, Manrope wordmark and small by Patrick credit
from `docs/design/logo-premium/` across the product.

1. Make the approved SVGs the canonical branding assets. Export transparent
   mark and lockup PNGs, plus a light-background lockup with dark lettering.
   Keep a reproducible asset generation script and preserve font licensing.
2. Generate the app icon with the new mark on the existing midnight tile.
   Replace the macOS ICNS and Windows icon PNG, then regenerate both Windows
   resource files. Keep resource version information in step with the app version.
3. Update About to show the approved full lockup, including its wordmark and
   byline, rather than reconstructing it with the existing monospaced text.
   Use the appropriate contrast variant for each theme and preserve narrow layouts,
   build details and buttons. Preserve the existing image caching approach.
   Audit other uses of the embedded mark so they inherit the new branding.
4. Update the lander header, footer, favicon, shared changelog branding and README
   logo. Check all tracked old-logo references. Leave historical product captures
   intact and record which About captures need retaking.
5. Follow project TDD for renderer behavior changes: cover lockup decoding,
   aspect ratio, reuse and theme selection before implementation. Run relevant
   UI tests, then `go test ./...` and `go vet ./...`. Build the macOS bundle,
   inspect generated icon sizes, and verify the Windows resources regenerate.
   Render and inspect About in dark/light themes and narrow layouts, and check
   the website on desktop and mobile.
6. Update the handoff with verification and any native owner checks still owed.
   Do not overwrite the installed app. Do not commit or push without instruction.
   Once an app branding PR is merged, follow CLAUDE.md's release workflow so
   installed copies receive the new branding.

## Completion

One consistent identity appears in the app's About page, Dock/executable/taskbar
icons, website and README. Native Windows display is an owner check if no Windows
runtime is available; resource generation alone does not verify it. Existing
installed copies receive the change through the release, not source edits alone.
