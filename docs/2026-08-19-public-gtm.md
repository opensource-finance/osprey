# Osprey / opensource.finance — GTM commercial brief

**Prepared:** 19 August 2026 (Europe/London, BST)  
**Subject:** Osprey (transaction monitoring) — public-source commercial brief for Joseph Goksu / opensource.finance  
**Method:** Public pages only. No invented metrics, customers, funding, licenses, or “interested” leads. If a number is not on a public page, it is marked **not published**.  
**Do not send** the outreach drafts. They are drafts only.

---

## What it is

Osprey is an open-source **transaction-monitoring engine**: a single Go binary that evaluates a payment against CEL rules (and, in compliance mode, FATF-style typologies) and returns `ALRT` or `NALT`. The marketing site positions it as “real-time transaction monitoring that deploys in 60 seconds” for fraud detection and AML/CFT, explicitly against bank-scale platforms.

A companion model, **Osprey Narrator**, turns that JSON into a SAR-style narrative. It is a separate artifact (Hugging Face / Ollama), not the engine.

---

## Public facts

| Fact | What is public | What is not |
|---|---|---|
| Product | Engine + API (`POST /evaluate`, rules, typologies). Two modes: `detection` (weighted fraud rules) and `compliance` (rules grouped into typologies). | Full case-management product is described on the homepage (“Cases / Investigation”) but **no shipped Cases product is published** in the GitHub org (2 public repos: `osprey` and `.github`). |
| License | Apache License 2.0 on the repo and on the site (“Apache 2.0 licensed”). Schema.org on the homepage also points at Apache-2.0. | Commercial add-on license **not published**. |
| Pricing | Homepage comparison column: **“Open Source”** / **“Free, single binary”**. Homepage schema.org `Offer.price` = `0` USD. README “community / pro” are **runtime profiles** (SQLite+memory vs Postgres+Redis+NATS), not a price list. Docs: `OSPREY_TIER=enterprise` is **not enabled** in the open-source build. | Dollar price, seats, usage fees, support SKU, SLA **not published**. There is **no `/pricing` route** in the SPA (only `/` and `/slides/kutanapay`). |
| How you get it | Homepage CTA **“Get Started”** → `https://github.com/opensource-finance/osprey`. Clone + `go run` or local Docker sandbox. | No product signup, no hosted SaaS form, no billing. |
| Hosted / sandbox | Docs: “Osprey does not currently provide a maintained public sandbox URL.” `sandbox.osprey.opensource.finance` **does not resolve** (checked 19 Aug 2026). A PR describes a future Coolify sandbox; it is not live. | Hosted SaaS URL **not published**. |
| Company / brand | Site: “opensource.finance”, founder **Joseph Goksu**, email `joseph@opensource.finance` (on the KutanaPay slide deck). Sessionize: “Through his company **Reaktif**, he ships Osprey (opensource.finance)”. Site `reaktif.io` exists as a cloud-native consultancy page (`account@reaktif.io`). GitHub user company: `@reaktif-io`. | **Reaktif LLC** as an active legal entity: **not published** on the product site. UK Companies House lists **REAKTIV LTD** (14406957) as **Dissolved 6 August 2024**. No public “Reaktif LLC” filing was found in this pass. |
| Motif / naming | Brand on the live site and GitHub is **Osprey** + **opensource.finance**. Org created 6 Jan 2026; repo created 7 Jan 2026. | No public “Motif” product page or repo was found for this company. Unrelated third-party products also use the name Osprey. |

### Social proof and traction (only what is public)

| Signal | Public figure | Source / date |
|---|---|---|
| GitHub stars | **0** | GitHub repo page + org API, 19 Aug 2026 |
| GitHub forks | **0** | Same |
| GitHub watchers | **0** (API) | Org API, 19 Aug 2026 |
| Named customers on site | **None found.** Demo UI uses fictional “Nova Bank / Alice V.” | Homepage JS bundle |
| Customer logos / case studies | **None found** | Homepage; no `/customers` route |
| Funding | **Not published** | — |
| GitHub release | **v0.1.0** (github-actions; page shows “07 Jul”, assets: 7) | github.com/opensource-finance/osprey/releases |
| Hugging Face Narrator | **0 likes; 15 all-time downloads; 4 recent downloads** | huggingface.co/josephgoksu/osprey-narrator-v0.1 (page JSON, 19 Aug 2026) |
| Ollama Narrator | Page shows **“1 Download”**, model updated “6 months ago”, 2.5 GB Q4_K_M | ollama.com/josephgoksu/osprey-narrator |
| Talks | AWS Community Day Türkiye, 9 May 2026, Istanbul — “Building an AI Agent That Detects Financial Crime on AWS” | josephgoksu.com/talks/aws-community-day-turkiye-2026 |
| Press / analyst coverage of Osprey | **None found** in 2025–2026 web search | Search, 19 Aug 2026 |

**Do not treat as a customer or lead:** the site ships a pitch deck at `/slides/kutanapay` using tenant id `kutanapay` and UK–Africa corridor questions. That is founder-authored sales material. Kutana Pay is a real UK–Africa B2B payments firm (kutanapay.com); **no public statement that they use or evaluated Osprey was found.**

### Site claims that are *claims*, not independently verified here

- “PaySim benchmark: **96% recall** on Kaggle fraud dataset” — homepage only; method, split, and paper **not published** on the pages read.
- “Verified in **118ms**” — demo chrome on the homepage, not a published benchmark.
- Comparison table prices for Unit21 (`$30k–$740k/yr`) and “enterprise vendor `$50K–$500K+/year`” — **Osprey’s own table**, not the vendors’ public price lists (those vendors do not publish list prices).
- “Tazama … **43+ Docker containers, 8 GB RAM**”; “**33** rules, **2 of 33** source-visible” — Osprey slide copy, sourced by them to “github.com/tazama-lf … Data as of March 2026.” Treat as their comparison, not a Tazama-published spec.
- Homepage feature “Universal Adapters: ISO 20022, JSON, GraphQL, gRPC” — **the published API is JSON HTTP** (`POST /evaluate`). ISO 20022 / GraphQL / gRPC adapters were **not found** in the public README or sandbox guide.
- Homepage pillars **Studio** (no-code rule builder) and **Cases** (investigation workflow) — described as product surfaces; **no public Studio/Cases repos or docs** in the org. Narrator *is* published separately.

---

## ICP hypothesis (tied to site copy)

The copy is **not written for banks**.

| Audience | Evidence on the site / repo | Weight |
|---|---|---|
| **Primary: small / early fintechs that move money and cannot buy enterprise TM** | “everyone who isn’t a bank”; “Existing solutions were built for banks with unlimited budgets. We built for the rest of us.”; “out of reach for the fintechs I cared about”; “1 Developer” vs “Platform Team”; meta description: “for fintechs”. | Strong |
| **Secondary: cross-border / corridor payment firms (UK–Africa, escrow, wallets, settlement)** | Only extra route is `/slides/kutanapay`. Questions: “How do your cross-border flows work today? (escrow, wallets, settlement)”; “AML/compliance setup for UK-Africa corridors”; “stablecoins or blockchain rails”; “What does the FCA expect from you as you scale volume?” Next steps: “rule pack for UK-Africa payment corridors”, “2-week proof of concept”. | Strong (pitch-specific) |
| **Crypto / e-commerce** | Homepage `<meta name="description">`: “for fintechs, crypto, and e-commerce.” GitHub topics: `aml`, `fintech`, `fraud-detection`. No crypto-specific product page, chain analytics, or Travel Rule feature. | Weak — meta only |
| **Neo-banks** | **Not named** on the site. | Not supported |
| **Banks / central banks / national switches** | Explicitly the *anti*-ICP. Tazama is described as the heavy platform for that world (“national payment infrastructure”, “43+ containers”). Tazama’s own about page targets “central banks … banks, government agencies and financial institutions.” | Opposite |

**Buyer vs user:** CTA and docs are **engineer-first** (clone, Docker, CEL, admin token). Compliance language (FCA, FATF, FinCEN, auditors, SAR) is the *job-to-be-done*, but the person who can install it this week is a founding engineer / Head of Eng at a regulated-or-about-to-be-regulated payments firm. The economic buyer later is an MLRO / Head of Compliance. The site does not yet speak that buyer’s language (policy pack, SAR workflow, model risk, vendor DD).

**Commercial model from the artifacts:** **open-source self-serve engine** (Apache 2.0, price 0 in schema, GitHub CTA). A paid SaaS is **not published**. `community` / `pro` look like a future commercial split (SQLite vs Postgres/Redis/NATS) but are documented only as runtime profiles.

---

## Competitor map

Category: **transaction monitoring** sitting on the AML/CFT + fraud line. Adjacent: sanctions/PEP screening (not what Osprey ships), crypto KYT, case management, SAR narrative.

Osprey’s own homepage table compares: Osprey vs Tazama vs Sardine vs Unit21 vs ComplyAdvantage.

| Name | URL | Who they sell to (their copy) | How Osprey differs (from public pages) |
|---|---|---|---|
| **Tazama** | https://www.tazama.org/about/ · https://github.com/tazama-lf | Central banks, payment-scheme operators, banks, government agencies; LF Charities + Gates; Digital Public Good (Sep 2024). Full stack: ingest → typology scoring → triage → case management. | Closest ancestor. Osprey’s pitch: same *kind* of detection, **one Go binary / 60-second deploy** vs Tazama’s platform footprint. Osprey has no published case-management / national-switch deployment. |
| **Marble (Checkmarble)** | https://docs.checkmarble.com/docs/welcome-to-marble · https://github.com/checkmarble/marble | “Banks, Fintechs, Crypto exchange and any company moving money.” OSS core + licensed enterprise; TM, screening, case investigation, no-code rules. Positions vs ComplyAdvantage / Actimize / Fiserv. | Marble is a **full platform** (data model, screening, cases, AI review). Osprey is an **evaluate API + CEL rules**. Marble has a commercial motion; Osprey has no published paid SKU. |
| **Jube** | https://jube.io/learn-more/ · https://github.com/jube-home/aml-fraud-transaction-monitoring | “Financial institutions and fintechs”; real-time TM + ML + **workflow case management**. AGPLv3, Docker/K8s. | Broader product (ML + cases). Copyleft vs Osprey’s Apache 2.0. Heavier runtime (Postgres, Redis, .NET) vs single binary. |
| **ComplyAdvantage** | https://complyadvantage.com/ | Financial-services firms; SaaS “Mesh” risk intelligence; screening + TM + agentic alert handling. Named customer story: Monex. | Hosted SaaS + proprietary data. Osprey: self-host, no screening database, no published case tool. |
| **Unit21** | https://www.unit21.ai/ · https://www.unit21.ai/products/aml-transaction-monitoring | “Leading fintechs and financial institutions”; fraud + AML + sanctions + filing in one platform. | Full ops platform (agents, cases, filings). Osprey is detection-only. Unit21 list price **not published** (Osprey’s table cites `$30k–$740k/yr` — treat as Osprey’s claim). |
| **Flagright** | https://www.flagright.com/transaction-monitoring · https://flagright.com/ | Fintechs, banks, crypto/stablecoin, payment processors. No-code rules + ML + case/AI forensics. Their copy: “100+ regulated institutions” / “100 financial institutions across 30+ countries” (vendor claim). | Same wedge (fast rules) but **SaaS + screening + investigations**. Osprey has no published no-code UI (Studio is copy only). |
| **Scorechain** | https://www.scorechain.com/ · https://www.scorechain.com/products/transaction-monitoring | Crypto businesses, VASPs, FIs doing digital assets; on-chain KYT, wallet risk, Travel Rule / MiCA language. | Osprey has **no chain analytics**. Crypto on Osprey is a meta-tag and a corridor question about stablecoins, not a product. |
| **OpenSanctions** (adjacent) | https://www.opensanctions.org/docs/monitoring/ | Screening data + `yente` matching; not a TM or case-management product. Commercial data license for business use. | Complementary, not a substitute. Osprey does not ship sanctions/PEP lists. A real AML stack would still need a screening source. |

**Also on Osprey’s own comparison table:** [Sardine](https://www.sardine.ai/) — agentic fraud + AML SaaS for banks, merchants, fintechs (device/behaviour + TM). Same difference: hosted platform vs self-hosted engine.

---

## Sales-cycle reality for a solo founder

This is a **regulated control**, not a developer tool that closes on GitHub stars.

1. **The buyer is not the installer.** An engineer can `docker run` in an hour. An MLRO cannot put Osprey in a live AML program without: a documented risk-based rule set, typology mapping they will defend to the FCA (or equivalent), alert handling, SAR/STR process, and an audit trail a supervisor can replay. Joseph’s own talk states SARs are legal documents and the model must not decide guilt or file alone.

2. **Missing control-plane features lengthen every deal.** Public engine = evaluate + rules + typologies + health. Not published as product: case management, four-eye review, maker-checker, sanctions screening, customer risk rating, SAR filing, model-risk / backtesting UI, SOC 2 / ISO 27001, DPA, insurance, 24/7 support. Those are standard vendor-DD questions from even a 10-person EMI.

3. **Trust artifacts are thin.** 0 stars, 0 named customers, no public sandbox, v0.1.0, solo maintainer, UK company on Companies House is a **dissolved** LTD. A compliance committee will treat this as “founder project / shadow-mode PoC,” not a production TM vendor. That is rational, not hostility.

4. **Cycle length (industry, not a published Osprey metric):** first-line TM vendor selection at a licensed firm is typically **months** (security questionnaire, legal, data-residency, model validation, parallel run). Osprey’s own table says Unit21 deployment is “Months (Sales).” A 2-week technical PoC (as on the KutanaPay slides) can prove the API; it cannot prove an AML program.

5. **Open source is a wedge, not the close.** Apache 2.0 + on-prem data control is a real differentiator vs SaaS-only vendors (the site says “100% Yours”). The close still requires a human who will stand behind the rules in an FCA visit. A solo founder can sell **shadow-mode / pre-license / corridor-specific rule packs** much faster than “replace ComplyAdvantage.”

6. **Do not sell to banks first.** The copy already excludes them. Banks buy Tazama-class platforms or Actimize / NICE / SymphonyAI. The reachable ICP is a founder-led EMI, MSB, or corridor PSP that is writing its first TM policy and still has an engineer who will own the binary.

---

## 3 outreach drafts

**Status: drafts only. Do not send. Do not contact anyone.**

These assume a cold email from Joseph, from `joseph@opensource.finance`, with no fake social proof.

### 1) Compliance officer — first TM stack at a licensed / pre-license EMI or PSP

**Subject:** Shadow-mode TM you can read in CEL, not a 9-month vendor project

I built Osprey (opensource.finance) after working on Tazama, the Linux Foundation transaction-monitoring platform — I wanted the same FATF-style rules without a platform team. It is an Apache 2.0 Go service: you POST a transaction, CEL rules fire, you get ALRT/NALT plus a reason trail an auditor can read. I am not asking you to rip out whatever you use for screening or SAR filing; those are separate. If you are still on spreadsheets or a banking-sponsor’s delayed feed, I can load a starter typology pack and run a two-week shadow on a sample of your traffic so you can see false-positive rate and evidence quality before anyone calls it production. If useful, reply with how you handle TM today and which corridor or product is in scope.

### 2) Fintech founder — cross-border / escrow / wallet rails (UK–Africa or similar)

**Subject:** TM that deploys like a sidecar, not like a bank project

You are moving money across corridors (escrow, wallets, settlement) and the FCA conversation gets real as volume scales. Enterprise TM is sold as a platform; I shipped Osprey as a single binary you can stand up in an hour and point `POST /evaluate` at. Rules are CEL, typologies are FATF-inspired starters (structuring, velocity, same-party — you must tune them; they are not a policy). I am a solo founder — I will not pretend this replaces a full compliance ops suite. If you want a corridor-specific rule pack and a two-week PoC on shadow traffic, tell me how a payment moves from payer to payee today and I will say whether Osprey is even the right layer.

### 3) Crypto / VASP compliance

**Subject:** Fiat-side TM engine — not a Chainalysis replacement

Osprey is open-source transaction monitoring for the **off-chain** event (transfer, cash-out, wallet credit): CEL rules, FATF-style typologies, ALRT/NALT, Apache 2.0. It does not score wallets, hop chains, or implement the Travel Rule — if that is the whole job, Scorechain / Elliptic / TRM is the category, not me. Where I am useful is the fiat or ledger side that still has to sit in an AML program next to on-chain tools, especially if you cannot justify a Unit21/Flagright contract yet. If you already have KYT and still need a deterministic, inspectable rules engine for internal transfers and off-ramps, I can run a shadow evaluate on a redacted sample. Reply with what you use for on-chain vs ledger TM today; if Osprey is a mismatch I will say so.

---

## Gaps / what Joseph must confirm

1. **Legal entity.** Who contracts — Joseph personally, a new vehicle, or “Reaktif LLC”? UK REAKTIV LTD is dissolved. This blocks any paid pilot that needs a DPA / invoice.
2. **What is actually sold.** Is the motion (a) OSS adoption, (b) paid support / rule-pack services, (c) hosted `pro` (Postgres/Redis/NATS), or (d) Studio + Cases later? Public pages do not choose.
3. **Studio and Cases.** Homepage presents them as product. Confirm shipped vs mock. Selling them as available will backfire in a compliance demo.
4. **Universal adapters.** Confirm whether ISO 20022 / GraphQL / gRPC exist or are roadmap. The public API is JSON.
5. **PaySim “96% recall”.** Publish method or stop using the number in sales. Unsourced AML accuracy claims are a trust problem.
6. **Rule tenancy.** Sandbox docs: “Rules and typologies apply to every tenant” (`tenantId: "*"`). Confirm whether that is still true; it is a deal-breaker for any multi-tenant SaaS story.
7. **Any real design partner.** Internal only. Do not put Kutana Pay on a slide as a customer unless they have agreed in writing. The public deck already names them as the audience.
8. **Assurance pack for MLROs.** Need: data-flow diagram, what is / is not decided by the engine, starter-kit disclaimer (already in README), logging/retention, and a one-page “how an auditor replays an alert.”
9. **Narrator boundary.** Keep the talk’s line: model drafts, analyst files. Do not productize “auto-SAR.”
10. **Public sandbox.** Docs and DNS say it is not live. Either ship a token-gated demo or stop implying instant hosted try-out.

---

## Source list

All accessed 19 August 2026 unless noted.

### Product and founder
- https://opensource.finance/ — homepage HTML + JS bundle `/assets/index-DbajBjVP.js` (SPA routes: `/`, `/slides/kutanapay`)
- https://github.com/opensource-finance/osprey — README, stars/forks 0
- https://github.com/opensource-finance — org (UK, created 2026-01-06; 2 public repos)
- https://api.github.com/orgs/opensource-finance/repos — `stargazers_count: 0`, `forks_count: 0`, `created_at: 2026-01-07`, `pushed_at: 2026-08-18`, license Apache-2.0
- https://github.com/opensource-finance/osprey/releases — v0.1.0
- https://raw.githubusercontent.com/opensource-finance/osprey/main/README.md
- https://raw.githubusercontent.com/opensource-finance/osprey/main/docs/SANDBOX.md — no maintained public sandbox
- https://raw.githubusercontent.com/opensource-finance/osprey/main/docs/STARTER_KIT.md — 12 FATF-inspired rules, 6 typologies; “starting points”
- https://raw.githubusercontent.com/opensource-finance/osprey/main/docs/ARCHITECTURE.md — community/pro profiles; enterprise tier not enabled
- https://github.com/opensource-finance/osprey/pull/1 — sandbox/Coolify prep; public URL not in evidence
- https://josephgoksu.com/talks/aws-community-day-turkiye-2026/ — 9 May 2026 talk
- https://josephgoksu.com/blog/fine-tuning-llm-for-compliance-narratives/ — 14 Feb 2026; Narrator method
- https://josephgoksu.com/ — personal site
- https://sessionize.com/joseph-goksu/ — Reaktif ships Osprey; Tazama founding engineer; Obrizum
- https://huggingface.co/josephgoksu/osprey-narrator-v0.1 — 0 likes, 15 all-time downloads
- https://ollama.com/josephgoksu/osprey-narrator — “1 Download” on page
- https://reaktif.io/ — consultancy site
- https://find-and-update.company-information.service.gov.uk/company/14406957 — REAKTIV LTD dissolved 6 Aug 2024
- mailto on slides: joseph@opensource.finance

### Pitch deck (not a customer claim)
- https://opensource.finance/slides/kutanapay
- https://kutanapay.com/ — Kutana Pay is a real company; **no Osprey relationship published**

### Category / competitors
- https://www.tazama.org/about/
- https://www.linuxfoundation.org/press/linux-foundation-launches-tazama-for-real-time-fraud-management
- https://docs.checkmarble.com/docs/welcome-to-marble
- https://github.com/checkmarble/marble
- https://jube.io/learn-more/
- https://github.com/jube-home/aml-fraud-transaction-monitoring
- https://complyadvantage.com/
- https://www.unit21.ai/
- https://www.unit21.ai/products/aml-transaction-monitoring
- https://www.flagright.com/transaction-monitoring
- https://flagright.com/
- https://www.scorechain.com/
- https://www.scorechain.com/products/transaction-monitoring
- https://www.opensanctions.org/docs/monitoring/
- https://www.sardine.ai/
- https://salv.com/blog/best-aml-software/ — 2025/2026 AML vendor landscape (context only)

### Checked and empty
- Web search for “opensource.finance Osprey pricing”, Motif-branded Osprey, and press mentions: **no additional product/pricing/customer pages found**.
- `https://sandbox.osprey.opensource.finance` — DNS does not resolve.
- `www.opensource.finance` — DNS does not resolve.
- SPA paths `/pricing`, `/docs`, `/about`, `/blog` return the same client shell; they are **not** separate content pages.

---

*End of brief. Outreach unsent.*
