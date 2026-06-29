import React from "react";

/**
 * Osprey Button — the brand's primary action element.
 * Pills by default (the marketing/CTA shape); set shape="rounded"
 * for the tighter app/console feel. Swiss-red primary, subtle
 * neutrals, 300ms color transitions, soft focus ring.
 */
export function Button({
  variant = "primary",
  size = "md",
  shape = "pill",
  fullWidth = false,
  disabled = false,
  type = "button",
  children,
  style,
  ...props
}) {
  const sizes = {
    sm: { padding: "0 0.875rem", height: 36, fontSize: "var(--text-sm)", gap: 6 },
    md: { padding: "0 1.25rem", height: 44, fontSize: "var(--text-base)", gap: 8 },
    lg: { padding: "0 2rem", height: 56, fontSize: "var(--text-lg)", gap: 10 },
  };

  const variants = {
    primary: {
      background: "var(--primary)",
      color: "var(--primary-foreground)",
      border: "1px solid transparent",
    },
    secondary: {
      background: "var(--surface-sunken)",
      color: "var(--foreground)",
      border: "1px solid transparent",
    },
    outline: {
      background: "transparent",
      color: "var(--foreground)",
      border: "1px solid var(--border)",
    },
    ghost: {
      background: "transparent",
      color: "var(--foreground)",
      border: "1px solid transparent",
    },
    destructive: {
      background: "color-mix(in oklch, var(--destructive) 10%, transparent)",
      color: "var(--destructive)",
      border: "1px solid transparent",
    },
    link: {
      background: "transparent",
      color: "var(--primary)",
      border: "1px solid transparent",
      textDecoration: "underline",
      textUnderlineOffset: "4px",
    },
  };

  const s = sizes[size] || sizes.md;
  const v = variants[variant] || variants.primary;

  const hover = {
    primary: () => (e) => (e.currentTarget.style.background = "var(--primary-hover)"),
    secondary: () => (e) => (e.currentTarget.style.background = "var(--surface-hover)"),
    outline: () => (e) => (e.currentTarget.style.background = "var(--surface-sunken)"),
    ghost: () => (e) => (e.currentTarget.style.background = "var(--surface-sunken)"),
    destructive: () => (e) =>
      (e.currentTarget.style.background = "color-mix(in oklch, var(--destructive) 20%, transparent)"),
    link: () => () => {},
  };

  return (
    <button
      type={type}
      disabled={disabled}
      onMouseEnter={!disabled ? hover[variant]?.() : undefined}
      onMouseLeave={
        !disabled
          ? (e) => (e.currentTarget.style.background = v.background)
          : undefined
      }
      onFocus={(e) => (e.currentTarget.style.boxShadow = "var(--shadow-focus)")}
      onBlur={(e) => (e.currentTarget.style.boxShadow = "none")}
      style={{
        display: "inline-flex",
        alignItems: "center",
        justifyContent: "center",
        gap: s.gap,
        height: s.height,
        padding: s.padding,
        width: fullWidth ? "100%" : "auto",
        fontFamily: "var(--font-sans)",
        fontWeight: "var(--weight-medium)",
        fontSize: s.fontSize,
        letterSpacing: "var(--tracking-tight)",
        lineHeight: 1,
        whiteSpace: "nowrap",
        borderRadius: shape === "pill" ? "var(--radius-pill)" : "var(--radius-md)",
        cursor: disabled ? "not-allowed" : "pointer",
        opacity: disabled ? 0.5 : 1,
        transition: "background var(--dur-base), box-shadow var(--dur-fast), opacity var(--dur-base)",
        ...v,
        ...style,
      }}
      {...props}
    >
      {children}
    </button>
  );
}
