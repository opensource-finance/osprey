---
name: osprey-design
description: Use this skill to generate well-branded interfaces and assets for opensource.finance & Osprey, either for production or throwaway prototypes/mocks/etc. Contains essential design guidelines, colors, type, fonts, assets, and UI kit components for prototyping.
user-invocable: true
---

Read the `readme.md` file within this skill, and explore the other available files.

If creating visual artifacts (slides, mocks, throwaway prototypes, etc), copy assets out and create static HTML files for the user to view. If working on production code, you can copy assets and read the rules here to become an expert in designing with this brand.

If the user invokes this skill without any other guidance, ask them what they want to build or design, ask some questions, and act as an expert designer who outputs HTML artifacts _or_ production code, depending on the need.

## Quick map
- `readme.md` — the full design guide: company context, voice & tone, visual foundations, iconography, caveats.
- `styles.css` — link this one file; it `@import`s every token + font. Reference tokens like `var(--primary)`, `var(--font-sans)`, `var(--radius-pill)`.
- `tokens/` — `colors.css`, `typography.css`, `spacing.css`, `fonts.css`.
- `assets/brand/` — `osprey-mark.svg`, `osprey-mark-mono.svg`, `og-image.png`.
- `assets/illustrations/` — 19-piece brand line-illustration set (`currentColor`, 48×48): open source · community · finance · vigilance.
- `components/` — React primitives (Button, Badge, Eyebrow, Card, FeatureCard, Input, Textarea, Field, Terminal, Avatar, OspreyMark, Wordmark). Each has a `.prompt.md` with usage.
- `ui_kits/website/` — the marketing landing page recreation.
- `ui_kits/narrator/` — the SAR-narrative console.

## The 10-second brief
Swiss-minimalist: near-white paper, soft-black ink, **one** Swiss-red accent (`oklch(0.55 0.22 27)`). DM Sans everywhere, medium weight, tracking-tighter, with a single **italic-serif accent word** in big headlines. Pills for buttons/badges/nav; large soft radii for cards. Subtle cool shadows, no gradient backdrops. A second "system" palette (blue/green/amber on slate) drives interactive product surfaces like the Osprey terminal. Voice: confident, technical, quietly anti-establishment — "Transaction monitoring for everyone who isn't a bank."
