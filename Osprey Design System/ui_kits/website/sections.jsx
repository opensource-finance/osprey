/* opensource.finance landing — header, hero, credibility, features, footer.
   Faithful recreation of src/components/landing/*, composing DS primitives. */
const { Button, Badge, Eyebrow, FeatureCard, Wordmark, Avatar } = window.OpensourceFinanceOspreyDesignSystem_08d2ca;

/* --- inline Lucide icons (paths lifted from the source) --- */
const Svg = ({ children, w = 24, sw = 1.5 }) => (
  <svg xmlns="http://www.w3.org/2000/svg" width={w} height={w} viewBox="0 0 24 24" fill="none"
       stroke="currentColor" strokeWidth={sw} strokeLinecap="round" strokeLinejoin="round">{children}</svg>
);
const IconBox = () => <Svg><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/></Svg>;
const IconZap = () => <Svg><path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/></Svg>;
const IconLayers = () => <Svg><polygon points="12 2 2 7 12 12 22 7 12 2"/><polyline points="2 17 12 22 22 17"/><polyline points="2 12 12 17 22 12"/></Svg>;
const IconFeather = () => <Svg><path d="M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91-.09z"/><path d="m12 15-3-3a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 0 1-4 2z"/><path d="M9 12H4s.55-3.03 2-4c1.62-1.08 5 0 5 0"/><path d="M12 15v5s3.03-.55 4-2c1.08-1.62 0-5 0-5"/></Svg>;
const IconGitHub = () => <Svg sw={1.8}><path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"/></Svg>;

const wrap = { maxWidth: "var(--container-content)", margin: "0 auto", padding: "0 24px" };

function Header() {
  const links = ["Features", "Vision", "Comparison", "Story"];
  return (
    <header style={{ position: "sticky", top: 24, zIndex: 50, width: "100%", display: "flex", justifyContent: "center", padding: "0 16px" }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 16, width: "100%", maxWidth: 768,
        borderRadius: "var(--radius-pill)", border: "1px solid var(--border-soft)", background: "color-mix(in srgb, var(--background) 65%, transparent)",
        backdropFilter: "blur(var(--blur-glass))", padding: "8px 8px 8px 22px", boxShadow: "var(--shadow-sm)" }}>
        <Wordmark brand="osf" dot="circle" size={15} />
        <nav style={{ display: "flex", gap: 24, fontSize: "var(--text-sm)", fontWeight: 500, color: "var(--text-muted)" }}>
          {links.map((l) => <a key={l} href={"#" + l.toLowerCase()} style={{ color: "inherit", textDecoration: "none" }}
            onMouseEnter={(e)=>e.currentTarget.style.color="var(--foreground)"} onMouseLeave={(e)=>e.currentTarget.style.color="var(--text-muted)"}>{l}</a>)}
        </nav>
        <Button size="sm">Get Started</Button>
      </div>
    </header>
  );
}

function Hero() {
  return (
    <section style={{ padding: "120px 24px 80px" }}>
      <div style={{ maxWidth: 960, margin: "0 auto", display: "flex", flexDirection: "column", gap: 24 }}>
        <div><Badge variant="soft" live>Introducing Osprey</Badge></div>
        <h1 style={{ fontSize: "clamp(3rem, 7vw, 5.5rem)", fontWeight: 500, letterSpacing: "var(--tracking-tighter)", lineHeight: 1.05, maxWidth: 900, margin: 0 }}>
          Transaction monitoring for <span style={{ fontFamily: "var(--font-serif)", fontStyle: "italic", fontWeight: 400, color: "var(--primary)", paddingRight: "0.08em" }}>everyone</span> who isn't a bank.
        </h1>
        <p style={{ fontSize: "var(--text-2xl)", color: "var(--text-muted)", maxWidth: 640, lineHeight: "var(--leading-relaxed)", margin: 0 }}>
          The open-source infrastructure that deploys in 60 seconds. Single binary. Built in Go. No enterprise bloat.
        </p>
        <div style={{ display: "flex", flexWrap: "wrap", gap: 16, paddingTop: 8 }}>
          <Button size="lg">Get Started</Button>
          <Button size="lg" variant="outline">View on GitHub&nbsp;→</Button>
        </div>
      </div>
    </section>
  );
}

function Credibility() {
  return (
    <section style={{ padding: "48px 0", borderTop: "1px solid var(--border-soft)", borderBottom: "1px solid var(--border-soft)" }}>
      <div style={{ ...wrap, display: "flex", flexWrap: "wrap", gap: 32, alignItems: "center", justifyContent: "space-between" }}>
        <div>
          <Eyebrow>Engineered by the</Eyebrow>
          <div style={{ display: "flex", flexDirection: "column", marginTop: 6 }}>
            <span style={{ fontSize: "var(--text-2xl)", fontWeight: 700, letterSpacing: "var(--tracking-tight)" }}>Founding Team of Tazama</span>
            <span style={{ fontSize: "var(--text-sm)", color: "var(--text-muted)" }}>The original real-time transaction monitoring platform</span>
          </div>
        </div>
        <div style={{ width: 1, height: 48, background: "var(--border)" }} />
        <div>
          <Eyebrow>Heritage &amp; Validation</Eyebrow>
          <div style={{ display: "flex", flexDirection: "column", gap: 4, marginTop: 6 }}>
            <div><span style={{ fontWeight: 600 }}>Bill &amp; Melinda Gates Foundation</span> <span style={{ fontSize: "var(--text-xs)", color: "var(--text-muted)" }}>(Original Funding)</span></div>
            <div><span style={{ fontWeight: 600 }}>Linux Foundation</span> <span style={{ fontSize: "var(--text-xs)", color: "var(--text-muted)" }}>(Current Steward)</span></div>
          </div>
        </div>
      </div>
    </section>
  );
}

function Features() {
  const items = [
    { icon: <IconBox />, title: "Single Binary", body: "Compiled Go. No JVM. No node_modules. Just one standard binary that runs anywhere." },
    { icon: <IconZap />, title: "60 Second Deploy", body: "From download to monitoring live transactions in less time than it takes to brew coffee." },
    { icon: <IconLayers />, title: "Universal Adapters", body: "ISO 20022, JSON, GraphQL, gRPC. Whatever your payment rail speaks, we speak it." },
    { icon: <IconFeather />, title: "No Bloat", body: "We stripped out the enterprise complexity. You don't need a Kubernetes cluster to run fraud checks." },
  ];
  return (
    <section id="features" style={{ padding: "112px 24px" }}>
      <div style={{ maxWidth: "var(--container-content)", margin: "0 auto", display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(300px,1fr))", gap: 32 }}>
        {items.map((f) => <FeatureCard key={f.title} icon={f.icon} title={f.title}>{f.body}</FeatureCard>)}
      </div>
    </section>
  );
}

function Footer() {
  return (
    <footer style={{ padding: "48px 0", borderTop: "1px solid var(--border)" }}>
      <div style={{ ...wrap, display: "flex", flexWrap: "wrap", gap: 24, justifyContent: "space-between", alignItems: "center" }}>
        <Wordmark brand="osf" size={18} />
        <div style={{ display: "flex", gap: 32, fontSize: "var(--text-sm)", fontWeight: 500, color: "var(--text-muted)" }}>
          <a href="#" style={{ color: "inherit", textDecoration: "none", display: "inline-flex", alignItems: "center", gap: 6 }}><IconGitHub /> GitHub</a>
          <a href="#" style={{ color: "inherit", textDecoration: "none" }}>Docs</a>
          <a href="#" style={{ color: "inherit", textDecoration: "none" }}>Platform</a>
        </div>
        <div style={{ fontSize: "var(--text-sm)", color: "var(--text-muted)", opacity: 0.6 }}>© 2026 Open Source Finance.</div>
      </div>
    </footer>
  );
}

Object.assign(window, { Header, Hero, Credibility, Features, Footer, wrap });
