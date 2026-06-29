import React from "react";

export interface CardProps extends React.HTMLAttributes<HTMLDivElement> {
  /** Adds hover lift (border appears + soft raise) for clickable cards. @default false */
  interactive?: boolean;
  /** @default "lg" */
  padding?: "none" | "sm" | "md" | "lg";
  children?: React.ReactNode;
}

/** Base white surface with hairline ring, soft 24px corners and subtle shadow. */
export function Card(props: CardProps): JSX.Element;
