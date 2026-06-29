# Website UI Kit — opensource.finance landing

A faithful recreation of the **opensource.finance** marketing site (`opensource-finance/opensource.finance`, `src/components/landing/*`), composing this design system's primitives.

**Run:** open `index.html`.

**Sections** (each a small component, all driven off DS tokens):
- `sections.jsx` — `Header` (glassy floating pill nav), `Hero` (display headline + italic-serif accent), `Credibility` (Tazama / Gates / Linux Foundation), `Features` (4 `FeatureCard`s), `Footer`.
- `demo.jsx` — `TransactionDemo`, the looping "god-view" state machine: sender phone → intercept wire + Osprey `Terminal` → ALLOWED → receiver notification.
- `platform.jsx` — `Comparison` (the sticky Osprey-vs-incumbents table with the red accent column), `PlatformVision` (Studio / Osprey / Cases tiles), `FounderStory`.
- `app.jsx` — composes `Landing`.

Components consumed from `window.OpensourceFinanceOspreyDesignSystem_08d2ca`: `Button`, `Badge`, `Eyebrow`, `FeatureCard`, `Terminal`, `Avatar`, `Wordmark`. Feature/status icons are inline Lucide paths lifted from source.
