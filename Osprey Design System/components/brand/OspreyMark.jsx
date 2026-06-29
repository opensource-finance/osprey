import React from "react";

/**
 * OspreyMark — the logo glyph: a rounded shield housing a stylized
 * eye/lens of interlocking shutters with a center pupil. Default
 * Swiss-system blue (#007AFF); pass color="currentColor" to tint.
 */
export function OspreyMark({ size = 24, color = "#007AFF", style, ...props }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      role="img"
      aria-label="Osprey"
      style={style}
      {...props}
    >
      <path
        d="M12 2L3 7V12C3 17.5228 7.47715 22 12 22C16.5228 22 21 17.5228 21 12V7L12 2Z"
        stroke={color}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        opacity="0.2"
      />
      <path d="M12 8V16" stroke={color} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M8.5359 10L15.4641 14" stroke={color} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M8.5359 14L15.4641 10" stroke={color} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
      <circle cx="12" cy="12" r="1.5" fill={color} />
    </svg>
  );
}
