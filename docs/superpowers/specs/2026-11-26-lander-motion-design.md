# Lander motion (design)

Date: 2026-11-26. Owner request: "fluid animations on the lander page". Builder's calls are marked **(call)**.

## Intent

The lander (`docs/index.html`, `docs/site-assets/lander.css`, `lander.js`) is quiet and flat on purpose (HANDOFF,
"Lander restyled": no gradients, glows, eyebrow labels, section numbers or card grids). Motion should make it feel
fluid and alive without breaking that: smooth, restrained, fast, and never in the way of reading or the download button.

## Principles

- Only `transform` and `opacity` animate (compositor only). No layout-shifting animation, no `filter`/blur, no glows,
  no gradient sweeps. Cumulative layout shift stays 0.
- One easing and a tight set of durations, as CSS custom properties: ease `cubic-bezier(.22,1,.36,1)`, 160 ms
  (feedback), 320 ms (small), 640 ms (reveals) **(call)**.
- Content is fully visible and usable without JavaScript (reveals are gated behind a `js` class on `<html>` set by
  the first inline line of script) and with `prefers-reduced-motion: reduce` (no movement at all, only instant
  state changes; the existing reduced-motion rule stays).
- No libraries, no new fonts, no new images. JetBrains Mono and Manrope stay as they are (the lander's own fonts).
- Nothing runs while the page is idle and offscreen: the screenshot wall drift pauses when the tab is hidden or
  the wall is offscreen (IntersectionObserver plus `visibilitychange`), so it costs 0% CPU there **(call)**.

## What moves

1. **Hero entrance on load**: headline, second line, buttons, then the window/wall rise 12 px and fade in, staggered
   70 ms, 640 ms each; the whole sequence under 1.2 s.
2. **Scroll reveals**: each `.k-sec` header, each ruled-list row, FAQ item, changelog release and install step fades in
   and rises 14 px once when it enters the viewport (IntersectionObserver, threshold 0.15, unobserve after); rows inside
   one list stagger 50 ms, capped at 6 steps so long lists never wait.
3. **Header**: it gains a 1 px bottom border and a slightly denser background after 8 px of scroll (opacity change of
   a pseudo-element, no layout change); the nav link of the section in view gets a sliding underline (transform on a
   single indicator) **(call)**.
4. **Feedback**: buttons and links press to 0.98 scale in 160 ms and ease color/border; feature-index rows nudge their
   icon 2 px on hover; focus rings stay instant and visible.
5. **FAQ**: `details` open and close with a smooth height (grid-template-rows 0fr to 1fr trick, no JS measuring) and
   the chevron rotates.
6. **Screenshot lightbox**: opens with a fade plus a scale from 0.96, closes the same way in reverse; Esc and a click
   outside close it as today; focus handling is unchanged.
7. **Smooth anchors**: in-page links scroll smoothly with an offset for the sticky header (`scroll-margin-top`).
8. **Wall**: keeps its drift; hovering a tile lifts it 3 px; nothing else.

## Non-goals and anti-goals

No parallax that moves text, no cursor-following effects, no scroll-jacking, no looping attention-grabbers, no
autoplaying video, no animated gradients or glows, no change to copy, layout or screenshots, no new dependencies.

## Verification

Headless-browser captures at 1440, 1024 and 390 px of the top, mid and bottom, with JavaScript on, JavaScript
off, and `prefers-reduced-motion: reduce`; a check that every `.k-sec` is visible within 2 s of scrolling to it and
without JS; CLS measured 0; no console errors; an idle check that the wall animation is paused when hidden.
The owner reviews a local preview before the PR is merged. A lander-only change does not get a release (CLAUDE.md).
