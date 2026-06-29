import React from "react";

/**
 * Wordmark — the brand lockup. `osf` renders the Swiss-red square
 * + "opensource.finance" (the OG lockup). `osprey` renders the
 * Osprey glyph + "OSPREY" wordmark. Square is the canonical dot
 * motif; pass dot="circle" for the header variant.
 */
export function Wordmark({ brand = "osf", dot = "square", color, size = 18, style, ...props }) {
  const text = color || "var(--foreground)";
  if (brand === "osprey") {
    return (
      <span style={{ display: "inline-flex", alignItems: "center", gap: 8, ...style }} {...props}>
        <svg width={size} height={size} viewBox="0 0 24 24" fill="none" role="img" aria-label="Osprey">
          <path d="M12 2L3 7V12C3 17.5228 7.47715 22 12 22C16.5228 22 21 17.5228 21 12V7L12 2Z" stroke="#007AFF" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" opacity="0.2" />
          <path d="M12 8V16" stroke="#007AFF" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
          <path d="M8.5359 10L15.4641 14" stroke="#007AFF" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
          <path d="M8.5359 14L15.4641 10" stroke="#007AFF" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
          <circle cx="12" cy="12" r="1.5" fill="#007AFF" />
        </svg>
        <span
          style={{
            fontFamily: "var(--font-sans)",
            fontWeight: "var(--weight-bold)",
            letterSpacing: "var(--tracking-tight)",
            fontSize: size,
            color: text,
          }}
        >
          OSPREY
        </span>
      </span>
    );
  }
  return (
    <span style={{ display: "inline-flex", alignItems: "center", gap: 10, ...style }} {...props}>
      <span
        style={{
          width: size * 0.55,
          height: size * 0.55,
          background: "var(--primary)",
          borderRadius: dot === "circle" ? "var(--radius-pill)" : "2px",
          flexShrink: 0,
        }}
      />
      <span
        style={{
          fontFamily: "var(--font-sans)",
          fontWeight: "var(--weight-bold)",
          letterSpacing: "var(--tracking-tight)",
          fontSize: size,
          color: text,
        }}
      >
        opensource.finance
      </span>
    </span>
  );
}
