import React from "react";

/** Avatar — initials chip (or image). Used for founder / analyst identities. */
export function Avatar({ initials, src, alt = "", size = 48, style, ...props }) {
  return (
    <div
      style={{
        display: "inline-flex",
        alignItems: "center",
        justifyContent: "center",
        width: size,
        height: size,
        borderRadius: "var(--radius-pill)",
        background: "var(--surface-sunken)",
        color: "var(--text-muted)",
        fontFamily: "var(--font-sans)",
        fontWeight: "var(--weight-bold)",
        fontSize: size * 0.32,
        overflow: "hidden",
        flexShrink: 0,
        ...style,
      }}
      {...props}
    >
      {src ? (
        <img src={src} alt={alt} style={{ width: "100%", height: "100%", objectFit: "cover" }} />
      ) : (
        initials
      )}
    </div>
  );
}
