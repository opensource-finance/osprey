import React from "react";

export interface TerminalLog {
  text: string;
  status?: string;
  /** Status color. @default "ok" */
  tone?: "ok" | "warn" | "info" | "muted";
}

export interface TerminalDecision {
  label: string;
  value: string;
  tone?: "ok" | "warn" | "info" | "muted";
}

export interface TerminalProps extends React.HTMLAttributes<HTMLDivElement> {
  /** Chrome title. @default "OSPREY" */
  title?: string;
  logs?: TerminalLog[];
  /** Optional FINAL_DECISION footer row. */
  decision?: TerminalDecision | null;
}

/**
 * The Osprey "god-view" log panel — deep-slate glass chrome, mono log stream, decision row.
 * @startingPoint section="Feedback" subtitle="Slate terminal with FATF log stream" viewport="700x320"
 */
export function Terminal(props: TerminalProps): JSX.Element;
