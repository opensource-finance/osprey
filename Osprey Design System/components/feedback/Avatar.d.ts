import React from "react";

export interface AvatarProps extends React.HTMLAttributes<HTMLDivElement> {
  /** Fallback initials shown when no image. */
  initials?: string;
  src?: string;
  alt?: string;
  /** Pixel diameter. @default 48 */
  size?: number;
}

/** Circular initials/image chip for person identities. */
export function Avatar(props: AvatarProps): JSX.Element;
