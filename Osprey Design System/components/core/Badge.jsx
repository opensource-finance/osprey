import React from "react";

/**
 * Badge / pill label. The brand's small status + category markers.
 * `live` adds the signature pulsing dot used on "Introducing Osprey",
 * "Origin Story" eyebrows, etc.
 */
export function Badge({ variant = "soft", live = false, children, style, ...props }) {
  const variants = {
    soft: {
      background: "var(--primary-soft)",
      color: "var(--primary)",
      border: "1px solid color-mix(in oklch, var(--primary) 20%, transparent)",
    },
    solid: {
      background: "var(--primary)",
      color: "var(--primary-foreground)",
      border: "1px solid transparent",
    },
    neutral: {
      background: "var(--surface-sunken)",
      color: "var(--text-muted)",
      border: "1px solid transparent",
    },
    outline: {
      background: "transparent",
      color: "var(--foreground)",
      border: "1px solid var(--border)",
    },
    success: {
      background: "color-mix(in oklch, var(--success) 12%, transparent)",
      color: "color-mix(in oklch, var(--success) 70%, black)",
      border: "1px solid color-mix(in oklch, var(--success) 30%, transparent)",
    },
  };
  const v = variants[variant] || variants.soft;
  const dotColor = variant === "success" ? "var(--success)" : "currentColor";

  return (
    <span
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: 6,
        height: 24,
        padding: "0 0.625rem",
        fontFamily: "var(--font-sans)",
        fontSize: "var(--text-xs)",
        fontWeight: "var(--weight-medium)",
        letterSpacing: "var(--tracking-tight)",
        lineHeight: 1,
        borderRadius: "var(--radius-pill)",
        whiteSpace: "nowrap",
        ...v,
        ...style,
      }}
      {...props}
    >
      {live && (
        <span style={{ position: "relative", display: "inline-flex", width: 8, height: 8 }}>
          <span
            style={{
              position: "absolute",
              inset: 0,
              borderRadius: "var(--radius-pill)",
              background: dotColor,
              opacity: 0.75,
              animation: "osprey-ping 1.6s var(--ease-out) infinite",
            }}
          />
          <span
            style={{
              position: "relative",
              width: 8,
              height: 8,
              borderRadius: "var(--radius-pill)",
              background: dotColor,
            }}
          />
        </span>
      )}
      {children}
      <style>{`@keyframes osprey-ping{75%,100%{transform:scale(2);opacity:0}}`}</style>
    </span>
  );
}
