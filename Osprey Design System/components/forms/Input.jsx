import React from "react";

/** Text input — hairline border, soft 8px corners, red focus ring. */
export function Input({ invalid = false, style, ...props }) {
  return (
    <input
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
        height: 44,
        padding: "0 var(--space-4)",
        fontFamily: "var(--font-sans)",
        fontSize: "var(--text-base)",
        color: "var(--foreground)",
        background: "var(--surface-card)",
        border: `1px solid ${invalid ? "var(--destructive)" : "var(--input)"}`,
        borderRadius: "var(--radius-sm)",
        outline: "none",
        transition: "border-color var(--dur-fast), box-shadow var(--dur-fast)",
        ...style,
      }}
      {...props}
    />
  );
}
