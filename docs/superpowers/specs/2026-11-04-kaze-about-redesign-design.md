# Settings › About redesign: design

Owner approved 2026-11-04 from `docs/design/about-redesign-mockup.html`. Scope: the About page only
(app icons, lander and README branding are separate work).

## TL;DR

A compact, centred colophon: the approved logo, the tagline and the name's meaning, then one card that
holds the facts and, in its footer, the actions.

## Layout (top to bottom, one centred column, at most 440dp wide)

- **Lockup:** the mark and wordmark from the approved `kaze-logo.svg` (docs/design/logo-premium), drawn as vector
  paths (embedded as `assets/branding/kaze-lockup.svg`), 208dp wide (168dp when the pane is under
  420dp). The SVG's outlined byline is not drawn: at this size it would be about 6px. "by Patrick" is
  live JetBrains Mono text (11sp, 10sp narrow) starting at the wordmark's left edge, its baseline on
  the bottom of the mark.
- **Tagline:** "Light, fast, and out of the way." Manrope Medium 15sp, title colour. Under it the
  meaning, 12.5sp muted: "風 Kaze means "wind"." (kanji from the bundled Japanese font).
- **Card** (the settings card colour and border, 12dp radius):
  - Rows, label column 78dp: **Version** `0.29` with the update state at the right (green dot "Up to
    date", accent dot "<v> available", nothing before a check); **Platform** `macOS · arm64`;
    **License** `MIT`. Values in mono. The version shows once on the page (the sidebar footer is
    shared chrome).
  - Footer bar (raised colour, top hairline): **Copy build info** (outline) at the left; **Report an
    issue** (outline) and **View on GitHub** (solid: title-colour fill, page-colour text) at the right.
- **Footnote:** "Made with Go and Gio, for the long session." 11.5sp dim, centred.

## Narrow (pane under 420dp)

Smaller lockup; the card takes the column width; the footer buttons stack full width with View on
GitHub first.

## Themes

Mark lilac `#c2b4eb` on dark themes, `#7563bf` on light (Washi), so it holds contrast. Wordmark in the
title colour. Everything else uses the page's theme tokens.

## Unchanged

Copy build info (same clipboard text), the GitHub and issue links, their failure notes.
