import React from "react";

export interface TextareaProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  invalid?: boolean;
  /** Use the mono stack — for pasting JSON / code. @default false */
  mono?: boolean;
  rows?: number;
}

/** Multi-line text input, with a mono variant for alert JSON. */
export function Textarea(props: TextareaProps): JSX.Element;
