import React from "react";

/**
 * Terminal — the Osprey "god-view" log panel. Deep-slate glass chrome
 * with traffic-light dots and the OSPREY monogram, a mono log stream
 * with semantic status colors, and an optional FINAL_DECISION row.
 *
 * logs: [{ text, status, tone }]  tone: "ok" | "warn" | "info" | "muted"
 * decision: { label, value, tone } | null
 */
export function Terminal({ title = "OSPREY", logs = [], decision = null, style, ...props }) {
  const toneColor = {
    ok: "var(--success)",
    warn: "var(--warning)",
    info: "var(--info)",
    muted: "#64748b",
  };
  return (
    <div
      style={{
        background: "color-mix(in srgb, var(--terminal-bg) 95%, transparent)",
        backdropFilter: "blur(var(--blur-glass))",
        border: "1px solid var(--terminal-border)",
        borderRadius: "var(--radius-lg)",
        boxShadow: "var(--shadow-2xl)",
        overflow: "hidden",
        fontFamily: "var(--font-mono)",
        ...style,
      }}
      {...props}
    >
      {/* Chrome */}
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          padding: "var(--space-2) var(--space-4)",
          borderBottom: "1px solid var(--terminal-border)",
          background: "rgba(15,23,42,0.5)",
        }}
      >
        <div style={{ display: "flex", gap: 6, opacity: 0.35 }}>
          {[0, 1, 2].map((i) => (
            <span key={i} style={{ width: 10, height: 10, borderRadius: "var(--radius-pill)", background: "#64748b" }} />
          ))}
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <span
            style={{
              fontSize: "var(--text-2xs)",
              fontWeight: "var(--weight-bold)",
              letterSpacing: "var(--tracking-wider)",
              color: "#94a3b8",
            }}
          >
            {title}
          </span>
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="var(--success)" strokeWidth="3">
            <circle cx="12" cy="12" r="6" />
            <path d="M12 2v4M12 18v4M2 12h4M18 12h4" />
          </svg>
        </div>
      </div>

      {/* Log stream */}
      <div style={{ padding: "var(--space-4)", display: "flex", flexDirection: "column", gap: "var(--space-2)" }}>
        {logs.map((log, i) => (
          <div
            key={i}
            style={{
              display: "flex",
              justifyContent: "space-between",
              gap: "var(--space-4)",
              fontSize: "var(--text-2xs)",
              lineHeight: "var(--leading-relaxed)",
              color: "var(--terminal-text)",
            }}
          >
            <span>{log.text}</span>
            {log.status && (
              <span style={{ color: toneColor[log.tone] || "var(--success)", fontWeight: "var(--weight-bold)" }}>
                {log.status}
              </span>
            )}
          </div>
        ))}

        {decision && (
          <div
            style={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
              marginTop: "var(--space-2)",
              paddingTop: "var(--space-2)",
              borderTop: "1px solid var(--terminal-border)",
              fontSize: "var(--text-2xs)",
            }}
          >
            <span style={{ color: "#94a3b8" }}>{decision.label}</span>
            <span
              style={{
                fontWeight: "var(--weight-bold)",
                color: toneColor[decision.tone] || "var(--success)",
                background: `color-mix(in srgb, ${toneColor[decision.tone] || "var(--success)"} 12%, transparent)`,
                padding: "2px 8px",
                borderRadius: "var(--radius-sm)",
              }}
            >
              {decision.value}
            </span>
          </div>
        )}
      </div>
    </div>
  );
}
