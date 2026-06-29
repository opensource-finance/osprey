/* Osprey Narrator — SAR-narrative console.
   Grounded in MODEL_CARD.md: alert JSON in → structured SAR narrative out
   (Alert Summary · Transaction Details · Risk Assessment · Rules · Typologies · Narrative · Actions). */
const NS = window.OpensourceFinanceOspreyDesignSystem_08d2ca;
const { Button, Badge, Eyebrow, Textarea, OspreyMark, Wordmark } = NS;

const ALERT_JSON = `{
  "decision": "ALRT",
  "composite_score": 724.50,
  "threshold": 500.0,
  "transaction": {
    "type": "wire_transfer",
    "amount": 147500.00,
    "currency": "USD",
    "originator": { "name": "Atlas Global Ventures", "country": "AE" },
    "beneficiary": { "name": "Pinnacle Commodities Ltd.", "country": "NG" }
  },
  "alert_id": "eval-7a3b2c1d-4e5f-6a7b-8c9d-0e1f2a3b4c5d"
}`;

const RULES = [
  ["FATF-R001", "High-Value Transaction", 0.875, true],
  ["FATF-R002", "Structuring Detection", 0.120, false],
  ["FATF-R003", "Rapid Movement of Funds", 0.720, true],
  ["FATF-R004", "Geographic Risk", 0.810, true],
  ["FATF-R005", "Unusual Transaction Pattern", 0.640, true],
  ["FATF-R006", "Shell Company Indicator", 0.690, true],
  ["FATF-R007", "PEP Transaction", 0.080, false],
  ["FATF-R008", "Round Amount Transaction", 0.550, true],
  ["FATF-R009", "Cross-Border Wire Transfer", 0.780, true],
  ["FATF-R010", "Dormant Account Activity", 0.040, false],
  ["FATF-R011", "Currency Exchange Anomaly", 0.210, false],
  ["FATF-R012", "Third-Party Payment", 0.190, false],
];

const TYPOLOGIES = [
  ["FATF-T001", "Structuring / Smurfing", "No match"],
  ["FATF-T002", "Trade-Based Money Laundering", "Match"],
  ["FATF-T003", "Shell Company Layering", "Match"],
  ["FATF-T004", "PEP Corruption Proceeds", "No match"],
  ["FATF-T005", "Funnel Account Activity", "Partial"],
  ["FATF-T006", "Currency Exchange Laundering", "No match"],
];

const matchTone = { "Match": { c: "var(--destructive)", b: "color-mix(in oklch,var(--destructive) 12%,transparent)" }, "Partial": { c: "#b45309", b: "#fffbeb" }, "No match": { c: "var(--text-muted)", b: "var(--surface-sunken)" } };

/* ---------- report ---------- */
function SectionTitle({ n, children }) {
  return (
    <h3 style={{ display: "flex", alignItems: "center", gap: 10, fontSize: "var(--text-xs)", fontWeight: 700, textTransform: "uppercase", letterSpacing: "var(--tracking-wider)", color: "var(--text-muted)", margin: "0 0 14px" }}>
      <span style={{ display: "inline-flex", alignItems: "center", justifyContent: "center", width: 20, height: 20, borderRadius: 6, background: "var(--primary-soft)", color: "var(--primary)", fontSize: "var(--text-2xs)" }}>{n}</span>
      {children}
    </h3>
  );
}
const Block = ({ children, last }) => <div style={{ paddingBottom: last ? 0 : 28, marginBottom: last ? 0 : 28, borderBottom: last ? "none" : "1px solid var(--border-soft)" }}>{children}</div>;

function KV({ k, v, accent }) {
  return (
    <div style={{ display: "flex", justifyContent: "space-between", gap: 16, padding: "7px 0", fontSize: "var(--text-sm)", borderBottom: "1px solid var(--border-soft)" }}>
      <span style={{ color: "var(--text-muted)" }}>{k}</span>
      <span style={{ fontWeight: 600, color: accent ? "var(--primary)" : "var(--foreground)", fontFamily: k.includes("ID") ? "var(--font-mono)" : "inherit", fontSize: k.includes("ID") ? "var(--text-xs)" : "var(--text-sm)" }}>{v}</span>
    </div>
  );
}

function SarReport() {
  const triggered = RULES.filter((r) => r[3]).length;
  return (
    <div style={{ maxWidth: 720, margin: "0 auto" }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", flexWrap: "wrap", gap: 12, marginBottom: 24 }}>
        <div>
          <Eyebrow tone="accent">Suspicious Activity Report — Draft</Eyebrow>
          <h2 style={{ fontSize: "var(--text-3xl)", fontWeight: 500, letterSpacing: "var(--tracking-tight)", margin: "6px 0 0" }}>Atlas Global Ventures → Pinnacle Commodities</h2>
        </div>
        <Badge variant="solid">ALRT</Badge>
      </div>

      <Block>
        <SectionTitle n="1">Alert Summary</SectionTitle>
        <KV k="Alert ID" v="eval-7a3b2c1d-…-0e1f2a3b4c5d" />
        <KV k="Transaction Type" v="wire_transfer" />
        <KV k="Amount" v="147,500.00 USD" />
        <KV k="Decision" v="ALRT (Alert Generated)" accent />
        <KV k="Composite Score" v="724.50  ·  threshold 500.0" accent />
      </Block>

      <Block>
        <SectionTitle n="2">Transaction Details</SectionTitle>
        <p style={{ margin: 0, fontSize: "var(--text-base)", lineHeight: "var(--leading-relaxed)", color: "var(--text-body)" }}>
          A wire transfer of <b>147,500.00 USD</b> was initiated by <b>Atlas Global Ventures</b> (AE) to <b>Pinnacle Commodities Ltd.</b> (NG). The transaction involves a cross-border movement between two higher-risk jurisdictions, settled in a single round-figure instruction.
        </p>
      </Block>

      <Block>
        <SectionTitle n="3">Risk Assessment</SectionTitle>
        <p style={{ margin: 0, fontSize: "var(--text-base)", lineHeight: "var(--leading-relaxed)", color: "var(--text-body)" }}>
          The composite risk score of <b>724.50</b> significantly exceeds the alert threshold of 500.0, indicating elevated risk. Seven of twelve FATF rules were triggered, with <b>High-Value Transaction</b> (0.875) and <b>Geographic Risk</b> (0.810) contributing the highest individual scores.
        </p>
      </Block>

      <Block>
        <SectionTitle n="4">Rules Triggered <span style={{ textTransform: "none", letterSpacing: 0, color: "var(--primary)", marginLeft: 4 }}>· {triggered}/12</span></SectionTitle>
        <div style={{ overflow: "hidden", borderRadius: "var(--radius-md)", border: "1px solid var(--border)" }}>
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: "var(--text-sm)" }}>
            <thead><tr style={{ background: "var(--surface-sunken)" }}>
              <th style={{ textAlign: "left", padding: "8px 12px", fontWeight: 600, color: "var(--text-muted)", fontSize: "var(--text-xs)" }}>Rule</th>
              <th style={{ textAlign: "right", padding: "8px 12px", fontWeight: 600, color: "var(--text-muted)", fontSize: "var(--text-xs)" }}>Score</th>
              <th style={{ textAlign: "right", padding: "8px 12px", fontWeight: 600, color: "var(--text-muted)", fontSize: "var(--text-xs)" }}>Triggered</th>
            </tr></thead>
            <tbody>
              {RULES.map((r) => (
                <tr key={r[0]} style={{ borderTop: "1px solid var(--border-soft)", opacity: r[3] ? 1 : 0.5 }}>
                  <td style={{ padding: "8px 12px" }}><span style={{ fontFamily: "var(--font-mono)", fontSize: "var(--text-xs)", color: "var(--text-muted)" }}>{r[0]}</span> {r[1]}</td>
                  <td style={{ padding: "8px 12px", textAlign: "right", fontFamily: "var(--font-mono)", fontWeight: 600, color: r[2] >= 0.6 ? "var(--primary)" : "var(--foreground)" }}>{r[2].toFixed(3)}</td>
                  <td style={{ padding: "8px 12px", textAlign: "right" }}>{r[3]
                    ? <span style={{ color: "var(--destructive)", fontWeight: 600 }}>Yes</span>
                    : <span style={{ color: "var(--text-muted)" }}>No</span>}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Block>

      <Block>
        <SectionTitle n="5">Typology Analysis</SectionTitle>
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {TYPOLOGIES.map((t) => {
            const tone = matchTone[t[2]];
            return (
              <div key={t[0]} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, padding: "10px 14px", borderRadius: "var(--radius-sm)", border: "1px solid var(--border-soft)", background: "var(--surface-card)" }}>
                <span style={{ fontSize: "var(--text-sm)" }}><span style={{ fontFamily: "var(--font-mono)", fontSize: "var(--text-xs)", color: "var(--text-muted)" }}>{t[0]}</span> {t[1]}</span>
                <span style={{ fontSize: "var(--text-xs)", fontWeight: 600, padding: "3px 10px", borderRadius: 999, color: tone.c, background: tone.b }}>{t[2]}</span>
              </div>
            );
          })}
        </div>
      </Block>

      <Block>
        <SectionTitle n="6">Narrative</SectionTitle>
        <p style={{ margin: 0, fontSize: "var(--text-base)", lineHeight: "var(--leading-relaxed)", color: "var(--text-body)" }}>
          The structure of this transaction is consistent with <b>trade-based money laundering</b> layered through a likely <b>shell-company</b> arrangement. A newly active corporate originator in the UAE remitting a large, round-figure sum to a commodities entity in Nigeria — absent supporting trade documentation — mirrors funnel-account behaviour. Combined with the geographic and rapid-movement signals, the pattern warrants escalation.
        </p>
      </Block>

      <Block last>
        <SectionTitle n="7">Recommended Actions</SectionTitle>
        <ul style={{ margin: 0, paddingLeft: 18, fontSize: "var(--text-base)", lineHeight: "var(--leading-relaxed)", color: "var(--text-body)", display: "flex", flexDirection: "column", gap: 8 }}>
          <li>Escalate to a senior compliance officer for SAR filing review.</li>
          <li>Request enhanced due diligence (EDD) on both counterparties.</li>
          <li>Investigate related transactions within the past 90 days.</li>
          <li>Hold settlement pending documentary evidence of the underlying trade.</li>
        </ul>
      </Block>
    </div>
  );
}

Object.assign(window, { SarReport, ALERT_JSON });
