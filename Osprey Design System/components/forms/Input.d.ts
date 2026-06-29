import React from "react";

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  /** Render error border. @default false */
  invalid?: boolean;
}

/** Single-line text input — hairline border, 8px corners, red focus ring. */
export function Input(props: InputProps): JSX.Element;
