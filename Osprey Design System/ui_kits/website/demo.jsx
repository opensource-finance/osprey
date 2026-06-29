/* The "god view" — Osprey intercepting a transaction. Looping state machine.
   Recreation of src/components/landing/transaction-demo.tsx using CSS transitions. */
const { Terminal: OspreyTerminal, Eyebrow: DemoEyebrow } = window.OpensourceFinanceOspreyDesignSystem_08d2ca;


const FiSpinner = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round"
       style={{ animation: "osf-spin 0.8s linear infinite" }}><path d="M12 2a10 10 0 0 1 10 10"/></svg>
);
const FiCheck = ({ c = "var(--sys-green)" }) => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke={c} strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>
);
const FiShield = ({ s = 18, c = "currentColor" }) => (
  <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke={c} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>
);

const LOGS = [
  { text: "ORIGIN_IP: 192.168.1.42 (US-WEST)", status: "OK", tone: "ok" },
  { text: "DEVICE_SIG: SHA-256 MATCH", status: "OK", tone: "ok" },
  { text: "BEHAVIOR_MODEL: DEVIATION +0.7σ", status: "WARN", tone: "warn" },
  { text: "RISK_SCORE: 12.4 (THRESHOLD 85)", status: "OK", tone: "ok" },
];

function Phone({ children, dark, style }) {
  return (
    <div style={{ width: 280, height: 540, background: dark ? "var(--ink)" : "var(--white)", borderRadius: 40, padding: 12,
      boxShadow: "var(--shadow-2xl)", border: dark ? "none" : "1px solid var(--border)", flexShrink: 0, ...style }}>
      <div style={{ width: "100%", height: "100%", background: dark ? "#fff" : "var(--sys-mist)", borderRadius: 32, overflow: "hidden", display: "flex", flexDirection: "column" }}>
        {children}
      </div>
    </div>
  );
}

function SenderDevice({ step }) {
  return (
    <Phone dark>
      <div style={{ height: 56, display: "flex", alignItems: "center", justifyContent: "center", borderBottom: "1px solid #f1f5f9" }}>
        <span style={{ fontSize: "var(--text-xs)", fontWeight: 700, color: "var(--ink)" }}>Nova Bank</span>
      </div>
      <div style={{ padding: 24, flex: 1, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 32 }}>
        <div style={{ textAlign: "center" }}>
          <div style={{ width: 64, height: 64, borderRadius: "50%", background: "#f1f5f9", margin: "0 auto", display: "flex", alignItems: "center", justifyContent: "center", fontSize: 26 }}>👤</div>
          <p style={{ fontSize: "var(--text-sm)", fontWeight: 700, color: "var(--ink)", margin: "8px 0 0" }}>Alice V.</p>
        </div>
        <div style={{ textAlign: "center" }}>
          <p style={{ fontSize: "var(--text-2xs)", fontWeight: 700, textTransform: "uppercase", letterSpacing: "var(--tracking-wider)", color: "#94a3b8", margin: "0 0 4px" }}>Sending</p>
          <div style={{ fontSize: "2.25rem", fontWeight: 700, letterSpacing: "-0.02em", color: "var(--ink)" }}>$2,450.00</div>
        </div>
        <button style={{ width: "100%", height: 48, borderRadius: 12, border: "none", fontSize: "var(--text-sm)", fontWeight: 700, cursor: "default",
          display: "flex", alignItems: "center", justifyContent: "center", gap: 8, transition: "all .3s",
          background: step === 0 ? "#0a0a0a" : "#f1f5f9", color: step === 0 ? "#fff" : "#94a3b8" }}>
          {step === 0 && "Send Money"}
          {step === 1 && <><FiSpinner /><span>Sending…</span></>}
          {step === 2 && <span style={{ color: "#64748b" }}>Held for verification</span>}
          {step >= 3 && <><FiCheck /><span style={{ color: "var(--ink)" }}>Approved · Sent</span></>}
        </button>
      </div>
    </Phone>
  );
}

function ReceiverDevice({ step }) {
  return (
    <Phone>
      <div style={{ height: 56, display: "flex", alignItems: "center", justifyContent: "center", borderBottom: "1px solid #f1f5f9" }}>
        <span style={{ fontSize: "var(--text-xs)", fontWeight: 700, color: "#94a3b8" }}>Notifications</span>
      </div>
      <div style={{ padding: 16, flex: 1, display: "flex", flexDirection: "column", alignItems: "center", paddingTop: 48 }}>
        <div style={{ width: "100%", background: "#fff", padding: 16, borderRadius: 16, boxShadow: "var(--shadow-lg)", border: "1px solid #f1f5f9",
          transition: "all .4s var(--ease-out)", opacity: step >= 5 ? 1 : 0, transform: step >= 5 ? "translateY(0) scale(1)" : "translateY(-16px) scale(0.95)" }}>
          <div style={{ display: "flex", gap: 12 }}>
            <div style={{ width: 40, height: 40, borderRadius: "50%", background: "var(--sys-green)", display: "flex", alignItems: "center", justifyContent: "center", color: "#fff", flexShrink: 0 }}><FiShield c="#fff" /></div>
            <div>
              <p style={{ fontSize: "var(--text-xs)", fontWeight: 700, color: "var(--ink)", margin: 0 }}>Payment Received</p>
              <div style={{ display: "flex", alignItems: "center", gap: 6, marginTop: 2 }}>
                <FiShield s={10} c="var(--sys-green)" /><p style={{ fontSize: "var(--text-2xs)", color: "#64748b", margin: 0 }}>Protected transfer</p>
              </div>
              <p style={{ fontSize: "var(--text-lg)", fontWeight: 700, color: "var(--ink)", margin: "4px 0 0" }}>$2,450.00</p>
              <p style={{ fontSize: "9px", color: "#94a3b8", margin: "8px 0 0", fontFamily: "var(--font-mono)" }}>Verified in 118ms</p>
            </div>
          </div>
        </div>
      </div>
    </Phone>
  );
}

function Intercept({ step }) {
  const showTerminal = step === 2 || step === 3;
  return (
    <div style={{ flex: 1, minWidth: 280, maxWidth: 420, height: 240, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", position: "relative" }}>
      {/* the wire */}
      <div style={{ width: "100%", height: 6, background: "var(--grey-300)", borderRadius: 999, position: "relative", overflow: "hidden", marginBottom: 32 }}>
        <div style={{ position: "absolute", top: 0, bottom: 0, width: 96, borderRadius: 999,
          background: step === 4 ? "linear-gradient(90deg,transparent,var(--sys-green),transparent)" : "linear-gradient(90deg,transparent,var(--ink),transparent)",
          transition: "left 0.6s linear, opacity .3s",
          opacity: (step === 1 || step === 4) ? 1 : 0,
          left: step === 1 ? "50%" : step === 4 ? "200%" : "-100%" }} />
        {showTerminal && <div style={{ position: "absolute", top: 0, bottom: 0, left: "50%", transform: "translateX(-50%)", width: 16, background: "var(--ink)", borderRadius: 999 }} />}
      </div>
      {/* terminal */}
      <div style={{ position: "absolute", top: 0, left: "50%", transform: "translateX(-50%)", width: 340,
        transition: "opacity .4s var(--ease-out), transform .4s var(--ease-out)",
        opacity: showTerminal ? 1 : 0, transform: showTerminal ? "translateX(-50%) translateY(0)" : "translateX(-50%) translateY(8px)", pointerEvents: "none" }}>
        <OspreyTerminal logs={LOGS} decision={step === 3 ? { label: "FINAL_DECISION", value: "ALLOWED", tone: "ok" } : { label: "FINAL_DECISION", value: "ANALYZING…", tone: "muted" }} />
      </div>
      <div style={{ position: "absolute", top: 0, left: "50%", transform: "translate(-50%,-50%)", width: 16, height: 16, background: "var(--ink)", borderRadius: "50%", border: "2px solid var(--grey-300)" }} />
    </div>
  );
}

function TransactionDemo() {
  const [step, setStep] = React.useState(0);
  React.useEffect(() => {
    let t;
    const run = () => {
      setStep(1);
      t = setTimeout(() => { setStep(2);
        t = setTimeout(() => { setStep(3);
          t = setTimeout(() => { setStep(4);
            t = setTimeout(() => { setStep(5);
              t = setTimeout(() => { setStep(0); t = setTimeout(run, 1000); }, 2500);
            }, 500);
          }, 900);
        }, 1900);
      }, 700);
    };
    t = setTimeout(run, 900);
    return () => clearTimeout(t);
  }, []);

  return (
    <section style={{ padding: "96px 0", background: "var(--sys-mist)", overflow: "hidden" }}>
      <div style={{ maxWidth: 720, margin: "0 auto 80px", padding: "0 24px", textAlign: "center", display: "flex", flexDirection: "column", gap: 16 }}>
        <DemoEyebrow tone="info">Active Interception</DemoEyebrow>
        <h2 style={{ fontSize: "clamp(2rem,5vw,3rem)", fontWeight: 700, letterSpacing: "var(--tracking-tighter)", color: "var(--ink)", lineHeight: 1.05, margin: 0 }}>
          Every transaction. Every signal.<br />Verified before it moves.
        </h2>
        <p style={{ fontSize: "var(--text-lg)", fontWeight: 500, color: "#64748b", maxWidth: 540, margin: "0 auto" }}>
          Osprey sits between intent and execution. Fraud is stopped. Trust is earned. Users never wait.
        </p>
      </div>
      <div style={{ display: "flex", flexWrap: "wrap", alignItems: "center", justifyContent: "center", gap: 0, maxWidth: 1100, margin: "0 auto", padding: "0 24px" }}>
        <SenderDevice step={step} />
        <Intercept step={step} />
        <ReceiverDevice step={step} />
      </div>
      <style>{`@keyframes osf-spin{to{transform:rotate(360deg)}}`}</style>
    </section>
  );
}

Object.assign(window, { TransactionDemo });
