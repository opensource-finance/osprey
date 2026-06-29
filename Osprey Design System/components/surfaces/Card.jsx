import React from "react";

/**
 * Card — the base surface. White, hairline ring, soft corners,
 * subtle shadow. `interactive` adds the brand's hover lift
 * (border appears + faint raise).
 */
export function Card({ interactive = false, padding = "lg", children, style, ...props }) {
  const pads = { none: 0, sm: "var(--space-4)", md: "var(--space-6)", lg: "var(--space-8)" };
  return (
    <div
      onMouseEnter={
        interactive
          ? (e) => {
              e.currentTarget.style.borderColor = "var(--border)";
              e.currentTarget.style.boxShadow = "var(--shadow-md)";
            }
          : undefined
      }
      onMouseLeave={
        interactive
          ? (e) => {
              e.currentTarget.style.borderColor = "transparent";
              e.currentTarget.style.boxShadow = "var(--shadow-sm)";
            }
          : undefined
      }
      style={{
        background: interactive ? "var(--surface-sunken)" : "var(--surface-card)",
        color: "var(--foreground)",
        borderRadius: "var(--radius-xl)",
        border: interactive ? "1px solid transparent" : "1px solid var(--border-soft)",
        boxShadow: "var(--shadow-sm)",
        padding: pads[padding],
        transition: "border-color var(--dur-base), box-shadow var(--dur-base)",
        ...style,
      }}
      {...props}
    >
      {children}
    </div>
  );
}
