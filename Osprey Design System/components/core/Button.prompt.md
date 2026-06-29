Primary action button — Swiss-red `primary` by default, pill-shaped to match the marketing CTAs.

```jsx
<Button variant="primary" size="lg">Get Started</Button>
<Button variant="outline">View on GitHub →</Button>
<Button variant="ghost" size="sm">Cancel</Button>
```

Variants: `primary` (red CTA), `secondary` (grey fill), `outline` (hairline border), `ghost` (text-only, hover fill), `destructive`, `link`. Sizes `sm | md | lg`. Set `shape="rounded"` for app/console contexts; default `shape="pill"` for marketing. Use one `primary` per view — the red accent is precious.
