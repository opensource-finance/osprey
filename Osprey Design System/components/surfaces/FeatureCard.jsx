import React from "react";

/**
 * FeatureCard — the marketing feature tile. Large 32px radius,
 * sunken grey fill that reveals a border on hover, and a circular
 * white icon chip that scales up on hover. Pass any line icon
 * (e.g. a Lucide <i data-lucide> or an <svg>) as `icon`.
 */
export function FeatureCard({ icon, title, children, style, ...props }) {
  return (
    <div
      onMouseEnter={(e) => {
        e.currentTarget.style.borderColor = "var(--border)";
        e.currentTarget.style.background = "var(--surface-hover)";
        const chip = e.currentTarget.querySelector("[data-chip]");
        if (chip) chip.style.transform = "scale(1.1)";
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.borderColor = "transparent";
        e.currentTarget.style.background = "var(--surface-sunken)";
        const chip = e.currentTarget.querySelector("[data-chip]");
        if (chip) chip.style.transform = "scale(1)";
      }}
      style={{
        background: "var(--surface-sunken)",
        border: "1px solid transparent",
        borderRadius: "var(--radius-2xl)",
        padding: "var(--space-8)",
        transition: "background var(--dur-base), border-color var(--dur-base)",
        ...style,
      }}
      {...props}
    >
      {icon && (
        <div
          data-chip
          style={{
            display: "inline-flex",
            alignItems: "center",
            justifyContent: "center",
            width: 48,
            height: 48,
            marginBottom: "var(--space-6)",
            borderRadius: "var(--radius-pill)",
            background: "var(--white)",
            color: "var(--primary)",
            border: "1px solid var(--border)",
            boxShadow: "var(--shadow-sm)",
            transition: "transform var(--dur-base)",
          }}
        >
          {icon}
        </div>
      )}
      <h3
        style={{
          fontSize: "var(--text-2xl)",
          fontWeight: "var(--weight-medium)",
          letterSpacing: "var(--tracking-tight)",
          margin: "0 0 var(--space-3)",
        }}
      >
        {title}
      </h3>
      <p
        style={{
          fontSize: "var(--text-lg)",
          color: "var(--text-muted)",
          lineHeight: "var(--leading-relaxed)",
          margin: 0,
        }}
      >
        {children}
      </p>
    </div>
  );
}
