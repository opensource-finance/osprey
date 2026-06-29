import React from "react";

export interface EyebrowProps extends React.HTMLAttributes<HTMLSpanElement> {
  /** @default "muted" */
  tone?: "muted" | "accent" | "info";
  children?: React.ReactNode;
}

/** UPPERCASE wide-tracked micro-label that sits above section headings. */
export function Eyebrow(props: EyebrowProps): JSX.Element;
