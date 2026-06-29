# Osprey Narrator UI Kit — SAR Narrative Console

A console for **Osprey Narrator** (`opensource-finance/osprey-narrator`). Note: Narrator ships as a HuggingFace LoRA adapter / Ollama GGUF — it has **no shipped GUI**. This kit renders the *documented* output structure from `MODEL_CARD.md` inside a plausible reviewer console; it is grounded in source, not a recreation of an existing screen.

**Run:** open `index.html`. Paste/keep the sample Osprey alert JSON, press **Generate narrative**, watch a simulated inference pass, then read the structured SAR draft.

**The SAR narrative follows the model card's 7 sections:** Alert Summary · Transaction Details · Risk Assessment · Rules Triggered (the 12 FATF rules) · Typology Analysis (the 6 typologies) · Narrative · Recommended Actions. The worked example (ALRT, score 724.50, $147,500 AE→NG wire) is the one documented in `MODEL_CARD.md`.

- `narrator.jsx` — `SarReport` (the report) + canonical rule/typology/alert data.
- `app.jsx` — `Topbar`, input `Console`, inference states.

Consumes `Button`, `Badge`, `Eyebrow`, `Textarea`, `OspreyMark`, `Wordmark` from the DS bundle.
