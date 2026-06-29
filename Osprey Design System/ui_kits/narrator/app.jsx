/* Narrator console shell — paste alert JSON, run inference, view the SAR narrative. */
const C = window.OpensourceFinanceOspreyDesignSystem_08d2ca;

/* brand illustrations (inline, currentColor) */
const ILL = {
  scrutiny: '<path d="M13 8 H27 L33 14 V40 H13 Z"/><path d="M27 8 V14 H33"/><path d="M18 18 H26 M18 22.5 H24"/><circle cx="26.5" cy="30" r="6"/><path d="M31 34.5 L36 39.5"/>',
  balance: '<path d="M24 11 V39"/><path d="M17 40 H31"/><path d="M12 16 H36"/><circle cx="24" cy="11" r="2.4" fill="currentColor"/><path d="M12 16 L7 26 M12 16 L17 26 M7 26 A6 6 0 0 0 17 26"/><path d="M36 16 L31 26 M36 16 L41 26 M31 26 A6 6 0 0 0 41 26"/>',
  network: '<path d="M24 24 L10 15 M24 24 L38 15 M24 24 L13 37 M24 24 L35 37"/><circle cx="10" cy="14" r="3.4"/><circle cx="38" cy="14" r="3.4"/><circle cx="12" cy="38" r="3.4"/><circle cx="36" cy="38" r="3.4"/><circle cx="24" cy="24" r="3.4" fill="currentColor"/>',
  records: '<path d="M24 14 C20 11 12 11 8 13 V36 C12 34 20 34 24 37 C28 34 36 34 40 36 V13 C36 11 28 11 24 14 Z"/><path d="M24 14 V37"/><path d="M12 19 H19 M12 24 H19 M29 19 H36 M29 24 H36"/>',
};
const Ill = ({ id, size = 28, color = "currentColor" }) => (
  <svg width={size} height={size} viewBox="0 0 48 48" fill="none" stroke={color} strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" dangerouslySetInnerHTML={{ __html: ILL[id] }} />
);

function Topbar() {
  return (
    <header style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 16, padding: "14px 24px", borderBottom: "1px solid var(--border)", background: "color-mix(in srgb,var(--background) 80%,transparent)", backdropFilter: "blur(var(--blur-glass))", position: "sticky", top: 0, zIndex: 50 }}>
      <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
        <C.OspreyMark size={22} />
        <span style={{ fontWeight: 700, letterSpacing: "var(--tracking-tight)", fontSize: "var(--text-lg)" }}>Osprey <span style={{ fontFamily: "var(--font-serif)", fontStyle: "italic", fontWeight: 400, color: "var(--primary)" }}>Narrator</span></span>
        <C.Badge variant="neutral">v0.1 · Qwen3-4B LoRA</C.Badge>
      </div>
      <div style={{ display: "flex", alignItems: "center", gap: 16, fontSize: "var(--text-xs)", color: "var(--text-muted)", fontFamily: "var(--font-mono)" }}>
        <span>PPL 2.65</span><span>·</span><span>12 rules · 6 typologies</span>
      </div>
    </header>
  );
}

function Console() {
  const [json, setJson] = React.useState(window.ALERT_JSON);
  const [phase, setPhase] = React.useState("idle"); // idle | running | done
  const run = () => {
    setPhase("running");
    setTimeout(() => setPhase("done"), 1400);
  };
  return (
    <div style={{ display: "grid", gridTemplateColumns: "minmax(320px, 420px) 1fr", minHeight: "calc(100vh - 56px)" }}>
      {/* input */}
      <aside style={{ borderRight: "1px solid var(--border)", padding: 24, display: "flex", flexDirection: "column", gap: 16, background: "var(--surface-sunken)" }}>
        <C.Eyebrow>Osprey alert · input</C.Eyebrow>
        <div style={{ display: "flex", gap: 8 }}>
          <C.Badge variant="soft">🤗 HuggingFace</C.Badge>
          <C.Badge variant="neutral">🦙 Ollama · Q4_K_M</C.Badge>
        </div>
        <C.Textarea mono rows={16} value={json} onChange={(e) => setJson(e.target.value)} style={{ flex: 1 }} />
        <C.Button fullWidth size="lg" onClick={run} disabled={phase === "running"}>
          {phase === "running" ? "Generating narrative…" : phase === "done" ? "Regenerate ↻" : "Generate narrative"}
        </C.Button>
        <p style={{ fontSize: "var(--text-xs)", color: "var(--text-muted)", margin: 0, lineHeight: "var(--leading-relaxed)" }}>
          From alert to narrative in seconds, not hours. Trained on synthetic data only — review before filing.
        </p>
      </aside>

      {/* output */}
      <main style={{ padding: "40px 32px", overflowY: "auto", background: "var(--background)" }}>
        {phase === "idle" && (
          <div style={{ height: "100%", minHeight: 420, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", textAlign: "center", gap: 20 }}>
            <div style={{ width: 96, height: 96, borderRadius: 28, background: "var(--primary-soft)", display: "flex", alignItems: "center", justifyContent: "center", color: "var(--primary)" }}>
              <Ill id="scrutiny" size={48} />
            </div>
            <div style={{ maxWidth: 380 }}>
              <h3 style={{ fontSize: "var(--text-xl)", fontWeight: 500, letterSpacing: "var(--tracking-tight)", margin: "0 0 6px", color: "var(--foreground)" }}>Awaiting an alert</h3>
              <p style={{ margin: 0, fontSize: "var(--text-base)", color: "var(--text-muted)", lineHeight: "var(--leading-relaxed)" }}>Paste an Osprey evaluation on the left and generate an analyst-ready SAR narrative in seconds.</p>
            </div>
            <div style={{ display: "flex", gap: 10, marginTop: 4 }}>
              {[["balance", "12 rules"], ["network", "6 typologies"], ["records", "Narrative"]].map(([id, t]) => (
                <div key={id} style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 8, width: 92, padding: "14px 8px", borderRadius: "var(--radius-md)", border: "1px solid var(--border-soft)", background: "var(--surface-card)", color: "var(--text-muted)" }}>
                  <Ill id={id} size={26} />
                  <span style={{ fontSize: "var(--text-xs)", fontFamily: "var(--font-mono)", letterSpacing: "0.04em" }}>{t}</span>
                </div>
              ))}
            </div>
          </div>
        )}
        {phase === "running" && (
          <div style={{ height: "100%", minHeight: 420, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 16, color: "var(--text-muted)" }}>
            <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="var(--primary)" strokeWidth="2.5" strokeLinecap="round" style={{ animation: "nar-spin .8s linear infinite" }}><path d="M12 2a10 10 0 0 1 10 10" /></svg>
            <span style={{ fontFamily: "var(--font-mono)", fontSize: "var(--text-sm)" }}>running inference · 1,024 tokens…</span>
          </div>
        )}
        {phase === "done" && <window.SarReport />}
      </main>
      <style>{`@keyframes nar-spin{to{transform:rotate(360deg)}}`}</style>
    </div>
  );
}

function NarratorApp() {
  return (
    <div style={{ fontFamily: "var(--font-sans)", color: "var(--foreground)", background: "var(--background)", minHeight: "100vh" }}>
      <Topbar />
      <Console />
    </div>
  );
}
ReactDOM.createRoot(document.getElementById("root")).render(<NarratorApp />);
