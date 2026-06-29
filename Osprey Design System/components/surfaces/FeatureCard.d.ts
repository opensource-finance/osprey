import React from "react";

export interface FeatureCardProps extends React.HTMLAttributes<HTMLDivElement> {
  /** A line icon (Lucide element or <svg>) shown in the circular chip. */
  icon?: React.ReactNode;
  /** Feature title. */
  title?: React.ReactNode;
  /** Description body. */
  children?: React.ReactNode;
}

/**
 * Marketing feature tile — 32px radius, sunken fill, white icon chip that scales on hover.
 * @startingPoint section="Surfaces" subtitle="Feature tile with icon chip & hover reveal" viewport="700x260"
 */
export function FeatureCard(props: FeatureCardProps): JSX.Element;
