import React from "react";

/** Multi-line input. Mono variant for pasting alert JSON into the Narrator. */
export function Textarea({ invalid = false, mono = false, rows = 5, style, ...props }) {
  return (
    <textarea
      rows={rows}
      onFocus={(e) => {
        e.currentTarget.style.borderColor = "var(--ring)";
        e.currentTarget.style.boxShadow = "var(--shadow-focus)";
      }}
      onBlur={(e) => {
        e.currentTarget.style.borderColor = invalid ? "var(--destructive)" : "var(--input)";
        e.currentTarget.style.boxShadow = "none";
      }}
      style={{
        width: "100%",
        padding: "var(--space-3) var(--space-4)",
        fontFamily: mono ? "var(--font-mono)" : "var(--font-sans)",
        fontSize: mono ? "var(--text-sm)" : "var(--text-base)",
        lineHeight: "var(--leading-relaxed)",
        color: "var(--foreground)",
        background: "var(--surface-card)",
        border: `1px solid ${invalid ? "var(--destructive)" : "var(--input)"}`,
        borderRadius: "var(--radius-sm)",
        outline: "none",
        resize: "vertical",
        transition: "border-color var(--dur-fast), box-shadow var(--dur-fast)",
        ...style,
      }}
      {...props}
    />
  );
}
