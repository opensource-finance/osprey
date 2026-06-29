# opensource.finance — Osprey Design System

> Open-source infrastructure for the next generation of fintech.
> *The osprey never misses.*

This is the brand & product design system for **opensource.finance** and its flagship project **Osprey** — a single-binary, real-time transaction-monitoring engine — plus **Osprey Narrator**, a fine-tuned LLM that turns alert JSON into analyst-ready compliance narratives. Use it to build on-brand interfaces, marketing pages, decks, and prototypes.

---

## 1. Company & product context

**opensource.finance** builds "the operating system for financial vigilance" — transaction monitoring and AI-powered compliance automation, all open source (Apache 2.0), built to run on *your* infrastructure. The thesis: the rigor of national-payment-rail fraud detection, made accessible to "everyone who isn't a bank."

Three surfaces make up the world:

| Product | What it is | Surface in this system |
|---|---|---|
| **🦅 Osprey** | Single-binary, real-time transaction-monitoring engine. Go + CEL-Go rule engine. Detection mode (weighted scoring) & Compliance mode (FATF typologies). Deploys in 60 seconds. | CLI / engine — **no GUI**. Represented by the marketing site + the live "god-view" transaction demo. |
| **🤖 Osprey Narrator** | Fine-tuned Qwen3-4B LoRA that converts an Osprey alert JSON into a structured SAR-style narrative (12 FATF rules + 6 typologies). Ships as a HuggingFace LoRA adapter and an Ollama GGUF. | Model / CLI — **no shipped GUI**. The `narrator` UI kit renders the *documented* SAR output structure as a console. |
| **🌐 opensource.finance** | The marketing website. Vite + React + Tailwind v4 + shadcn, DM Sans, Swiss-minimalist. | The `website` UI kit — a faithful recreation. |

**Heritage:** the founding team came from **Tazama** (the open-source transaction monitor born out of the **Gates Foundation** LevelOne Project, now stewarded by the **Linux Foundation**). Positioning line: *"Banks have Tazama. Everyone else has Osprey."*

### Sources this system was built from

All three are GitHub repos under the `opensource-finance` org. Explore them to go deeper / verify fidelity:

- **Website (primary design source):** https://github.com/opensource-finance/opensource.finance — `src/index.css` (tokens), `src/components/landing/*` (sections), `src/components/ui/*` (shadcn primitives), `src/components/brand/osprey-logo.tsx` (logo), `src/components/landing/transaction-demo.tsx` (the interactive "god view").
- **Engine:** https://github.com/opensource-finance/osprey — `README.md` (positioning, modes, API), `docs/`.
- **Narrator:** https://github.com/opensource-finance/osprey-narrator — `MODEL_CARD.md` (SAR output structure, FATF rule/typology lists), `GUIDE.md`.

> The reader is **not** assumed to have access to these — everything needed is captured here — but they're stored so a deeper pass can pull exact source.

---

## 2. Content fundamentals — voice & tone

The voice is **confident, technical, and quietly anti-establishment.** It speaks developer-to-developer, with the swagger of people who built the serious version and decided to give it away.

**Person & address**
- **"We"** = the company/product ("We built for the rest of us." "We stripped out the enterprise complexity.").
- **"You"** = the developer reading ("You don't need a Kubernetes cluster to run fraud checks." "Own your data.").
- **"I"** appears *only* in the founder story (Joseph Goksu) — first-person, personal, mission-driven.

**Rhythm & structure**
- Short, punchy declaratives. Fragments for emphasis, stacked: *"Single binary. Built in Go. No enterprise bloat."* — *"No JVM. No Node_modules. Just one standard binary."*
- A claim, then the proof number: *"60 seconds," "single binary," "7+ microservices," "$30k–$740k/yr," "Verified in 118ms."*
- Confident, slightly provocative headers: *"The Uncomfortable Truth," "Transaction monitoring for everyone who isn't a bank."*
- Concrete, human analogies: *"in less time than it takes to brew coffee."*

**Casing**
- **Hero / big headlines:** sentence case, tight tracking. One word set in *italic serif* for warmth ("...for *everyone* who isn't a bank").
- **Section titles:** Title Case ("The Platform Vision," "The Uncomfortable Truth").
- **Micro-labels / eyebrows:** UPPERCASE, wide letter-spacing, tiny (10–12px), often muted or in an accent color — "ACTIVE INTERCEPTION," "SENDING," "ENGINEERED BY THE."
- **Product/tech names** keep their canonical casing: Osprey, Tazama, FATF, CEL-Go, ISO 20022, gRPC, AML/CFT, SAR.

**Emoji** — yes, but scoped. Emoji appear in **marketing/README copy and platform listings** as friendly product/section markers (🦅 Osprey, 🤖 Narrator, 🔗 links, 🤗 HuggingFace, 🦙 Ollama, 🌐 site). They are **not** used inside dense product UI — there, line icons (Lucide/Feather) and the wordmark dot carry the load. The one in-UI emoji in the source is a placeholder avatar (👤). Rule of thumb: emoji to *label and delight in prose*, never to convey state in a console.

**Vibe in three words:** rigorous, accessible, irreverent.

Example snippets to match:
> "Transaction monitoring for everyone who isn't a bank."
> "No Kubernetes. No microservices. Just download and run."
> "Banks have Tazama. Everyone else has Osprey."
> "From alert to narrative in seconds, not hours."
> "Own your data."

---

## 3. Visual foundations

**Overall feel:** Swiss-minimalist / international-typographic. Lots of white space, one decisive accent, tight type, soft corners. Editorial restraint with a single warm flourish (the italic serif word).

**Color**
- **Base:** near-white paper (`oklch(0.99 0 0)`) and soft-black ink with a faint blue cast (`oklch(0.15 0.02 260)`). Greys are pure neutral (chroma 0).
- **The one accent — Swiss Red** (`oklch(0.55 0.22 27)`, a warm coral-vermilion). Used sparingly but decisively: the wordmark square, primary buttons, the active-column accent in the comparison table, badge fills, links on hover, the live "pulse" dot.
- **System palette** for interactive product surfaces (the transaction "god view"): Apple-style **blue `#007AFF`** (logo lens, "Active Interception" labels), **green `#34C759`** (ALLOWED / verified), **amber `#F5A623`** (WARN), on **deep slate `#0F172A`** terminal chrome over an Apple-grey **`#F5F5F7`** canvas.
- **Dark mode** is deep charcoal (NOT pitch black), same faint-blue cast, with a slightly lifted red.

**Typography**
- **DM Sans** everywhere — the product/marketing typeface. Headlines run `font-weight: 500` (medium, not bold) with **tracking-tighter** (`-0.03em`) and tight `1.1` line-height.
- **Signature move:** one **italic serif** word dropped into a sans headline for warmth (hero "everyone," the S/O/C monogram letters in the platform cards). Serif here is a *substitution* — see Caveats.
- **JetBrains Mono** for the terminal / log stream (also a substitution — site uses system mono).
- **Micro-labels:** UPPERCASE, `letter-spacing: 0.1em`, 10–12px, muted or accent.

**Spacing & layout**
- 4px base grid. Section rhythm is huge — `py-24` to `py-32` (96–128px). Content centers in `max-w-5xl` (1024px); the comparison table goes `max-w-7xl`.
- Generous internal padding: cards 24px, feature cards 32px.

**Corners (soft & generous)**
- **Pills** (`9999px`) for all interactive chrome: buttons, badges, nav, the floating header.
- **Large radii** for content: feature cards `2rem` (32px), platform cards `1.5rem` (24px), device frames `2.5rem` (40px). Default card/button `0.75rem` (12px). Base `--radius` is `1rem`.

**Borders**
- Hairline `1px` in light grey (`oklch(0.92 0 0)`), often at reduced opacity (`border/40`). A **4px red top-bar** marks the "Osprey" column in the comparison table — the one heavy border in the system.

**Shadows (subtle, cool, blue-black)**
- Small and soft on resting cards (`shadow-sm`); deeper on floating UI (device frames, terminal use `shadow-2xl`). No harsh or colored drop shadows. Inner shadows are not a motif.

**Backgrounds & texture**
- Mostly flat white / near-white. **No gradients** as page backgrounds. Section differentiation is by subtle grey wash (`bg-secondary/20`) or the Apple-grey demo canvas (`#F5F5F7`). The only gradients are *functional*: the moving "packet" on the transaction wire (transparent → color → transparent). No photography, no illustration, no repeating patterns — the interest comes from type, the red accent, and the live demo.

**Glass / transparency / blur**
- The floating header is glassy: `background/60` + `backdrop-blur-md`. The terminal panel is `slate/95` + `backdrop-blur-md`. Blur is reserved for *floating overlays* (header, terminal), never decorative.

**Motion**
- Calm and professional. Entrances **fade + slide-up from 8px**, ~700ms, eased out, with small stagger delays (`delay-200`). A 2px pulse dot animates on "live" badges. The transaction demo is a looping state machine (send → intercept → analyze → ALLOWED → release → received) driven by framer-motion. Reduced-motion should fall back to the visible end-state.

**Hover & press states**
- **Hover:** opacity drop (`hover:opacity-80`), accent darken (`hover:bg-primary/90`), a border *appears* on previously-borderless cards, and contained icons scale up (`group-hover:scale-110`). Transitions are `transition-colors` / `transition-all` at 300ms.
- **Press / active:** color deepen (the `--primary-hover` / `red-600` token). No aggressive shrink.
- **Focus:** a soft 3px ring in the accent color at ~30% (`--shadow-focus`).

**Imagery color vibe:** there's essentially no photography. The OG/brand image is the white wordmark + red square on near-black — high-contrast, cool, graphic. Keep imagery (if ever added) clean, cool, and high-contrast.

---

## 4. Iconography

**System:** stroke-based line icons in the **Lucide / Feather** family. The source uses raw Lucide SVGs inline (`box`, `zap`, `boxes`, `feather`, `check-circle`, `x-circle`) and imports `react-icons/fi` (Feather: `FiCheckCircle`, `FiActivity`, `FiShield`) in the demo.

**Characteristics**
- `stroke-width` **1.5–2.5** (1.5 for decorative/feature icons, 2–2.5 for inline status), `stroke-linecap="round"`, `stroke-linejoin="round"`, `fill="none"`, `currentColor`.
- Icons sit at 16–24px inline, or 24px inside a 48px white circular chip with a hairline border + `shadow-sm` (the feature cards).
- Status icons inherit semantic color: green check, red x, blue activity.

**Recommendation:** load **Lucide** from CDN — it's a 1:1 match for what the codebase already draws.
```html
<script src="https://unpkg.com/lucide@latest"></script>
<script>lucide.createIcons();</script>
<!-- <i data-lucide="shield-check"></i> -->
```
Feather is the legacy equivalent if Lucide is unavailable. **Do not** hand-roll icon SVGs or use emoji as functional icons in product UI (emoji are for prose — see Content Fundamentals).

**Brand illustration set** — beyond functional UI icons, the system ships a bespoke **line-illustration set** (`assets/illustrations/*.svg`, showcased in the "Brand illustrations" card) capturing the product's soul across four pillars: **Vigilance/Osprey** (osprey, monitor, signal, verified, alert, scrutiny), **Open source** (open-source, binary, fork, open, global, deploy), **Community** (community, network, contribute), **Finance** (balance, funds, records, transfer). All share the same single-weight grammar — 48×48 viewBox, `stroke-width: 2.2`, round caps/joins, `currentColor` (so they tint per context; a brand-red accent dot echoes the wordmark motif on light tiles). They sit on calm "illustration palette" tiles (earthy editorial tones rooted in the core hues — Swiss-red terracotta, deep teal, forest, slate, olive, plus soft sage/mint/blush/lavender/cream). Use them for empty states, feature markers, section headers, and category chips — **not** as inline UI affordances (use Lucide there).

**Brand mark:** the Osprey logo (`assets/brand/osprey-mark.svg`) is a custom SVG — a rounded shield housing a stylized eye/lens of interlocking shutters with a center pupil ("active monitoring"). Drawn in `#007AFF`; a `currentColor` variant (`osprey-mark-mono.svg`) is provided for tinting. The recurring **wordmark dot** — a tiny Swiss-red square (OG image) or circle (header) preceding "opensource.finance" / badge text — is the lightweight brand signature; reach for it before the full logo.

---

## 5. Index / manifest

**Root**
- `styles.css` — the single entry point consumers link. `@import`s the four token files + a tiny base reset.
- `readme.md` — this guide.
- `SKILL.md` — Agent-Skills-compatible front-matter for use in Claude Code.

**`tokens/`** — `fonts.css` (webfonts), `colors.css`, `typography.css`, `spacing.css` (radius / shadow / motion).

**`assets/brand/`** — `osprey-mark.svg`, `osprey-mark-mono.svg`, `og-image.png` (wordmark lockup).

**`assets/illustrations/`** — the 19-piece brand line-illustration set (`osprey`, `monitor`, `signal`, `verified`, `alert`, `scrutiny`, `open-source`, `binary`, `fork`, `open`, `global`, `deploy`, `community`, `network`, `contribute`, `balance`, `funds`, `records`, `transfer`). `currentColor`, 48×48.

**`components/`** — reusable React primitives (`window.<Namespace>.<Name>`):
- `core/` — `Button`, `Badge`, `Eyebrow`
- `surfaces/` — `Card`, `FeatureCard`
- `forms/` — `Input`, `Textarea`, `Field`
- `feedback/` — `Terminal`, `Avatar`
- `brand/` — `OspreyMark`, `Wordmark`

**`ui_kits/`** — full-screen recreations:
- `website/` — the opensource.finance landing page (faithful, interactive).
- `narrator/` — the SAR-narrative console (grounded in `MODEL_CARD.md`).

**Foundation cards** — small `@dsCard` HTML specimens populate the Design System tab (Type, Colors, Spacing, Brand groups).

---

## 6. Caveats & substitutions

- **Serif accent font:** the live site uses the browser's default serif for the italic accent word. This system substitutes **Newsreader** (Google Fonts) as an on-brand editorial serif. *Flag for the team — swap if there's a licensed serif.*
- **Monospace:** the site uses the system mono stack; this system substitutes **JetBrains Mono** for consistent terminal rendering. *Flag.*
- **No GUI for Osprey engine or Narrator:** both ship as CLI / model artifacts. The `narrator` UI kit is a reasonable rendering of the *documented* SAR output structure, not a recreation of a shipped screen. The `website` kit is a faithful recreation of real code.
- **Icons** are loaded from the Lucide CDN (matches source); no icon binaries are vendored.
