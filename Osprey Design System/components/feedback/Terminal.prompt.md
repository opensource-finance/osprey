The signature Osprey "god-view" terminal — a deep-slate glass panel showing a mono log stream and a final decision.

```jsx
<Terminal
  logs={[
    { text: "ORIGIN_IP: 192.168.1.42 (US-WEST)", status: "OK", tone: "ok" },
    { text: "DEVICE_SIG: SHA-256 MATCH", status: "OK", tone: "ok" },
    { text: "BEHAVIOR_MODEL: DEVIATION +0.7σ", status: "WARN", tone: "warn" },
    { text: "RISK_SCORE: 12.4 (THRESHOLD 85)", status: "OK", tone: "ok" },
  ]}
  decision={{ label: "FINAL_DECISION", value: "ALLOWED", tone: "ok" }}
/>
```

Use over the Apple-grey (`--sys-mist`) demo canvas. Keep log text terse and uppercase, FATF/engineering flavored. Green = OK, amber = WARN, blue = INFO.
