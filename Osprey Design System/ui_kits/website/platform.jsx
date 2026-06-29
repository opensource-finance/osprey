/* Comparison table, platform vision, founder story.
   Recreation of comparison.tsx / platform-vision.tsx / founder-story.tsx. */
const { Badge: PBadge, Eyebrow: PEyebrow, Avatar: PAvatar } = window.OpensourceFinanceOspreyDesignSystem_08d2ca;

const Check = ({ c = "var(--sys-green)" }) => <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke={c} strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>;
const Cross = () => <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--text-muted)" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>;

function Comparison() {
  const rows = [
    ["Deployment", "60 Seconds", "Weeks (K8s)", "Immediate (API)", "Months (Sales)", "Immediate (API)"],
    ["Architecture", "Single Binary", "7+ Microservices", "SaaS Only", "SaaS Only", "SaaS Only"],
    ["Team Required", "1 Developer", "Platform Team", "Eng + Compliance", "Eng + Compliance", "Eng + Compliance"],
    ["Pricing Model", "Open Source", "High Ops Cost", "Vol. Commit", "$30k–$740k/yr", "Tiered / Custom"],
  ];
  const cols = ["Tazama", "Sardine", "Unit21", "ComplyAdvantage"];
  const th = { padding: "20px 16px", fontSize: "var(--text-lg)", fontWeight: 500, color: "var(--foreground)", borderBottom: "1px solid var(--border)", textAlign: "left" };
  const td = { padding: "20px 16px", color: "var(--text-muted)", borderBottom: "1px solid var(--border-soft)", fontSize: "var(--text-lg)" };
  const tdO = { padding: "20px 16px", color: "var(--primary)", fontWeight: 700, background: "color-mix(in srgb,var(--white) 50%,transparent)", borderLeft: "1px solid color-mix(in oklch,var(--primary) 10%,transparent)", borderRight: "1px solid color-mix(in oklch,var(--primary) 10%,transparent)", borderBottom: "1px solid color-mix(in oklch,var(--primary) 10%,transparent)", fontSize: "var(--text-lg)", position: "relative" };
  const lead = { position: "sticky", left: 0, padding: "20px 16px", fontWeight: 500, background: "var(--background)", borderBottom: "1px solid var(--border-soft)" };
  return (
    <section id="comparison" style={{ padding: "96px 0", background: "var(--surface-sunken)" }}>
      <div style={{ maxWidth: "var(--container-wide)", margin: "0 auto", padding: "0 24px" }}>
        <div style={{ marginBottom: 56 }}>
          <h2 style={{ fontSize: "var(--text-4xl)", fontWeight: 500, letterSpacing: "var(--tracking-tight)", margin: "0 0 16px" }}>The Uncomfortable Truth</h2>
          <p style={{ fontSize: "var(--text-xl)", color: "var(--text-muted)", maxWidth: 560, margin: 0 }}>
            Existing solutions were built for banks with unlimited budgets. We built for the rest of us.
          </p>
        </div>
        <div style={{ overflowX: "auto" }}>
          <table style={{ width: "100%", minWidth: 920, borderCollapse: "separate", borderSpacing: 0, textAlign: "left" }}>
            <thead>
              <tr>
                <th style={{ ...th, fontSize: "var(--text-xs)", textTransform: "uppercase", letterSpacing: "var(--tracking-wider)", color: "var(--text-muted)", position: "sticky", left: 0, background: "var(--background)" }}>Feature</th>
                <th style={{ ...th, color: "var(--primary)", fontSize: "var(--text-xl)", fontWeight: 700, background: "color-mix(in srgb,var(--white) 50%,transparent)", borderTop: "4px solid var(--primary)", borderLeft: "1px solid color-mix(in oklch,var(--primary) 10%,transparent)", borderRight: "1px solid color-mix(in oklch,var(--primary) 10%,transparent)", borderRadius: "12px 12px 0 0" }}>
                  <span style={{ display: "inline-flex", alignItems: "center", gap: 8 }}>Osprey <PBadge variant="soft">Dev</PBadge></span>
                </th>
                {cols.map((c) => <th key={c} style={{ ...th, opacity: 0.6 }}>{c}</th>)}
              </tr>
            </thead>
            <tbody>
              {rows.map((r, ri) => (
                <tr key={ri}>
                  <td style={lead}>{r[0]}</td>
                  <td style={ri === rows.length - 1 ? { ...tdO, borderRadius: "0 0 12px 12px" } : tdO}>{r[1]}</td>
                  {r.slice(2).map((c, ci) => <td key={ci} style={td}>{c}</td>)}
                </tr>
              ))}
              <tr>
                <td style={{ ...lead, borderBottom: "none" }}>Data Control</td>
                <td style={{ ...tdO, borderRadius: "0 0 12px 12px", borderBottom: "1px solid color-mix(in oklch,var(--primary) 10%,transparent)" }}><span style={{ display: "inline-flex", alignItems: "center", gap: 8 }}><Check />100% Yours</span></td>
                <td style={{ ...td, borderBottom: "none" }}><span style={{ display: "inline-flex", alignItems: "center", gap: 8 }}><Check c="var(--text-muted)" />100% Yours</span></td>
                <td style={{ ...td, borderBottom: "none" }}><span style={{ display: "inline-flex", alignItems: "center", gap: 8 }}><Cross />Vendor Lock-in</span></td>
                <td style={{ ...td, borderBottom: "none" }}><span style={{ display: "inline-flex", alignItems: "center", gap: 8 }}><Cross />Vendor Lock-in</span></td>
                <td style={{ ...td, borderBottom: "none" }}><span style={{ display: "inline-flex", alignItems: "center", gap: 8 }}><Cross />Vendor Lock-in</span></td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}

function PlatformVision() {
  const tiles = [
    { letter: "S", title: "Studio", sub: "Policy & Rules", body: "No-code builder for defining risk controls and typologies without engineering bottlenecks.", tone: "muted", chip: "Design", chipBg: "#eff6ff", chipFg: "#1d4ed8", lc: "#2563eb", lbg: "#eff6ff" },
    { letter: "O", title: "Osprey", sub: "The Engine", body: "High-performance real-time transaction monitoring and screening.", tone: "core", chip: "Core" },
    { letter: "C", title: "Cases", sub: "Investigation", body: "Streamlined workflow for analysts to review, decision, and report suspicious activity.", tone: "muted", chip: "Action", chipBg: "#fffbeb", chipFg: "#b45309", lc: "#d97706", lbg: "#fffbeb" },
  ];
  return (
    <section id="vision" style={{ padding: "96px 24px", borderBottom: "1px solid var(--border-soft)" }}>
      <div style={{ maxWidth: "var(--container-content)", margin: "0 auto" }}>
        <div style={{ marginBottom: 48 }}>
          <h2 style={{ fontSize: "var(--text-3xl)", fontWeight: 500, letterSpacing: "var(--tracking-tight)", margin: "0 0 16px" }}>The Platform Vision</h2>
          <p style={{ fontSize: "var(--text-lg)", color: "var(--text-muted)", maxWidth: 560, margin: 0 }}>
            Open-source infrastructure for the next generation of fintech. We're building the operating system for financial vigilance.
          </p>
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(240px,1fr))", gap: 24 }}>
          {tiles.map((t) => {
            const core = t.tone === "core";
            return (
              <div key={t.title} style={{ position: "relative", overflow: "hidden", borderRadius: "var(--radius-xl)", padding: 24,
                border: core ? "1px solid color-mix(in oklch,var(--primary) 20%,transparent)" : "1px solid var(--border)",
                background: core ? "var(--primary-soft)" : "color-mix(in srgb,var(--white) 50%,transparent)",
                boxShadow: core ? "var(--shadow-md)" : "var(--shadow-sm)",
                opacity: core ? 1 : 0.55, filter: core ? "none" : "grayscale(1) blur(0.4px)" }}>
                <div style={{ position: "absolute", top: 16, right: 16 }}>
                  <span style={{ fontSize: "var(--text-xs)", fontWeight: 500, padding: "4px 8px", borderRadius: 999,
                    background: core ? "var(--primary-soft)" : t.chipBg, color: core ? "var(--primary)" : t.chipFg,
                    boxShadow: core ? "inset 0 0 0 1px color-mix(in oklch,var(--primary) 20%,transparent)" : "none" }}>{t.chip}</span>
                </div>
                <div style={{ width: 40, height: 40, borderRadius: 10, marginBottom: 16, display: "flex", alignItems: "center", justifyContent: "center",
                  fontFamily: "var(--font-serif)", fontStyle: "italic", fontSize: "var(--text-xl)",
                  background: core ? "var(--primary)" : t.lbg, color: core ? "#fff" : t.lc }}>{t.letter}</div>
                <h3 style={{ fontSize: "var(--text-xl)", fontWeight: 700, letterSpacing: "var(--tracking-tight)", margin: "0 0 8px" }}>{t.title}</h3>
                <p style={{ fontSize: "var(--text-sm)", color: "var(--text-muted)", margin: "0 0 16px" }}>{t.sub}</p>
                <p style={{ fontSize: "var(--text-sm)", color: "var(--foreground)", opacity: 0.85, lineHeight: "var(--leading-relaxed)", margin: 0 }}>
                  {t.body}{core && <span style={{ fontWeight: 600, color: "var(--primary)" }}> In Development.</span>}
                </p>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}

function FounderStory() {
  const paras = [
    <>The journey began with the <b>LevelOne Project</b>, a <b>Gates Foundation</b> initiative to increase financial inclusion — a low-cost, real-time payment infrastructure for the world's unbanked.</>,
    <>I worked as a software engineer on <b>Tazama</b>, the transaction monitoring arm of that project. We built a robust, open-source framework to detect fraud across national payment switches.</>,
    <>I then moved to the private sector, architecting secure-by-design infrastructure for highly regulated environments — sovereign-grade security, landing-zone isolation, zero-trust as requirements, not features.</>,
    <>But I saw a gap. This level of rigor was completely out of reach for the fintechs I cared about. They were stuck with brittle integrations or expensive enterprise contracts.</>,
    <>I built <b>Osprey</b> to fulfill the original promise of equity — re-engineered into a single, efficient binary. No massive integration projects. No enterprise bloat.</>,
  ];
  return (
    <section id="story" style={{ padding: "96px 24px", borderTop: "1px solid var(--border-soft)", background: "var(--white)" }}>
      <div style={{ maxWidth: "var(--container-content)", margin: "0 auto", display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(300px,1fr))", gap: 48, alignItems: "start" }}>
        <div>
          <PBadge variant="soft" live>Origin Story</PBadge>
          <h2 style={{ fontSize: "clamp(2rem,4vw,3rem)", fontWeight: 500, letterSpacing: "var(--tracking-tight)", lineHeight: 1.1, margin: "24px 0 0" }}>
            We didn't start with a business plan. <span style={{ color: "var(--text-muted)" }}>We started with a mission.</span>
          </h2>
        </div>
        <div style={{ display: "flex", flexDirection: "column", gap: 24, fontSize: "var(--text-lg)", color: "var(--text-muted)", lineHeight: "var(--leading-relaxed)" }}>
          {paras.map((p, i) => <p key={i} style={{ margin: 0 }}>{p}</p>)}
          <div style={{ display: "flex", alignItems: "center", gap: 16, paddingTop: 8 }}>
            <PAvatar initials="JG" />
            <div><div style={{ fontWeight: 700, color: "var(--foreground)" }}>Joseph Goksu</div><div style={{ fontSize: "var(--text-sm)" }}>Founder, opensource.finance</div></div>
          </div>
        </div>
      </div>
      <style>{`#story b{color:var(--foreground);font-weight:500}`}</style>
    </section>
  );
}

Object.assign(window, { Comparison, PlatformVision, FounderStory });
