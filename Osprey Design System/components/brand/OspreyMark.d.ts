import React from "react";

export interface OspreyMarkProps extends React.SVGAttributes<SVGElement> {
  /** Pixel size. @default 24 */
  size?: number;
  /** Stroke/fill color. @default "#007AFF" */
  color?: string;
}

/** The Osprey logo glyph — shield + monitoring "eye". */
export function OspreyMark(props: OspreyMarkProps): JSX.Element;
