import React from "react";

export interface WordmarkProps extends React.HTMLAttributes<HTMLSpanElement> {
  /** `osf` = red square + opensource.finance; `osprey` = glyph + OSPREY. @default "osf" */
  brand?: "osf" | "osprey";
  /** Dot motif shape for the osf lockup. @default "square" */
  dot?: "square" | "circle";
  /** Wordmark text color (defaults to --foreground). */
  color?: string;
  /** Text size in px (the dot scales with it). @default 18 */
  size?: number;
}

/**
 * Brand wordmark lockup — the red-square "opensource.finance" or the Osprey glyph wordmark.
 * @startingPoint section="Brand" subtitle="opensource.finance & Osprey lockups" viewport="700x120"
 */
export function Wordmark(props: WordmarkProps): JSX.Element;
