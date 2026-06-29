import React from "react";

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  /**
   * Visual style. `primary` is Swiss-red (default CTA); `outline`/`ghost`/`secondary`
   * for lower emphasis; `link` for inline text actions; `destructive` for dangerous ops.
   * @default "primary"
   */
  variant?: "primary" | "secondary" | "outline" | "ghost" | "destructive" | "link";
  /** @default "md" */
  size?: "sm" | "md" | "lg";
  /** Pill is the brand/marketing shape; rounded for tighter app chrome. @default "pill" */
  shape?: "pill" | "rounded";
  /** Stretch to container width. @default false */
  fullWidth?: boolean;
  disabled?: boolean;
  children?: React.ReactNode;
}

/**
 * The brand's primary action element. Swiss-red primary, soft focus ring, calm transitions.
 * @startingPoint section="Core" subtitle="Pill & rounded buttons in every variant" viewport="700x180"
 */
export function Button(props: ButtonProps): JSX.Element;
