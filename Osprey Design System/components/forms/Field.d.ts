import React from "react";

export interface FieldProps extends React.HTMLAttributes<HTMLDivElement> {
  label?: React.ReactNode;
  /** Helper text shown below the control. */
  hint?: React.ReactNode;
  /** Error text — overrides hint and turns it red. */
  error?: React.ReactNode;
  htmlFor?: string;
  children?: React.ReactNode;
}

/** Label + control + hint/error layout wrapper. */
export function Field(props: FieldProps): JSX.Element;
