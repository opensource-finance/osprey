import React from "react";

/**
 * Eyebrow — the UPPERCASE micro-label that sits above headings
 * ("ACTIVE INTERCEPTION", "ENGINEERED BY THE"). Wide tracking,
 * tiny, muted or accent-colored.
 */
export function Eyebrow({ tone = "muted", children, style, ...props }) {
  const tones = {
    muted: "var(--text-muted)",
    accent: "var(--primary)",
    info: "var(--info)",
  };
  return (
    <span
      style={{
        display: "inline-block",
        fontFamily: "var(--font-sans)",
        fontSize: "var(--text-2xs)",
        fontWeight: "var(--weight-bold)",
        letterSpacing: "var(--tracking-wider)",
        textTransform: "uppercase",
        color: tones[tone] || tones.muted,
        ...style,
      }}
      {...props}
    >
      {children}
    </span>
  );
}
