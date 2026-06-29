import React from "react";

export interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  /**
   * `soft` is the default tinted-red pill; `solid` a filled red; `neutral` grey;
   * `outline` hairline; `success` green for verified/allowed states.
   * @default "soft"
   */
  variant?: "soft" | "solid" | "neutral" | "outline" | "success";
  /** Show the signature pulsing dot (used on "live"/eyebrow badges). @default false */
  live?: boolean;
  children?: React.ReactNode;
}

/** Small pill label for status & category markers, with optional pulsing "live" dot. */
export function Badge(props: BadgeProps): JSX.Element;
