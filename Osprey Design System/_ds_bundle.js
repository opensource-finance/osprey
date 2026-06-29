/* @ds-bundle: {"format":3,"namespace":"OpensourceFinanceOspreyDesignSystem_08d2ca","components":[{"name":"OspreyMark","sourcePath":"components/brand/OspreyMark.jsx"},{"name":"Wordmark","sourcePath":"components/brand/Wordmark.jsx"},{"name":"Badge","sourcePath":"components/core/Badge.jsx"},{"name":"Button","sourcePath":"components/core/Button.jsx"},{"name":"Eyebrow","sourcePath":"components/core/Eyebrow.jsx"},{"name":"Avatar","sourcePath":"components/feedback/Avatar.jsx"},{"name":"Terminal","sourcePath":"components/feedback/Terminal.jsx"},{"name":"Field","sourcePath":"components/forms/Field.jsx"},{"name":"Input","sourcePath":"components/forms/Input.jsx"},{"name":"Textarea","sourcePath":"components/forms/Textarea.jsx"},{"name":"Card","sourcePath":"components/surfaces/Card.jsx"},{"name":"FeatureCard","sourcePath":"components/surfaces/FeatureCard.jsx"}],"sourceHashes":{"components/brand/OspreyMark.jsx":"376fd1a42a9e","components/brand/Wordmark.jsx":"f58aadeabf1b","components/core/Badge.jsx":"bfd107f1a472","components/core/Button.jsx":"8f170fae1c51","components/core/Eyebrow.jsx":"bc9073ca68b9","components/feedback/Avatar.jsx":"aa520e1bc947","components/feedback/Terminal.jsx":"b5e80a440804","components/forms/Field.jsx":"71cdd7948c5c","components/forms/Input.jsx":"b21606988f63","components/forms/Textarea.jsx":"ad6ac9e761ae","components/surfaces/Card.jsx":"8934a431e3e9","components/surfaces/FeatureCard.jsx":"6d45d1463e9e","ui_kits/narrator/app.jsx":"891946027d6d","ui_kits/narrator/narrator.jsx":"053630b8f7f0","ui_kits/website/app.jsx":"79e09041432b","ui_kits/website/demo.jsx":"a782ec4c859b","ui_kits/website/platform.jsx":"560e3065ff70","ui_kits/website/sections.jsx":"082587047859"},"inlinedExternals":[],"unexposedExports":[]} */

(() => {

const __ds_ns = (window.OpensourceFinanceOspreyDesignSystem_08d2ca = window.OpensourceFinanceOspreyDesignSystem_08d2ca || {});

const __ds_scope = {};

(__ds_ns.__errors = __ds_ns.__errors || []);

// components/brand/OspreyMark.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
/**
 * OspreyMark — the logo glyph: a rounded shield housing a stylized
 * eye/lens of interlocking shutters with a center pupil. Default
 * Swiss-system blue (#007AFF); pass color="currentColor" to tint.
 */
function OspreyMark({
  size = 24,
  color = "#007AFF",
  style,
  ...props
}) {
  return /*#__PURE__*/React.createElement("svg", _extends({
    width: size,
    height: size,
    viewBox: "0 0 24 24",
    fill: "none",
    xmlns: "http://www.w3.org/2000/svg",
    role: "img",
    "aria-label": "Osprey",
    style: style
  }, props), /*#__PURE__*/React.createElement("path", {
    d: "M12 2L3 7V12C3 17.5228 7.47715 22 12 22C16.5228 22 21 17.5228 21 12V7L12 2Z",
    stroke: color,
    strokeWidth: "2",
    strokeLinecap: "round",
    strokeLinejoin: "round",
    opacity: "0.2"
  }), /*#__PURE__*/React.createElement("path", {
    d: "M12 8V16",
    stroke: color,
    strokeWidth: "2",
    strokeLinecap: "round",
    strokeLinejoin: "round"
  }), /*#__PURE__*/React.createElement("path", {
    d: "M8.5359 10L15.4641 14",
    stroke: color,
    strokeWidth: "2",
    strokeLinecap: "round",
    strokeLinejoin: "round"
  }), /*#__PURE__*/React.createElement("path", {
    d: "M8.5359 14L15.4641 10",
    stroke: color,
    strokeWidth: "2",
    strokeLinecap: "round",
    strokeLinejoin: "round"
  }), /*#__PURE__*/React.createElement("circle", {
    cx: "12",
    cy: "12",
    r: "1.5",
    fill: color
  }));
}
Object.assign(__ds_scope, { OspreyMark });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/brand/OspreyMark.jsx", error: String((e && e.message) || e) }); }

// components/brand/Wordmark.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
/**
 * Wordmark — the brand lockup. `osf` renders the Swiss-red square
 * + "opensource.finance" (the OG lockup). `osprey` renders the
 * Osprey glyph + "OSPREY" wordmark. Square is the canonical dot
 * motif; pass dot="circle" for the header variant.
 */
function Wordmark({
  brand = "osf",
  dot = "square",
  color,
  size = 18,
  style,
  ...props
}) {
  const text = color || "var(--foreground)";
  if (brand === "osprey") {
    return /*#__PURE__*/React.createElement("span", _extends({
      style: {
        display: "inline-flex",
        alignItems: "center",
        gap: 8,
        ...style
      }
    }, props), /*#__PURE__*/React.createElement("svg", {
      width: size,
      height: size,
      viewBox: "0 0 24 24",
      fill: "none",
      role: "img",
      "aria-label": "Osprey"
    }, /*#__PURE__*/React.createElement("path", {
      d: "M12 2L3 7V12C3 17.5228 7.47715 22 12 22C16.5228 22 21 17.5228 21 12V7L12 2Z",
      stroke: "#007AFF",
      strokeWidth: "2",
      strokeLinecap: "round",
      strokeLinejoin: "round",
      opacity: "0.2"
    }), /*#__PURE__*/React.createElement("path", {
      d: "M12 8V16",
      stroke: "#007AFF",
      strokeWidth: "2",
      strokeLinecap: "round",
      strokeLinejoin: "round"
    }), /*#__PURE__*/React.createElement("path", {
      d: "M8.5359 10L15.4641 14",
      stroke: "#007AFF",
      strokeWidth: "2",
      strokeLinecap: "round",
      strokeLinejoin: "round"
    }), /*#__PURE__*/React.createElement("path", {
      d: "M8.5359 14L15.4641 10",
      stroke: "#007AFF",
      strokeWidth: "2",
      strokeLinecap: "round",
      strokeLinejoin: "round"
    }), /*#__PURE__*/React.createElement("circle", {
      cx: "12",
      cy: "12",
      r: "1.5",
      fill: "#007AFF"
    })), /*#__PURE__*/React.createElement("span", {
      style: {
        fontFamily: "var(--font-sans)",
        fontWeight: "var(--weight-bold)",
        letterSpacing: "var(--tracking-tight)",
        fontSize: size,
        color: text
      }
    }, "OSPREY"));
  }
  return /*#__PURE__*/React.createElement("span", _extends({
    style: {
      display: "inline-flex",
      alignItems: "center",
      gap: 10,
      ...style
    }
  }, props), /*#__PURE__*/React.createElement("span", {
    style: {
      width: size * 0.55,
      height: size * 0.55,
      background: "var(--primary)",
      borderRadius: dot === "circle" ? "var(--radius-pill)" : "2px",
      flexShrink: 0
    }
  }), /*#__PURE__*/React.createElement("span", {
    style: {
      fontFamily: "var(--font-sans)",
      fontWeight: "var(--weight-bold)",
      letterSpacing: "var(--tracking-tight)",
      fontSize: size,
      color: text
    }
  }, "opensource.finance"));
}
Object.assign(__ds_scope, { Wordmark });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/brand/Wordmark.jsx", error: String((e && e.message) || e) }); }

// components/core/Badge.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
/**
 * Badge / pill label. The brand's small status + category markers.
 * `live` adds the signature pulsing dot used on "Introducing Osprey",
 * "Origin Story" eyebrows, etc.
 */
function Badge({
  variant = "soft",
  live = false,
  children,
  style,
  ...props
}) {
  const variants = {
    soft: {
      background: "var(--primary-soft)",
      color: "var(--primary)",
      border: "1px solid color-mix(in oklch, var(--primary) 20%, transparent)"
    },
    solid: {
      background: "var(--primary)",
      color: "var(--primary-foreground)",
      border: "1px solid transparent"
    },
    neutral: {
      background: "var(--surface-sunken)",
      color: "var(--text-muted)",
      border: "1px solid transparent"
    },
    outline: {
      background: "transparent",
      color: "var(--foreground)",
      border: "1px solid var(--border)"
    },
    success: {
      background: "color-mix(in oklch, var(--success) 12%, transparent)",
      color: "color-mix(in oklch, var(--success) 70%, black)",
      border: "1px solid color-mix(in oklch, var(--success) 30%, transparent)"
    }
  };
  const v = variants[variant] || variants.soft;
  const dotColor = variant === "success" ? "var(--success)" : "currentColor";
  return /*#__PURE__*/React.createElement("span", _extends({
    style: {
      display: "inline-flex",
      alignItems: "center",
      gap: 6,
      height: 24,
      padding: "0 0.625rem",
      fontFamily: "var(--font-sans)",
      fontSize: "var(--text-xs)",
      fontWeight: "var(--weight-medium)",
      letterSpacing: "var(--tracking-tight)",
      lineHeight: 1,
      borderRadius: "var(--radius-pill)",
      whiteSpace: "nowrap",
      ...v,
      ...style
    }
  }, props), live && /*#__PURE__*/React.createElement("span", {
    style: {
      position: "relative",
      display: "inline-flex",
      width: 8,
      height: 8
    }
  }, /*#__PURE__*/React.createElement("span", {
    style: {
      position: "absolute",
      inset: 0,
      borderRadius: "var(--radius-pill)",
      background: dotColor,
      opacity: 0.75,
      animation: "osprey-ping 1.6s var(--ease-out) infinite"
    }
  }), /*#__PURE__*/React.createElement("span", {
    style: {
      position: "relative",
      width: 8,
      height: 8,
      borderRadius: "var(--radius-pill)",
      background: dotColor
    }
  })), children, /*#__PURE__*/React.createElement("style", null, `@keyframes osprey-ping{75%,100%{transform:scale(2);opacity:0}}`));
}
Object.assign(__ds_scope, { Badge });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/core/Badge.jsx", error: String((e && e.message) || e) }); }

// components/core/Button.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
/**
 * Osprey Button — the brand's primary action element.
 * Pills by default (the marketing/CTA shape); set shape="rounded"
 * for the tighter app/console feel. Swiss-red primary, subtle
 * neutrals, 300ms color transitions, soft focus ring.
 */
function Button({
  variant = "primary",
  size = "md",
  shape = "pill",
  fullWidth = false,
  disabled = false,
  type = "button",
  children,
  style,
  ...props
}) {
  const sizes = {
    sm: {
      padding: "0 0.875rem",
      height: 36,
      fontSize: "var(--text-sm)",
      gap: 6
    },
    md: {
      padding: "0 1.25rem",
      height: 44,
      fontSize: "var(--text-base)",
      gap: 8
    },
    lg: {
      padding: "0 2rem",
      height: 56,
      fontSize: "var(--text-lg)",
      gap: 10
    }
  };
  const variants = {
    primary: {
      background: "var(--primary)",
      color: "var(--primary-foreground)",
      border: "1px solid transparent"
    },
    secondary: {
      background: "var(--surface-sunken)",
      color: "var(--foreground)",
      border: "1px solid transparent"
    },
    outline: {
      background: "transparent",
      color: "var(--foreground)",
      border: "1px solid var(--border)"
    },
    ghost: {
      background: "transparent",
      color: "var(--foreground)",
      border: "1px solid transparent"
    },
    destructive: {
      background: "color-mix(in oklch, var(--destructive) 10%, transparent)",
      color: "var(--destructive)",
      border: "1px solid transparent"
    },
    link: {
      background: "transparent",
      color: "var(--primary)",
      border: "1px solid transparent",
      textDecoration: "underline",
      textUnderlineOffset: "4px"
    }
  };
  const s = sizes[size] || sizes.md;
  const v = variants[variant] || variants.primary;
  const hover = {
    primary: () => e => e.currentTarget.style.background = "var(--primary-hover)",
    secondary: () => e => e.currentTarget.style.background = "var(--surface-hover)",
    outline: () => e => e.currentTarget.style.background = "var(--surface-sunken)",
    ghost: () => e => e.currentTarget.style.background = "var(--surface-sunken)",
    destructive: () => e => e.currentTarget.style.background = "color-mix(in oklch, var(--destructive) 20%, transparent)",
    link: () => () => {}
  };
  return /*#__PURE__*/React.createElement("button", _extends({
    type: type,
    disabled: disabled,
    onMouseEnter: !disabled ? hover[variant]?.() : undefined,
    onMouseLeave: !disabled ? e => e.currentTarget.style.background = v.background : undefined,
    onFocus: e => e.currentTarget.style.boxShadow = "var(--shadow-focus)",
    onBlur: e => e.currentTarget.style.boxShadow = "none",
    style: {
      display: "inline-flex",
      alignItems: "center",
      justifyContent: "center",
      gap: s.gap,
      height: s.height,
      padding: s.padding,
      width: fullWidth ? "100%" : "auto",
      fontFamily: "var(--font-sans)",
      fontWeight: "var(--weight-medium)",
      fontSize: s.fontSize,
      letterSpacing: "var(--tracking-tight)",
      lineHeight: 1,
      whiteSpace: "nowrap",
      borderRadius: shape === "pill" ? "var(--radius-pill)" : "var(--radius-md)",
      cursor: disabled ? "not-allowed" : "pointer",
      opacity: disabled ? 0.5 : 1,
      transition: "background var(--dur-base), box-shadow var(--dur-fast), opacity var(--dur-base)",
      ...v,
      ...style
    }
  }, props), children);
}
Object.assign(__ds_scope, { Button });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/core/Button.jsx", error: String((e && e.message) || e) }); }

// components/core/Eyebrow.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
/**
 * Eyebrow — the UPPERCASE micro-label that sits above headings
 * ("ACTIVE INTERCEPTION", "ENGINEERED BY THE"). Wide tracking,
 * tiny, muted or accent-colored.
 */
function Eyebrow({
  tone = "muted",
  children,
  style,
  ...props
}) {
  const tones = {
    muted: "var(--text-muted)",
    accent: "var(--primary)",
    info: "var(--info)"
  };
  return /*#__PURE__*/React.createElement("span", _extends({
    style: {
      display: "inline-block",
      fontFamily: "var(--font-sans)",
      fontSize: "var(--text-2xs)",
      fontWeight: "var(--weight-bold)",
      letterSpacing: "var(--tracking-wider)",
      textTransform: "uppercase",
      color: tones[tone] || tones.muted,
      ...style
    }
  }, props), children);
}
Object.assign(__ds_scope, { Eyebrow });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/core/Eyebrow.jsx", error: String((e && e.message) || e) }); }

// components/feedback/Avatar.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
/** Avatar — initials chip (or image). Used for founder / analyst identities. */
function Avatar({
  initials,
  src,
  alt = "",
  size = 48,
  style,
  ...props
}) {
  return /*#__PURE__*/React.createElement("div", _extends({
    style: {
      display: "inline-flex",
      alignItems: "center",
      justifyContent: "center",
      width: size,
      height: size,
      borderRadius: "var(--radius-pill)",
      background: "var(--surface-sunken)",
      color: "var(--text-muted)",
      fontFamily: "var(--font-sans)",
      fontWeight: "var(--weight-bold)",
      fontSize: size * 0.32,
      overflow: "hidden",
      flexShrink: 0,
      ...style
    }
  }, props), src ? /*#__PURE__*/React.createElement("img", {
    src: src,
    alt: alt,
    style: {
      width: "100%",
      height: "100%",
      objectFit: "cover"
    }
  }) : initials);
}
Object.assign(__ds_scope, { Avatar });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/feedback/Avatar.jsx", error: String((e && e.message) || e) }); }

// components/feedback/Terminal.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
/**
 * Terminal — the Osprey "god-view" log panel. Deep-slate glass chrome
 * with traffic-light dots and the OSPREY monogram, a mono log stream
 * with semantic status colors, and an optional FINAL_DECISION row.
 *
 * logs: [{ text, status, tone }]  tone: "ok" | "warn" | "info" | "muted"
 * decision: { label, value, tone } | null
 */
function Terminal({
  title = "OSPREY",
  logs = [],
  decision = null,
  style,
  ...props
}) {
  const toneColor = {
    ok: "var(--success)",
    warn: "var(--warning)",
    info: "var(--info)",
    muted: "#64748b"
  };
  return /*#__PURE__*/React.createElement("div", _extends({
    style: {
      background: "color-mix(in srgb, var(--terminal-bg) 95%, transparent)",
      backdropFilter: "blur(var(--blur-glass))",
      border: "1px solid var(--terminal-border)",
      borderRadius: "var(--radius-lg)",
      boxShadow: "var(--shadow-2xl)",
      overflow: "hidden",
      fontFamily: "var(--font-mono)",
      ...style
    }
  }, props), /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      justifyContent: "space-between",
      alignItems: "center",
      padding: "var(--space-2) var(--space-4)",
      borderBottom: "1px solid var(--terminal-border)",
      background: "rgba(15,23,42,0.5)"
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      gap: 6,
      opacity: 0.35
    }
  }, [0, 1, 2].map(i => /*#__PURE__*/React.createElement("span", {
    key: i,
    style: {
      width: 10,
      height: 10,
      borderRadius: "var(--radius-pill)",
      background: "#64748b"
    }
  }))), /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      alignItems: "center",
      gap: 8
    }
  }, /*#__PURE__*/React.createElement("span", {
    style: {
      fontSize: "var(--text-2xs)",
      fontWeight: "var(--weight-bold)",
      letterSpacing: "var(--tracking-wider)",
      color: "#94a3b8"
    }
  }, title), /*#__PURE__*/React.createElement("svg", {
    width: "13",
    height: "13",
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "var(--success)",
    strokeWidth: "3"
  }, /*#__PURE__*/React.createElement("circle", {
    cx: "12",
    cy: "12",
    r: "6"
  }), /*#__PURE__*/React.createElement("path", {
    d: "M12 2v4M12 18v4M2 12h4M18 12h4"
  })))), /*#__PURE__*/React.createElement("div", {
    style: {
      padding: "var(--space-4)",
      display: "flex",
      flexDirection: "column",
      gap: "var(--space-2)"
    }
  }, logs.map((log, i) => /*#__PURE__*/React.createElement("div", {
    key: i,
    style: {
      display: "flex",
      justifyContent: "space-between",
      gap: "var(--space-4)",
      fontSize: "var(--text-2xs)",
      lineHeight: "var(--leading-relaxed)",
      color: "var(--terminal-text)"
    }
  }, /*#__PURE__*/React.createElement("span", null, log.text), log.status && /*#__PURE__*/React.createElement("span", {
    style: {
      color: toneColor[log.tone] || "var(--success)",
      fontWeight: "var(--weight-bold)"
    }
  }, log.status))), decision && /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      justifyContent: "space-between",
      alignItems: "center",
      marginTop: "var(--space-2)",
      paddingTop: "var(--space-2)",
      borderTop: "1px solid var(--terminal-border)",
      fontSize: "var(--text-2xs)"
    }
  }, /*#__PURE__*/React.createElement("span", {
    style: {
      color: "#94a3b8"
    }
  }, decision.label), /*#__PURE__*/React.createElement("span", {
    style: {
      fontWeight: "var(--weight-bold)",
      color: toneColor[decision.tone] || "var(--success)",
      background: `color-mix(in srgb, ${toneColor[decision.tone] || "var(--success)"} 12%, transparent)`,
      padding: "2px 8px",
      borderRadius: "var(--radius-sm)"
    }
  }, decision.value))));
}
Object.assign(__ds_scope, { Terminal });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/feedback/Terminal.jsx", error: String((e && e.message) || e) }); }

// components/forms/Field.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
/** Field — label + control + optional hint/error wrapper. */
function Field({
  label,
  hint,
  error,
  htmlFor,
  children,
  style,
  ...props
}) {
  return /*#__PURE__*/React.createElement("div", _extends({
    style: {
      display: "flex",
      flexDirection: "column",
      gap: "var(--space-2)",
      ...style
    }
  }, props), label && /*#__PURE__*/React.createElement("label", {
    htmlFor: htmlFor,
    style: {
      fontFamily: "var(--font-sans)",
      fontSize: "var(--text-sm)",
      fontWeight: "var(--weight-medium)",
      color: "var(--foreground)"
    }
  }, label), children, (hint || error) && /*#__PURE__*/React.createElement("span", {
    style: {
      fontSize: "var(--text-xs)",
      color: error ? "var(--destructive)" : "var(--text-muted)"
    }
  }, error || hint));
}
Object.assign(__ds_scope, { Field });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/forms/Field.jsx", error: String((e && e.message) || e) }); }

// components/forms/Input.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
/** Text input — hairline border, soft 8px corners, red focus ring. */
function Input({
  invalid = false,
  style,
  ...props
}) {
  return /*#__PURE__*/React.createElement("input", _extends({
    onFocus: e => {
      e.currentTarget.style.borderColor = "var(--ring)";
      e.currentTarget.style.boxShadow = "var(--shadow-focus)";
    },
    onBlur: e => {
      e.currentTarget.style.borderColor = invalid ? "var(--destructive)" : "var(--input)";
      e.currentTarget.style.boxShadow = "none";
    },
    style: {
      width: "100%",
      height: 44,
      padding: "0 var(--space-4)",
      fontFamily: "var(--font-sans)",
      fontSize: "var(--text-base)",
      color: "var(--foreground)",
      background: "var(--surface-card)",
      border: `1px solid ${invalid ? "var(--destructive)" : "var(--input)"}`,
      borderRadius: "var(--radius-sm)",
      outline: "none",
      transition: "border-color var(--dur-fast), box-shadow var(--dur-fast)",
      ...style
    }
  }, props));
}
Object.assign(__ds_scope, { Input });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/forms/Input.jsx", error: String((e && e.message) || e) }); }

// components/forms/Textarea.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
/** Multi-line input. Mono variant for pasting alert JSON into the Narrator. */
function Textarea({
  invalid = false,
  mono = false,
  rows = 5,
  style,
  ...props
}) {
  return /*#__PURE__*/React.createElement("textarea", _extends({
    rows: rows,
    onFocus: e => {
      e.currentTarget.style.borderColor = "var(--ring)";
      e.currentTarget.style.boxShadow = "var(--shadow-focus)";
    },
    onBlur: e => {
      e.currentTarget.style.borderColor = invalid ? "var(--destructive)" : "var(--input)";
      e.currentTarget.style.boxShadow = "none";
    },
    style: {
      width: "100%",
      padding: "var(--space-3) var(--space-4)",
      fontFamily: mono ? "var(--font-mono)" : "var(--font-sans)",
      fontSize: mono ? "var(--text-sm)" : "var(--text-base)",
      lineHeight: "var(--leading-relaxed)",
      color: "var(--foreground)",
      background: "var(--surface-card)",
      border: `1px solid ${invalid ? "var(--destructive)" : "var(--input)"}`,
      borderRadius: "var(--radius-sm)",
      outline: "none",
      resize: "vertical",
      transition: "border-color var(--dur-fast), box-shadow var(--dur-fast)",
      ...style
    }
  }, props));
}
Object.assign(__ds_scope, { Textarea });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/forms/Textarea.jsx", error: String((e && e.message) || e) }); }

// components/surfaces/Card.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
/**
 * Card — the base surface. White, hairline ring, soft corners,
 * subtle shadow. `interactive` adds the brand's hover lift
 * (border appears + faint raise).
 */
function Card({
  interactive = false,
  padding = "lg",
  children,
  style,
  ...props
}) {
  const pads = {
    none: 0,
    sm: "var(--space-4)",
    md: "var(--space-6)",
    lg: "var(--space-8)"
  };
  return /*#__PURE__*/React.createElement("div", _extends({
    onMouseEnter: interactive ? e => {
      e.currentTarget.style.borderColor = "var(--border)";
      e.currentTarget.style.boxShadow = "var(--shadow-md)";
    } : undefined,
    onMouseLeave: interactive ? e => {
      e.currentTarget.style.borderColor = "transparent";
      e.currentTarget.style.boxShadow = "var(--shadow-sm)";
    } : undefined,
    style: {
      background: interactive ? "var(--surface-sunken)" : "var(--surface-card)",
      color: "var(--foreground)",
      borderRadius: "var(--radius-xl)",
      border: interactive ? "1px solid transparent" : "1px solid var(--border-soft)",
      boxShadow: "var(--shadow-sm)",
      padding: pads[padding],
      transition: "border-color var(--dur-base), box-shadow var(--dur-base)",
      ...style
    }
  }, props), children);
}
Object.assign(__ds_scope, { Card });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/surfaces/Card.jsx", error: String((e && e.message) || e) }); }

// components/surfaces/FeatureCard.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
/**
 * FeatureCard — the marketing feature tile. Large 32px radius,
 * sunken grey fill that reveals a border on hover, and a circular
 * white icon chip that scales up on hover. Pass any line icon
 * (e.g. a Lucide <i data-lucide> or an <svg>) as `icon`.
 */
function FeatureCard({
  icon,
  title,
  children,
  style,
  ...props
}) {
  return /*#__PURE__*/React.createElement("div", _extends({
    onMouseEnter: e => {
      e.currentTarget.style.borderColor = "var(--border)";
      e.currentTarget.style.background = "var(--surface-hover)";
      const chip = e.currentTarget.querySelector("[data-chip]");
      if (chip) chip.style.transform = "scale(1.1)";
    },
    onMouseLeave: e => {
      e.currentTarget.style.borderColor = "transparent";
      e.currentTarget.style.background = "var(--surface-sunken)";
      const chip = e.currentTarget.querySelector("[data-chip]");
      if (chip) chip.style.transform = "scale(1)";
    },
    style: {
      background: "var(--surface-sunken)",
      border: "1px solid transparent",
      borderRadius: "var(--radius-2xl)",
      padding: "var(--space-8)",
      transition: "background var(--dur-base), border-color var(--dur-base)",
      ...style
    }
  }, props), icon && /*#__PURE__*/React.createElement("div", {
    "data-chip": true,
    style: {
      display: "inline-flex",
      alignItems: "center",
      justifyContent: "center",
      width: 48,
      height: 48,
      marginBottom: "var(--space-6)",
      borderRadius: "var(--radius-pill)",
      background: "var(--white)",
      color: "var(--primary)",
      border: "1px solid var(--border)",
      boxShadow: "var(--shadow-sm)",
      transition: "transform var(--dur-base)"
    }
  }, icon), /*#__PURE__*/React.createElement("h3", {
    style: {
      fontSize: "var(--text-2xl)",
      fontWeight: "var(--weight-medium)",
      letterSpacing: "var(--tracking-tight)",
      margin: "0 0 var(--space-3)"
    }
  }, title), /*#__PURE__*/React.createElement("p", {
    style: {
      fontSize: "var(--text-lg)",
      color: "var(--text-muted)",
      lineHeight: "var(--leading-relaxed)",
      margin: 0
    }
  }, children));
}
Object.assign(__ds_scope, { FeatureCard });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/surfaces/FeatureCard.jsx", error: String((e && e.message) || e) }); }

// ui_kits/narrator/app.jsx
try { (() => {
/* Narrator console shell — paste alert JSON, run inference, view the SAR narrative. */
const C = window.OpensourceFinanceOspreyDesignSystem_08d2ca;

/* brand illustrations (inline, currentColor) */
const ILL = {
  scrutiny: '<path d="M13 8 H27 L33 14 V40 H13 Z"/><path d="M27 8 V14 H33"/><path d="M18 18 H26 M18 22.5 H24"/><circle cx="26.5" cy="30" r="6"/><path d="M31 34.5 L36 39.5"/>',
  balance: '<path d="M24 11 V39"/><path d="M17 40 H31"/><path d="M12 16 H36"/><circle cx="24" cy="11" r="2.4" fill="currentColor"/><path d="M12 16 L7 26 M12 16 L17 26 M7 26 A6 6 0 0 0 17 26"/><path d="M36 16 L31 26 M36 16 L41 26 M31 26 A6 6 0 0 0 41 26"/>',
  network: '<path d="M24 24 L10 15 M24 24 L38 15 M24 24 L13 37 M24 24 L35 37"/><circle cx="10" cy="14" r="3.4"/><circle cx="38" cy="14" r="3.4"/><circle cx="12" cy="38" r="3.4"/><circle cx="36" cy="38" r="3.4"/><circle cx="24" cy="24" r="3.4" fill="currentColor"/>',
  records: '<path d="M24 14 C20 11 12 11 8 13 V36 C12 34 20 34 24 37 C28 34 36 34 40 36 V13 C36 11 28 11 24 14 Z"/><path d="M24 14 V37"/><path d="M12 19 H19 M12 24 H19 M29 19 H36 M29 24 H36"/>'
};
const Ill = ({
  id,
  size = 28,
  color = "currentColor"
}) => /*#__PURE__*/React.createElement("svg", {
  width: size,
  height: size,
  viewBox: "0 0 48 48",
  fill: "none",
  stroke: color,
  strokeWidth: "2.2",
  strokeLinecap: "round",
  strokeLinejoin: "round",
  dangerouslySetInnerHTML: {
    __html: ILL[id]
  }
});
function Topbar() {
  return /*#__PURE__*/React.createElement("header", {
    style: {
      display: "flex",
      alignItems: "center",
      justifyContent: "space-between",
      gap: 16,
      padding: "14px 24px",
      borderBottom: "1px solid var(--border)",
      background: "color-mix(in srgb,var(--background) 80%,transparent)",
      backdropFilter: "blur(var(--blur-glass))",
      position: "sticky",
      top: 0,
      zIndex: 50
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      alignItems: "center",
      gap: 12
    }
  }, /*#__PURE__*/React.createElement(C.OspreyMark, {
    size: 22
  }), /*#__PURE__*/React.createElement("span", {
    style: {
      fontWeight: 700,
      letterSpacing: "var(--tracking-tight)",
      fontSize: "var(--text-lg)"
    }
  }, "Osprey ", /*#__PURE__*/React.createElement("span", {
    style: {
      fontFamily: "var(--font-serif)",
      fontStyle: "italic",
      fontWeight: 400,
      color: "var(--primary)"
    }
  }, "Narrator")), /*#__PURE__*/React.createElement(C.Badge, {
    variant: "neutral"
  }, "v0.1 \xB7 Qwen3-4B LoRA")), /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      alignItems: "center",
      gap: 16,
      fontSize: "var(--text-xs)",
      color: "var(--text-muted)",
      fontFamily: "var(--font-mono)"
    }
  }, /*#__PURE__*/React.createElement("span", null, "PPL 2.65"), /*#__PURE__*/React.createElement("span", null, "\xB7"), /*#__PURE__*/React.createElement("span", null, "12 rules \xB7 6 typologies")));
}
function Console() {
  const [json, setJson] = React.useState(window.ALERT_JSON);
  const [phase, setPhase] = React.useState("idle"); // idle | running | done
  const run = () => {
    setPhase("running");
    setTimeout(() => setPhase("done"), 1400);
  };
  return /*#__PURE__*/React.createElement("div", {
    style: {
      display: "grid",
      gridTemplateColumns: "minmax(320px, 420px) 1fr",
      minHeight: "calc(100vh - 56px)"
    }
  }, /*#__PURE__*/React.createElement("aside", {
    style: {
      borderRight: "1px solid var(--border)",
      padding: 24,
      display: "flex",
      flexDirection: "column",
      gap: 16,
      background: "var(--surface-sunken)"
    }
  }, /*#__PURE__*/React.createElement(C.Eyebrow, null, "Osprey alert \xB7 input"), /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      gap: 8
    }
  }, /*#__PURE__*/React.createElement(C.Badge, {
    variant: "soft"
  }, "\uD83E\uDD17 HuggingFace"), /*#__PURE__*/React.createElement(C.Badge, {
    variant: "neutral"
  }, "\uD83E\uDD99 Ollama \xB7 Q4_K_M")), /*#__PURE__*/React.createElement(C.Textarea, {
    mono: true,
    rows: 16,
    value: json,
    onChange: e => setJson(e.target.value),
    style: {
      flex: 1
    }
  }), /*#__PURE__*/React.createElement(C.Button, {
    fullWidth: true,
    size: "lg",
    onClick: run,
    disabled: phase === "running"
  }, phase === "running" ? "Generating narrative…" : phase === "done" ? "Regenerate ↻" : "Generate narrative"), /*#__PURE__*/React.createElement("p", {
    style: {
      fontSize: "var(--text-xs)",
      color: "var(--text-muted)",
      margin: 0,
      lineHeight: "var(--leading-relaxed)"
    }
  }, "From alert to narrative in seconds, not hours. Trained on synthetic data only \u2014 review before filing.")), /*#__PURE__*/React.createElement("main", {
    style: {
      padding: "40px 32px",
      overflowY: "auto",
      background: "var(--background)"
    }
  }, phase === "idle" && /*#__PURE__*/React.createElement("div", {
    style: {
      height: "100%",
      minHeight: 420,
      display: "flex",
      flexDirection: "column",
      alignItems: "center",
      justifyContent: "center",
      textAlign: "center",
      gap: 20
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      width: 96,
      height: 96,
      borderRadius: 28,
      background: "var(--primary-soft)",
      display: "flex",
      alignItems: "center",
      justifyContent: "center",
      color: "var(--primary)"
    }
  }, /*#__PURE__*/React.createElement(Ill, {
    id: "scrutiny",
    size: 48
  })), /*#__PURE__*/React.createElement("div", {
    style: {
      maxWidth: 380
    }
  }, /*#__PURE__*/React.createElement("h3", {
    style: {
      fontSize: "var(--text-xl)",
      fontWeight: 500,
      letterSpacing: "var(--tracking-tight)",
      margin: "0 0 6px",
      color: "var(--foreground)"
    }
  }, "Awaiting an alert"), /*#__PURE__*/React.createElement("p", {
    style: {
      margin: 0,
      fontSize: "var(--text-base)",
      color: "var(--text-muted)",
      lineHeight: "var(--leading-relaxed)"
    }
  }, "Paste an Osprey evaluation on the left and generate an analyst-ready SAR narrative in seconds.")), /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      gap: 10,
      marginTop: 4
    }
  }, [["balance", "12 rules"], ["network", "6 typologies"], ["records", "Narrative"]].map(([id, t]) => /*#__PURE__*/React.createElement("div", {
    key: id,
    style: {
      display: "flex",
      flexDirection: "column",
      alignItems: "center",
      gap: 8,
      width: 92,
      padding: "14px 8px",
      borderRadius: "var(--radius-md)",
      border: "1px solid var(--border-soft)",
      background: "var(--surface-card)",
      color: "var(--text-muted)"
    }
  }, /*#__PURE__*/React.createElement(Ill, {
    id: id,
    size: 26
  }), /*#__PURE__*/React.createElement("span", {
    style: {
      fontSize: "var(--text-xs)",
      fontFamily: "var(--font-mono)",
      letterSpacing: "0.04em"
    }
  }, t))))), phase === "running" && /*#__PURE__*/React.createElement("div", {
    style: {
      height: "100%",
      minHeight: 420,
      display: "flex",
      flexDirection: "column",
      alignItems: "center",
      justifyContent: "center",
      gap: 16,
      color: "var(--text-muted)"
    }
  }, /*#__PURE__*/React.createElement("svg", {
    width: "32",
    height: "32",
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "var(--primary)",
    strokeWidth: "2.5",
    strokeLinecap: "round",
    style: {
      animation: "nar-spin .8s linear infinite"
    }
  }, /*#__PURE__*/React.createElement("path", {
    d: "M12 2a10 10 0 0 1 10 10"
  })), /*#__PURE__*/React.createElement("span", {
    style: {
      fontFamily: "var(--font-mono)",
      fontSize: "var(--text-sm)"
    }
  }, "running inference \xB7 1,024 tokens\u2026")), phase === "done" && /*#__PURE__*/React.createElement(window.SarReport, null)), /*#__PURE__*/React.createElement("style", null, `@keyframes nar-spin{to{transform:rotate(360deg)}}`));
}
function NarratorApp() {
  return /*#__PURE__*/React.createElement("div", {
    style: {
      fontFamily: "var(--font-sans)",
      color: "var(--foreground)",
      background: "var(--background)",
      minHeight: "100vh"
    }
  }, /*#__PURE__*/React.createElement(Topbar, null), /*#__PURE__*/React.createElement(Console, null));
}
ReactDOM.createRoot(document.getElementById("root")).render(/*#__PURE__*/React.createElement(NarratorApp, null));
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/narrator/app.jsx", error: String((e && e.message) || e) }); }

// ui_kits/narrator/narrator.jsx
try { (() => {
/* Osprey Narrator — SAR-narrative console.
   Grounded in MODEL_CARD.md: alert JSON in → structured SAR narrative out
   (Alert Summary · Transaction Details · Risk Assessment · Rules · Typologies · Narrative · Actions). */
const NS = window.OpensourceFinanceOspreyDesignSystem_08d2ca;
const {
  Button,
  Badge,
  Eyebrow,
  Textarea,
  OspreyMark,
  Wordmark
} = NS;
const ALERT_JSON = `{
  "decision": "ALRT",
  "composite_score": 724.50,
  "threshold": 500.0,
  "transaction": {
    "type": "wire_transfer",
    "amount": 147500.00,
    "currency": "USD",
    "originator": { "name": "Atlas Global Ventures", "country": "AE" },
    "beneficiary": { "name": "Pinnacle Commodities Ltd.", "country": "NG" }
  },
  "alert_id": "eval-7a3b2c1d-4e5f-6a7b-8c9d-0e1f2a3b4c5d"
}`;
const RULES = [["FATF-R001", "High-Value Transaction", 0.875, true], ["FATF-R002", "Structuring Detection", 0.120, false], ["FATF-R003", "Rapid Movement of Funds", 0.720, true], ["FATF-R004", "Geographic Risk", 0.810, true], ["FATF-R005", "Unusual Transaction Pattern", 0.640, true], ["FATF-R006", "Shell Company Indicator", 0.690, true], ["FATF-R007", "PEP Transaction", 0.080, false], ["FATF-R008", "Round Amount Transaction", 0.550, true], ["FATF-R009", "Cross-Border Wire Transfer", 0.780, true], ["FATF-R010", "Dormant Account Activity", 0.040, false], ["FATF-R011", "Currency Exchange Anomaly", 0.210, false], ["FATF-R012", "Third-Party Payment", 0.190, false]];
const TYPOLOGIES = [["FATF-T001", "Structuring / Smurfing", "No match"], ["FATF-T002", "Trade-Based Money Laundering", "Match"], ["FATF-T003", "Shell Company Layering", "Match"], ["FATF-T004", "PEP Corruption Proceeds", "No match"], ["FATF-T005", "Funnel Account Activity", "Partial"], ["FATF-T006", "Currency Exchange Laundering", "No match"]];
const matchTone = {
  "Match": {
    c: "var(--destructive)",
    b: "color-mix(in oklch,var(--destructive) 12%,transparent)"
  },
  "Partial": {
    c: "#b45309",
    b: "#fffbeb"
  },
  "No match": {
    c: "var(--text-muted)",
    b: "var(--surface-sunken)"
  }
};

/* ---------- report ---------- */
function SectionTitle({
  n,
  children
}) {
  return /*#__PURE__*/React.createElement("h3", {
    style: {
      display: "flex",
      alignItems: "center",
      gap: 10,
      fontSize: "var(--text-xs)",
      fontWeight: 700,
      textTransform: "uppercase",
      letterSpacing: "var(--tracking-wider)",
      color: "var(--text-muted)",
      margin: "0 0 14px"
    }
  }, /*#__PURE__*/React.createElement("span", {
    style: {
      display: "inline-flex",
      alignItems: "center",
      justifyContent: "center",
      width: 20,
      height: 20,
      borderRadius: 6,
      background: "var(--primary-soft)",
      color: "var(--primary)",
      fontSize: "var(--text-2xs)"
    }
  }, n), children);
}
const Block = ({
  children,
  last
}) => /*#__PURE__*/React.createElement("div", {
  style: {
    paddingBottom: last ? 0 : 28,
    marginBottom: last ? 0 : 28,
    borderBottom: last ? "none" : "1px solid var(--border-soft)"
  }
}, children);
function KV({
  k,
  v,
  accent
}) {
  return /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      justifyContent: "space-between",
      gap: 16,
      padding: "7px 0",
      fontSize: "var(--text-sm)",
      borderBottom: "1px solid var(--border-soft)"
    }
  }, /*#__PURE__*/React.createElement("span", {
    style: {
      color: "var(--text-muted)"
    }
  }, k), /*#__PURE__*/React.createElement("span", {
    style: {
      fontWeight: 600,
      color: accent ? "var(--primary)" : "var(--foreground)",
      fontFamily: k.includes("ID") ? "var(--font-mono)" : "inherit",
      fontSize: k.includes("ID") ? "var(--text-xs)" : "var(--text-sm)"
    }
  }, v));
}
function SarReport() {
  const triggered = RULES.filter(r => r[3]).length;
  return /*#__PURE__*/React.createElement("div", {
    style: {
      maxWidth: 720,
      margin: "0 auto"
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      alignItems: "center",
      justifyContent: "space-between",
      flexWrap: "wrap",
      gap: 12,
      marginBottom: 24
    }
  }, /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement(Eyebrow, {
    tone: "accent"
  }, "Suspicious Activity Report \u2014 Draft"), /*#__PURE__*/React.createElement("h2", {
    style: {
      fontSize: "var(--text-3xl)",
      fontWeight: 500,
      letterSpacing: "var(--tracking-tight)",
      margin: "6px 0 0"
    }
  }, "Atlas Global Ventures \u2192 Pinnacle Commodities")), /*#__PURE__*/React.createElement(Badge, {
    variant: "solid"
  }, "ALRT")), /*#__PURE__*/React.createElement(Block, null, /*#__PURE__*/React.createElement(SectionTitle, {
    n: "1"
  }, "Alert Summary"), /*#__PURE__*/React.createElement(KV, {
    k: "Alert ID",
    v: "eval-7a3b2c1d-\u2026-0e1f2a3b4c5d"
  }), /*#__PURE__*/React.createElement(KV, {
    k: "Transaction Type",
    v: "wire_transfer"
  }), /*#__PURE__*/React.createElement(KV, {
    k: "Amount",
    v: "147,500.00 USD"
  }), /*#__PURE__*/React.createElement(KV, {
    k: "Decision",
    v: "ALRT (Alert Generated)",
    accent: true
  }), /*#__PURE__*/React.createElement(KV, {
    k: "Composite Score",
    v: "724.50  \xB7  threshold 500.0",
    accent: true
  })), /*#__PURE__*/React.createElement(Block, null, /*#__PURE__*/React.createElement(SectionTitle, {
    n: "2"
  }, "Transaction Details"), /*#__PURE__*/React.createElement("p", {
    style: {
      margin: 0,
      fontSize: "var(--text-base)",
      lineHeight: "var(--leading-relaxed)",
      color: "var(--text-body)"
    }
  }, "A wire transfer of ", /*#__PURE__*/React.createElement("b", null, "147,500.00 USD"), " was initiated by ", /*#__PURE__*/React.createElement("b", null, "Atlas Global Ventures"), " (AE) to ", /*#__PURE__*/React.createElement("b", null, "Pinnacle Commodities Ltd."), " (NG). The transaction involves a cross-border movement between two higher-risk jurisdictions, settled in a single round-figure instruction.")), /*#__PURE__*/React.createElement(Block, null, /*#__PURE__*/React.createElement(SectionTitle, {
    n: "3"
  }, "Risk Assessment"), /*#__PURE__*/React.createElement("p", {
    style: {
      margin: 0,
      fontSize: "var(--text-base)",
      lineHeight: "var(--leading-relaxed)",
      color: "var(--text-body)"
    }
  }, "The composite risk score of ", /*#__PURE__*/React.createElement("b", null, "724.50"), " significantly exceeds the alert threshold of 500.0, indicating elevated risk. Seven of twelve FATF rules were triggered, with ", /*#__PURE__*/React.createElement("b", null, "High-Value Transaction"), " (0.875) and ", /*#__PURE__*/React.createElement("b", null, "Geographic Risk"), " (0.810) contributing the highest individual scores.")), /*#__PURE__*/React.createElement(Block, null, /*#__PURE__*/React.createElement(SectionTitle, {
    n: "4"
  }, "Rules Triggered ", /*#__PURE__*/React.createElement("span", {
    style: {
      textTransform: "none",
      letterSpacing: 0,
      color: "var(--primary)",
      marginLeft: 4
    }
  }, "\xB7 ", triggered, "/12")), /*#__PURE__*/React.createElement("div", {
    style: {
      overflow: "hidden",
      borderRadius: "var(--radius-md)",
      border: "1px solid var(--border)"
    }
  }, /*#__PURE__*/React.createElement("table", {
    style: {
      width: "100%",
      borderCollapse: "collapse",
      fontSize: "var(--text-sm)"
    }
  }, /*#__PURE__*/React.createElement("thead", null, /*#__PURE__*/React.createElement("tr", {
    style: {
      background: "var(--surface-sunken)"
    }
  }, /*#__PURE__*/React.createElement("th", {
    style: {
      textAlign: "left",
      padding: "8px 12px",
      fontWeight: 600,
      color: "var(--text-muted)",
      fontSize: "var(--text-xs)"
    }
  }, "Rule"), /*#__PURE__*/React.createElement("th", {
    style: {
      textAlign: "right",
      padding: "8px 12px",
      fontWeight: 600,
      color: "var(--text-muted)",
      fontSize: "var(--text-xs)"
    }
  }, "Score"), /*#__PURE__*/React.createElement("th", {
    style: {
      textAlign: "right",
      padding: "8px 12px",
      fontWeight: 600,
      color: "var(--text-muted)",
      fontSize: "var(--text-xs)"
    }
  }, "Triggered"))), /*#__PURE__*/React.createElement("tbody", null, RULES.map(r => /*#__PURE__*/React.createElement("tr", {
    key: r[0],
    style: {
      borderTop: "1px solid var(--border-soft)",
      opacity: r[3] ? 1 : 0.5
    }
  }, /*#__PURE__*/React.createElement("td", {
    style: {
      padding: "8px 12px"
    }
  }, /*#__PURE__*/React.createElement("span", {
    style: {
      fontFamily: "var(--font-mono)",
      fontSize: "var(--text-xs)",
      color: "var(--text-muted)"
    }
  }, r[0]), " ", r[1]), /*#__PURE__*/React.createElement("td", {
    style: {
      padding: "8px 12px",
      textAlign: "right",
      fontFamily: "var(--font-mono)",
      fontWeight: 600,
      color: r[2] >= 0.6 ? "var(--primary)" : "var(--foreground)"
    }
  }, r[2].toFixed(3)), /*#__PURE__*/React.createElement("td", {
    style: {
      padding: "8px 12px",
      textAlign: "right"
    }
  }, r[3] ? /*#__PURE__*/React.createElement("span", {
    style: {
      color: "var(--destructive)",
      fontWeight: 600
    }
  }, "Yes") : /*#__PURE__*/React.createElement("span", {
    style: {
      color: "var(--text-muted)"
    }
  }, "No")))))))), /*#__PURE__*/React.createElement(Block, null, /*#__PURE__*/React.createElement(SectionTitle, {
    n: "5"
  }, "Typology Analysis"), /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      flexDirection: "column",
      gap: 8
    }
  }, TYPOLOGIES.map(t => {
    const tone = matchTone[t[2]];
    return /*#__PURE__*/React.createElement("div", {
      key: t[0],
      style: {
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        gap: 12,
        padding: "10px 14px",
        borderRadius: "var(--radius-sm)",
        border: "1px solid var(--border-soft)",
        background: "var(--surface-card)"
      }
    }, /*#__PURE__*/React.createElement("span", {
      style: {
        fontSize: "var(--text-sm)"
      }
    }, /*#__PURE__*/React.createElement("span", {
      style: {
        fontFamily: "var(--font-mono)",
        fontSize: "var(--text-xs)",
        color: "var(--text-muted)"
      }
    }, t[0]), " ", t[1]), /*#__PURE__*/React.createElement("span", {
      style: {
        fontSize: "var(--text-xs)",
        fontWeight: 600,
        padding: "3px 10px",
        borderRadius: 999,
        color: tone.c,
        background: tone.b
      }
    }, t[2]));
  }))), /*#__PURE__*/React.createElement(Block, null, /*#__PURE__*/React.createElement(SectionTitle, {
    n: "6"
  }, "Narrative"), /*#__PURE__*/React.createElement("p", {
    style: {
      margin: 0,
      fontSize: "var(--text-base)",
      lineHeight: "var(--leading-relaxed)",
      color: "var(--text-body)"
    }
  }, "The structure of this transaction is consistent with ", /*#__PURE__*/React.createElement("b", null, "trade-based money laundering"), " layered through a likely ", /*#__PURE__*/React.createElement("b", null, "shell-company"), " arrangement. A newly active corporate originator in the UAE remitting a large, round-figure sum to a commodities entity in Nigeria \u2014 absent supporting trade documentation \u2014 mirrors funnel-account behaviour. Combined with the geographic and rapid-movement signals, the pattern warrants escalation.")), /*#__PURE__*/React.createElement(Block, {
    last: true
  }, /*#__PURE__*/React.createElement(SectionTitle, {
    n: "7"
  }, "Recommended Actions"), /*#__PURE__*/React.createElement("ul", {
    style: {
      margin: 0,
      paddingLeft: 18,
      fontSize: "var(--text-base)",
      lineHeight: "var(--leading-relaxed)",
      color: "var(--text-body)",
      display: "flex",
      flexDirection: "column",
      gap: 8
    }
  }, /*#__PURE__*/React.createElement("li", null, "Escalate to a senior compliance officer for SAR filing review."), /*#__PURE__*/React.createElement("li", null, "Request enhanced due diligence (EDD) on both counterparties."), /*#__PURE__*/React.createElement("li", null, "Investigate related transactions within the past 90 days."), /*#__PURE__*/React.createElement("li", null, "Hold settlement pending documentary evidence of the underlying trade."))));
}
Object.assign(window, {
  SarReport,
  ALERT_JSON
});
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/narrator/narrator.jsx", error: String((e && e.message) || e) }); }

// ui_kits/website/app.jsx
try { (() => {
/* opensource.finance — full landing page composition. */
function Landing() {
  const {
    Header,
    Hero,
    Credibility,
    Features,
    TransactionDemo,
    Comparison,
    PlatformVision,
    FounderStory,
    Footer
  } = window;
  return /*#__PURE__*/React.createElement("div", {
    style: {
      background: "var(--background)",
      color: "var(--foreground)",
      fontFamily: "var(--font-sans)",
      minHeight: "100vh"
    }
  }, /*#__PURE__*/React.createElement(Header, null), /*#__PURE__*/React.createElement(Hero, null), /*#__PURE__*/React.createElement(Credibility, null), /*#__PURE__*/React.createElement(Features, null), /*#__PURE__*/React.createElement(TransactionDemo, null), /*#__PURE__*/React.createElement(Comparison, null), /*#__PURE__*/React.createElement(PlatformVision, null), /*#__PURE__*/React.createElement(FounderStory, null), /*#__PURE__*/React.createElement(Footer, null));
}
ReactDOM.createRoot(document.getElementById("root")).render(/*#__PURE__*/React.createElement(Landing, null));
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/website/app.jsx", error: String((e && e.message) || e) }); }

// ui_kits/website/demo.jsx
try { (() => {
/* The "god view" — Osprey intercepting a transaction. Looping state machine.
   Recreation of src/components/landing/transaction-demo.tsx using CSS transitions. */
const {
  Terminal: OspreyTerminal,
  Eyebrow: DemoEyebrow
} = window.OpensourceFinanceOspreyDesignSystem_08d2ca;
const FiSpinner = () => /*#__PURE__*/React.createElement("svg", {
  width: "16",
  height: "16",
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: "currentColor",
  strokeWidth: "2.5",
  strokeLinecap: "round",
  style: {
    animation: "osf-spin 0.8s linear infinite"
  }
}, /*#__PURE__*/React.createElement("path", {
  d: "M12 2a10 10 0 0 1 10 10"
}));
const FiCheck = ({
  c = "var(--sys-green)"
}) => /*#__PURE__*/React.createElement("svg", {
  width: "16",
  height: "16",
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: c,
  strokeWidth: "2.5",
  strokeLinecap: "round",
  strokeLinejoin: "round"
}, /*#__PURE__*/React.createElement("path", {
  d: "M22 11.08V12a10 10 0 1 1-5.93-9.14"
}), /*#__PURE__*/React.createElement("polyline", {
  points: "22 4 12 14.01 9 11.01"
}));
const FiShield = ({
  s = 18,
  c = "currentColor"
}) => /*#__PURE__*/React.createElement("svg", {
  width: s,
  height: s,
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: c,
  strokeWidth: "2",
  strokeLinecap: "round",
  strokeLinejoin: "round"
}, /*#__PURE__*/React.createElement("path", {
  d: "M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"
}));
const LOGS = [{
  text: "ORIGIN_IP: 192.168.1.42 (US-WEST)",
  status: "OK",
  tone: "ok"
}, {
  text: "DEVICE_SIG: SHA-256 MATCH",
  status: "OK",
  tone: "ok"
}, {
  text: "BEHAVIOR_MODEL: DEVIATION +0.7σ",
  status: "WARN",
  tone: "warn"
}, {
  text: "RISK_SCORE: 12.4 (THRESHOLD 85)",
  status: "OK",
  tone: "ok"
}];
function Phone({
  children,
  dark,
  style
}) {
  return /*#__PURE__*/React.createElement("div", {
    style: {
      width: 280,
      height: 540,
      background: dark ? "var(--ink)" : "var(--white)",
      borderRadius: 40,
      padding: 12,
      boxShadow: "var(--shadow-2xl)",
      border: dark ? "none" : "1px solid var(--border)",
      flexShrink: 0,
      ...style
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      width: "100%",
      height: "100%",
      background: dark ? "#fff" : "var(--sys-mist)",
      borderRadius: 32,
      overflow: "hidden",
      display: "flex",
      flexDirection: "column"
    }
  }, children));
}
function SenderDevice({
  step
}) {
  return /*#__PURE__*/React.createElement(Phone, {
    dark: true
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      height: 56,
      display: "flex",
      alignItems: "center",
      justifyContent: "center",
      borderBottom: "1px solid #f1f5f9"
    }
  }, /*#__PURE__*/React.createElement("span", {
    style: {
      fontSize: "var(--text-xs)",
      fontWeight: 700,
      color: "var(--ink)"
    }
  }, "Nova Bank")), /*#__PURE__*/React.createElement("div", {
    style: {
      padding: 24,
      flex: 1,
      display: "flex",
      flexDirection: "column",
      alignItems: "center",
      justifyContent: "center",
      gap: 32
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      textAlign: "center"
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      width: 64,
      height: 64,
      borderRadius: "50%",
      background: "#f1f5f9",
      margin: "0 auto",
      display: "flex",
      alignItems: "center",
      justifyContent: "center",
      fontSize: 26
    }
  }, "\uD83D\uDC64"), /*#__PURE__*/React.createElement("p", {
    style: {
      fontSize: "var(--text-sm)",
      fontWeight: 700,
      color: "var(--ink)",
      margin: "8px 0 0"
    }
  }, "Alice V.")), /*#__PURE__*/React.createElement("div", {
    style: {
      textAlign: "center"
    }
  }, /*#__PURE__*/React.createElement("p", {
    style: {
      fontSize: "var(--text-2xs)",
      fontWeight: 700,
      textTransform: "uppercase",
      letterSpacing: "var(--tracking-wider)",
      color: "#94a3b8",
      margin: "0 0 4px"
    }
  }, "Sending"), /*#__PURE__*/React.createElement("div", {
    style: {
      fontSize: "2.25rem",
      fontWeight: 700,
      letterSpacing: "-0.02em",
      color: "var(--ink)"
    }
  }, "$2,450.00")), /*#__PURE__*/React.createElement("button", {
    style: {
      width: "100%",
      height: 48,
      borderRadius: 12,
      border: "none",
      fontSize: "var(--text-sm)",
      fontWeight: 700,
      cursor: "default",
      display: "flex",
      alignItems: "center",
      justifyContent: "center",
      gap: 8,
      transition: "all .3s",
      background: step === 0 ? "#0a0a0a" : "#f1f5f9",
      color: step === 0 ? "#fff" : "#94a3b8"
    }
  }, step === 0 && "Send Money", step === 1 && /*#__PURE__*/React.createElement(React.Fragment, null, /*#__PURE__*/React.createElement(FiSpinner, null), /*#__PURE__*/React.createElement("span", null, "Sending\u2026")), step === 2 && /*#__PURE__*/React.createElement("span", {
    style: {
      color: "#64748b"
    }
  }, "Held for verification"), step >= 3 && /*#__PURE__*/React.createElement(React.Fragment, null, /*#__PURE__*/React.createElement(FiCheck, null), /*#__PURE__*/React.createElement("span", {
    style: {
      color: "var(--ink)"
    }
  }, "Approved \xB7 Sent")))));
}
function ReceiverDevice({
  step
}) {
  return /*#__PURE__*/React.createElement(Phone, null, /*#__PURE__*/React.createElement("div", {
    style: {
      height: 56,
      display: "flex",
      alignItems: "center",
      justifyContent: "center",
      borderBottom: "1px solid #f1f5f9"
    }
  }, /*#__PURE__*/React.createElement("span", {
    style: {
      fontSize: "var(--text-xs)",
      fontWeight: 700,
      color: "#94a3b8"
    }
  }, "Notifications")), /*#__PURE__*/React.createElement("div", {
    style: {
      padding: 16,
      flex: 1,
      display: "flex",
      flexDirection: "column",
      alignItems: "center",
      paddingTop: 48
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      width: "100%",
      background: "#fff",
      padding: 16,
      borderRadius: 16,
      boxShadow: "var(--shadow-lg)",
      border: "1px solid #f1f5f9",
      transition: "all .4s var(--ease-out)",
      opacity: step >= 5 ? 1 : 0,
      transform: step >= 5 ? "translateY(0) scale(1)" : "translateY(-16px) scale(0.95)"
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      gap: 12
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      width: 40,
      height: 40,
      borderRadius: "50%",
      background: "var(--sys-green)",
      display: "flex",
      alignItems: "center",
      justifyContent: "center",
      color: "#fff",
      flexShrink: 0
    }
  }, /*#__PURE__*/React.createElement(FiShield, {
    c: "#fff"
  })), /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement("p", {
    style: {
      fontSize: "var(--text-xs)",
      fontWeight: 700,
      color: "var(--ink)",
      margin: 0
    }
  }, "Payment Received"), /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      alignItems: "center",
      gap: 6,
      marginTop: 2
    }
  }, /*#__PURE__*/React.createElement(FiShield, {
    s: 10,
    c: "var(--sys-green)"
  }), /*#__PURE__*/React.createElement("p", {
    style: {
      fontSize: "var(--text-2xs)",
      color: "#64748b",
      margin: 0
    }
  }, "Protected transfer")), /*#__PURE__*/React.createElement("p", {
    style: {
      fontSize: "var(--text-lg)",
      fontWeight: 700,
      color: "var(--ink)",
      margin: "4px 0 0"
    }
  }, "$2,450.00"), /*#__PURE__*/React.createElement("p", {
    style: {
      fontSize: "9px",
      color: "#94a3b8",
      margin: "8px 0 0",
      fontFamily: "var(--font-mono)"
    }
  }, "Verified in 118ms"))))));
}
function Intercept({
  step
}) {
  const showTerminal = step === 2 || step === 3;
  return /*#__PURE__*/React.createElement("div", {
    style: {
      flex: 1,
      minWidth: 280,
      maxWidth: 420,
      height: 240,
      display: "flex",
      flexDirection: "column",
      alignItems: "center",
      justifyContent: "center",
      position: "relative"
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      width: "100%",
      height: 6,
      background: "var(--grey-300)",
      borderRadius: 999,
      position: "relative",
      overflow: "hidden",
      marginBottom: 32
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      position: "absolute",
      top: 0,
      bottom: 0,
      width: 96,
      borderRadius: 999,
      background: step === 4 ? "linear-gradient(90deg,transparent,var(--sys-green),transparent)" : "linear-gradient(90deg,transparent,var(--ink),transparent)",
      transition: "left 0.6s linear, opacity .3s",
      opacity: step === 1 || step === 4 ? 1 : 0,
      left: step === 1 ? "50%" : step === 4 ? "200%" : "-100%"
    }
  }), showTerminal && /*#__PURE__*/React.createElement("div", {
    style: {
      position: "absolute",
      top: 0,
      bottom: 0,
      left: "50%",
      transform: "translateX(-50%)",
      width: 16,
      background: "var(--ink)",
      borderRadius: 999
    }
  })), /*#__PURE__*/React.createElement("div", {
    style: {
      position: "absolute",
      top: 0,
      left: "50%",
      transform: "translateX(-50%)",
      width: 340,
      transition: "opacity .4s var(--ease-out), transform .4s var(--ease-out)",
      opacity: showTerminal ? 1 : 0,
      transform: showTerminal ? "translateX(-50%) translateY(0)" : "translateX(-50%) translateY(8px)",
      pointerEvents: "none"
    }
  }, /*#__PURE__*/React.createElement(OspreyTerminal, {
    logs: LOGS,
    decision: step === 3 ? {
      label: "FINAL_DECISION",
      value: "ALLOWED",
      tone: "ok"
    } : {
      label: "FINAL_DECISION",
      value: "ANALYZING…",
      tone: "muted"
    }
  })), /*#__PURE__*/React.createElement("div", {
    style: {
      position: "absolute",
      top: 0,
      left: "50%",
      transform: "translate(-50%,-50%)",
      width: 16,
      height: 16,
      background: "var(--ink)",
      borderRadius: "50%",
      border: "2px solid var(--grey-300)"
    }
  }));
}
function TransactionDemo() {
  const [step, setStep] = React.useState(0);
  React.useEffect(() => {
    let t;
    const run = () => {
      setStep(1);
      t = setTimeout(() => {
        setStep(2);
        t = setTimeout(() => {
          setStep(3);
          t = setTimeout(() => {
            setStep(4);
            t = setTimeout(() => {
              setStep(5);
              t = setTimeout(() => {
                setStep(0);
                t = setTimeout(run, 1000);
              }, 2500);
            }, 500);
          }, 900);
        }, 1900);
      }, 700);
    };
    t = setTimeout(run, 900);
    return () => clearTimeout(t);
  }, []);
  return /*#__PURE__*/React.createElement("section", {
    style: {
      padding: "96px 0",
      background: "var(--sys-mist)",
      overflow: "hidden"
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      maxWidth: 720,
      margin: "0 auto 80px",
      padding: "0 24px",
      textAlign: "center",
      display: "flex",
      flexDirection: "column",
      gap: 16
    }
  }, /*#__PURE__*/React.createElement(DemoEyebrow, {
    tone: "info"
  }, "Active Interception"), /*#__PURE__*/React.createElement("h2", {
    style: {
      fontSize: "clamp(2rem,5vw,3rem)",
      fontWeight: 700,
      letterSpacing: "var(--tracking-tighter)",
      color: "var(--ink)",
      lineHeight: 1.05,
      margin: 0
    }
  }, "Every transaction. Every signal.", /*#__PURE__*/React.createElement("br", null), "Verified before it moves."), /*#__PURE__*/React.createElement("p", {
    style: {
      fontSize: "var(--text-lg)",
      fontWeight: 500,
      color: "#64748b",
      maxWidth: 540,
      margin: "0 auto"
    }
  }, "Osprey sits between intent and execution. Fraud is stopped. Trust is earned. Users never wait.")), /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      flexWrap: "wrap",
      alignItems: "center",
      justifyContent: "center",
      gap: 0,
      maxWidth: 1100,
      margin: "0 auto",
      padding: "0 24px"
    }
  }, /*#__PURE__*/React.createElement(SenderDevice, {
    step: step
  }), /*#__PURE__*/React.createElement(Intercept, {
    step: step
  }), /*#__PURE__*/React.createElement(ReceiverDevice, {
    step: step
  })), /*#__PURE__*/React.createElement("style", null, `@keyframes osf-spin{to{transform:rotate(360deg)}}`));
}
Object.assign(window, {
  TransactionDemo
});
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/website/demo.jsx", error: String((e && e.message) || e) }); }

// ui_kits/website/platform.jsx
try { (() => {
/* Comparison table, platform vision, founder story.
   Recreation of comparison.tsx / platform-vision.tsx / founder-story.tsx. */
const {
  Badge: PBadge,
  Eyebrow: PEyebrow,
  Avatar: PAvatar
} = window.OpensourceFinanceOspreyDesignSystem_08d2ca;
const Check = ({
  c = "var(--sys-green)"
}) => /*#__PURE__*/React.createElement("svg", {
  width: "18",
  height: "18",
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: c,
  strokeWidth: "2.5",
  strokeLinecap: "round",
  strokeLinejoin: "round"
}, /*#__PURE__*/React.createElement("path", {
  d: "M22 11.08V12a10 10 0 1 1-5.93-9.14"
}), /*#__PURE__*/React.createElement("polyline", {
  points: "22 4 12 14.01 9 11.01"
}));
const Cross = () => /*#__PURE__*/React.createElement("svg", {
  width: "18",
  height: "18",
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: "var(--text-muted)",
  strokeWidth: "2.5",
  strokeLinecap: "round",
  strokeLinejoin: "round"
}, /*#__PURE__*/React.createElement("circle", {
  cx: "12",
  cy: "12",
  r: "10"
}), /*#__PURE__*/React.createElement("line", {
  x1: "15",
  y1: "9",
  x2: "9",
  y2: "15"
}), /*#__PURE__*/React.createElement("line", {
  x1: "9",
  y1: "9",
  x2: "15",
  y2: "15"
}));
function Comparison() {
  const rows = [["Deployment", "60 Seconds", "Weeks (K8s)", "Immediate (API)", "Months (Sales)", "Immediate (API)"], ["Architecture", "Single Binary", "7+ Microservices", "SaaS Only", "SaaS Only", "SaaS Only"], ["Team Required", "1 Developer", "Platform Team", "Eng + Compliance", "Eng + Compliance", "Eng + Compliance"], ["Pricing Model", "Open Source", "High Ops Cost", "Vol. Commit", "$30k–$740k/yr", "Tiered / Custom"]];
  const cols = ["Tazama", "Sardine", "Unit21", "ComplyAdvantage"];
  const th = {
    padding: "20px 16px",
    fontSize: "var(--text-lg)",
    fontWeight: 500,
    color: "var(--foreground)",
    borderBottom: "1px solid var(--border)",
    textAlign: "left"
  };
  const td = {
    padding: "20px 16px",
    color: "var(--text-muted)",
    borderBottom: "1px solid var(--border-soft)",
    fontSize: "var(--text-lg)"
  };
  const tdO = {
    padding: "20px 16px",
    color: "var(--primary)",
    fontWeight: 700,
    background: "color-mix(in srgb,var(--white) 50%,transparent)",
    borderLeft: "1px solid color-mix(in oklch,var(--primary) 10%,transparent)",
    borderRight: "1px solid color-mix(in oklch,var(--primary) 10%,transparent)",
    borderBottom: "1px solid color-mix(in oklch,var(--primary) 10%,transparent)",
    fontSize: "var(--text-lg)",
    position: "relative"
  };
  const lead = {
    position: "sticky",
    left: 0,
    padding: "20px 16px",
    fontWeight: 500,
    background: "var(--background)",
    borderBottom: "1px solid var(--border-soft)"
  };
  return /*#__PURE__*/React.createElement("section", {
    id: "comparison",
    style: {
      padding: "96px 0",
      background: "var(--surface-sunken)"
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      maxWidth: "var(--container-wide)",
      margin: "0 auto",
      padding: "0 24px"
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      marginBottom: 56
    }
  }, /*#__PURE__*/React.createElement("h2", {
    style: {
      fontSize: "var(--text-4xl)",
      fontWeight: 500,
      letterSpacing: "var(--tracking-tight)",
      margin: "0 0 16px"
    }
  }, "The Uncomfortable Truth"), /*#__PURE__*/React.createElement("p", {
    style: {
      fontSize: "var(--text-xl)",
      color: "var(--text-muted)",
      maxWidth: 560,
      margin: 0
    }
  }, "Existing solutions were built for banks with unlimited budgets. We built for the rest of us.")), /*#__PURE__*/React.createElement("div", {
    style: {
      overflowX: "auto"
    }
  }, /*#__PURE__*/React.createElement("table", {
    style: {
      width: "100%",
      minWidth: 920,
      borderCollapse: "separate",
      borderSpacing: 0,
      textAlign: "left"
    }
  }, /*#__PURE__*/React.createElement("thead", null, /*#__PURE__*/React.createElement("tr", null, /*#__PURE__*/React.createElement("th", {
    style: {
      ...th,
      fontSize: "var(--text-xs)",
      textTransform: "uppercase",
      letterSpacing: "var(--tracking-wider)",
      color: "var(--text-muted)",
      position: "sticky",
      left: 0,
      background: "var(--background)"
    }
  }, "Feature"), /*#__PURE__*/React.createElement("th", {
    style: {
      ...th,
      color: "var(--primary)",
      fontSize: "var(--text-xl)",
      fontWeight: 700,
      background: "color-mix(in srgb,var(--white) 50%,transparent)",
      borderTop: "4px solid var(--primary)",
      borderLeft: "1px solid color-mix(in oklch,var(--primary) 10%,transparent)",
      borderRight: "1px solid color-mix(in oklch,var(--primary) 10%,transparent)",
      borderRadius: "12px 12px 0 0"
    }
  }, /*#__PURE__*/React.createElement("span", {
    style: {
      display: "inline-flex",
      alignItems: "center",
      gap: 8
    }
  }, "Osprey ", /*#__PURE__*/React.createElement(PBadge, {
    variant: "soft"
  }, "Dev"))), cols.map(c => /*#__PURE__*/React.createElement("th", {
    key: c,
    style: {
      ...th,
      opacity: 0.6
    }
  }, c)))), /*#__PURE__*/React.createElement("tbody", null, rows.map((r, ri) => /*#__PURE__*/React.createElement("tr", {
    key: ri
  }, /*#__PURE__*/React.createElement("td", {
    style: lead
  }, r[0]), /*#__PURE__*/React.createElement("td", {
    style: ri === rows.length - 1 ? {
      ...tdO,
      borderRadius: "0 0 12px 12px"
    } : tdO
  }, r[1]), r.slice(2).map((c, ci) => /*#__PURE__*/React.createElement("td", {
    key: ci,
    style: td
  }, c)))), /*#__PURE__*/React.createElement("tr", null, /*#__PURE__*/React.createElement("td", {
    style: {
      ...lead,
      borderBottom: "none"
    }
  }, "Data Control"), /*#__PURE__*/React.createElement("td", {
    style: {
      ...tdO,
      borderRadius: "0 0 12px 12px",
      borderBottom: "1px solid color-mix(in oklch,var(--primary) 10%,transparent)"
    }
  }, /*#__PURE__*/React.createElement("span", {
    style: {
      display: "inline-flex",
      alignItems: "center",
      gap: 8
    }
  }, /*#__PURE__*/React.createElement(Check, null), "100% Yours")), /*#__PURE__*/React.createElement("td", {
    style: {
      ...td,
      borderBottom: "none"
    }
  }, /*#__PURE__*/React.createElement("span", {
    style: {
      display: "inline-flex",
      alignItems: "center",
      gap: 8
    }
  }, /*#__PURE__*/React.createElement(Check, {
    c: "var(--text-muted)"
  }), "100% Yours")), /*#__PURE__*/React.createElement("td", {
    style: {
      ...td,
      borderBottom: "none"
    }
  }, /*#__PURE__*/React.createElement("span", {
    style: {
      display: "inline-flex",
      alignItems: "center",
      gap: 8
    }
  }, /*#__PURE__*/React.createElement(Cross, null), "Vendor Lock-in")), /*#__PURE__*/React.createElement("td", {
    style: {
      ...td,
      borderBottom: "none"
    }
  }, /*#__PURE__*/React.createElement("span", {
    style: {
      display: "inline-flex",
      alignItems: "center",
      gap: 8
    }
  }, /*#__PURE__*/React.createElement(Cross, null), "Vendor Lock-in")), /*#__PURE__*/React.createElement("td", {
    style: {
      ...td,
      borderBottom: "none"
    }
  }, /*#__PURE__*/React.createElement("span", {
    style: {
      display: "inline-flex",
      alignItems: "center",
      gap: 8
    }
  }, /*#__PURE__*/React.createElement(Cross, null), "Vendor Lock-in"))))))));
}
function PlatformVision() {
  const tiles = [{
    letter: "S",
    title: "Studio",
    sub: "Policy & Rules",
    body: "No-code builder for defining risk controls and typologies without engineering bottlenecks.",
    tone: "muted",
    chip: "Design",
    chipBg: "#eff6ff",
    chipFg: "#1d4ed8",
    lc: "#2563eb",
    lbg: "#eff6ff"
  }, {
    letter: "O",
    title: "Osprey",
    sub: "The Engine",
    body: "High-performance real-time transaction monitoring and screening.",
    tone: "core",
    chip: "Core"
  }, {
    letter: "C",
    title: "Cases",
    sub: "Investigation",
    body: "Streamlined workflow for analysts to review, decision, and report suspicious activity.",
    tone: "muted",
    chip: "Action",
    chipBg: "#fffbeb",
    chipFg: "#b45309",
    lc: "#d97706",
    lbg: "#fffbeb"
  }];
  return /*#__PURE__*/React.createElement("section", {
    id: "vision",
    style: {
      padding: "96px 24px",
      borderBottom: "1px solid var(--border-soft)"
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      maxWidth: "var(--container-content)",
      margin: "0 auto"
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      marginBottom: 48
    }
  }, /*#__PURE__*/React.createElement("h2", {
    style: {
      fontSize: "var(--text-3xl)",
      fontWeight: 500,
      letterSpacing: "var(--tracking-tight)",
      margin: "0 0 16px"
    }
  }, "The Platform Vision"), /*#__PURE__*/React.createElement("p", {
    style: {
      fontSize: "var(--text-lg)",
      color: "var(--text-muted)",
      maxWidth: 560,
      margin: 0
    }
  }, "Open-source infrastructure for the next generation of fintech. We're building the operating system for financial vigilance.")), /*#__PURE__*/React.createElement("div", {
    style: {
      display: "grid",
      gridTemplateColumns: "repeat(auto-fit,minmax(240px,1fr))",
      gap: 24
    }
  }, tiles.map(t => {
    const core = t.tone === "core";
    return /*#__PURE__*/React.createElement("div", {
      key: t.title,
      style: {
        position: "relative",
        overflow: "hidden",
        borderRadius: "var(--radius-xl)",
        padding: 24,
        border: core ? "1px solid color-mix(in oklch,var(--primary) 20%,transparent)" : "1px solid var(--border)",
        background: core ? "var(--primary-soft)" : "color-mix(in srgb,var(--white) 50%,transparent)",
        boxShadow: core ? "var(--shadow-md)" : "var(--shadow-sm)",
        opacity: core ? 1 : 0.55,
        filter: core ? "none" : "grayscale(1) blur(0.4px)"
      }
    }, /*#__PURE__*/React.createElement("div", {
      style: {
        position: "absolute",
        top: 16,
        right: 16
      }
    }, /*#__PURE__*/React.createElement("span", {
      style: {
        fontSize: "var(--text-xs)",
        fontWeight: 500,
        padding: "4px 8px",
        borderRadius: 999,
        background: core ? "var(--primary-soft)" : t.chipBg,
        color: core ? "var(--primary)" : t.chipFg,
        boxShadow: core ? "inset 0 0 0 1px color-mix(in oklch,var(--primary) 20%,transparent)" : "none"
      }
    }, t.chip)), /*#__PURE__*/React.createElement("div", {
      style: {
        width: 40,
        height: 40,
        borderRadius: 10,
        marginBottom: 16,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        fontFamily: "var(--font-serif)",
        fontStyle: "italic",
        fontSize: "var(--text-xl)",
        background: core ? "var(--primary)" : t.lbg,
        color: core ? "#fff" : t.lc
      }
    }, t.letter), /*#__PURE__*/React.createElement("h3", {
      style: {
        fontSize: "var(--text-xl)",
        fontWeight: 700,
        letterSpacing: "var(--tracking-tight)",
        margin: "0 0 8px"
      }
    }, t.title), /*#__PURE__*/React.createElement("p", {
      style: {
        fontSize: "var(--text-sm)",
        color: "var(--text-muted)",
        margin: "0 0 16px"
      }
    }, t.sub), /*#__PURE__*/React.createElement("p", {
      style: {
        fontSize: "var(--text-sm)",
        color: "var(--foreground)",
        opacity: 0.85,
        lineHeight: "var(--leading-relaxed)",
        margin: 0
      }
    }, t.body, core && /*#__PURE__*/React.createElement("span", {
      style: {
        fontWeight: 600,
        color: "var(--primary)"
      }
    }, " In Development.")));
  }))));
}
function FounderStory() {
  const paras = [/*#__PURE__*/React.createElement(React.Fragment, null, "The journey began with the ", /*#__PURE__*/React.createElement("b", null, "LevelOne Project"), ", a ", /*#__PURE__*/React.createElement("b", null, "Gates Foundation"), " initiative to increase financial inclusion \u2014 a low-cost, real-time payment infrastructure for the world's unbanked."), /*#__PURE__*/React.createElement(React.Fragment, null, "I worked as a software engineer on ", /*#__PURE__*/React.createElement("b", null, "Tazama"), ", the transaction monitoring arm of that project. We built a robust, open-source framework to detect fraud across national payment switches."), /*#__PURE__*/React.createElement(React.Fragment, null, "I then moved to the private sector, architecting secure-by-design infrastructure for highly regulated environments \u2014 sovereign-grade security, landing-zone isolation, zero-trust as requirements, not features."), /*#__PURE__*/React.createElement(React.Fragment, null, "But I saw a gap. This level of rigor was completely out of reach for the fintechs I cared about. They were stuck with brittle integrations or expensive enterprise contracts."), /*#__PURE__*/React.createElement(React.Fragment, null, "I built ", /*#__PURE__*/React.createElement("b", null, "Osprey"), " to fulfill the original promise of equity \u2014 re-engineered into a single, efficient binary. No massive integration projects. No enterprise bloat.")];
  return /*#__PURE__*/React.createElement("section", {
    id: "story",
    style: {
      padding: "96px 24px",
      borderTop: "1px solid var(--border-soft)",
      background: "var(--white)"
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      maxWidth: "var(--container-content)",
      margin: "0 auto",
      display: "grid",
      gridTemplateColumns: "repeat(auto-fit,minmax(300px,1fr))",
      gap: 48,
      alignItems: "start"
    }
  }, /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement(PBadge, {
    variant: "soft",
    live: true
  }, "Origin Story"), /*#__PURE__*/React.createElement("h2", {
    style: {
      fontSize: "clamp(2rem,4vw,3rem)",
      fontWeight: 500,
      letterSpacing: "var(--tracking-tight)",
      lineHeight: 1.1,
      margin: "24px 0 0"
    }
  }, "We didn't start with a business plan. ", /*#__PURE__*/React.createElement("span", {
    style: {
      color: "var(--text-muted)"
    }
  }, "We started with a mission."))), /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      flexDirection: "column",
      gap: 24,
      fontSize: "var(--text-lg)",
      color: "var(--text-muted)",
      lineHeight: "var(--leading-relaxed)"
    }
  }, paras.map((p, i) => /*#__PURE__*/React.createElement("p", {
    key: i,
    style: {
      margin: 0
    }
  }, p)), /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      alignItems: "center",
      gap: 16,
      paddingTop: 8
    }
  }, /*#__PURE__*/React.createElement(PAvatar, {
    initials: "JG"
  }), /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement("div", {
    style: {
      fontWeight: 700,
      color: "var(--foreground)"
    }
  }, "Joseph Goksu"), /*#__PURE__*/React.createElement("div", {
    style: {
      fontSize: "var(--text-sm)"
    }
  }, "Founder, opensource.finance"))))), /*#__PURE__*/React.createElement("style", null, `#story b{color:var(--foreground);font-weight:500}`));
}
Object.assign(window, {
  Comparison,
  PlatformVision,
  FounderStory
});
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/website/platform.jsx", error: String((e && e.message) || e) }); }

// ui_kits/website/sections.jsx
try { (() => {
/* opensource.finance landing — header, hero, credibility, features, footer.
   Faithful recreation of src/components/landing/*, composing DS primitives. */
const {
  Button,
  Badge,
  Eyebrow,
  FeatureCard,
  Wordmark,
  Avatar
} = window.OpensourceFinanceOspreyDesignSystem_08d2ca;

/* --- inline Lucide icons (paths lifted from the source) --- */
const Svg = ({
  children,
  w = 24,
  sw = 1.5
}) => /*#__PURE__*/React.createElement("svg", {
  xmlns: "http://www.w3.org/2000/svg",
  width: w,
  height: w,
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: "currentColor",
  strokeWidth: sw,
  strokeLinecap: "round",
  strokeLinejoin: "round"
}, children);
const IconBox = () => /*#__PURE__*/React.createElement(Svg, null, /*#__PURE__*/React.createElement("path", {
  d: "M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"
}), /*#__PURE__*/React.createElement("polyline", {
  points: "3.27 6.96 12 12.01 20.73 6.96"
}), /*#__PURE__*/React.createElement("line", {
  x1: "12",
  y1: "22.08",
  x2: "12",
  y2: "12"
}));
const IconZap = () => /*#__PURE__*/React.createElement(Svg, null, /*#__PURE__*/React.createElement("path", {
  d: "M13 2L3 14h9l-1 8 10-12h-9l1-8z"
}));
const IconLayers = () => /*#__PURE__*/React.createElement(Svg, null, /*#__PURE__*/React.createElement("polygon", {
  points: "12 2 2 7 12 12 22 7 12 2"
}), /*#__PURE__*/React.createElement("polyline", {
  points: "2 17 12 22 22 17"
}), /*#__PURE__*/React.createElement("polyline", {
  points: "2 12 12 17 22 12"
}));
const IconFeather = () => /*#__PURE__*/React.createElement(Svg, null, /*#__PURE__*/React.createElement("path", {
  d: "M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91-.09z"
}), /*#__PURE__*/React.createElement("path", {
  d: "m12 15-3-3a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 0 1-4 2z"
}), /*#__PURE__*/React.createElement("path", {
  d: "M9 12H4s.55-3.03 2-4c1.62-1.08 5 0 5 0"
}), /*#__PURE__*/React.createElement("path", {
  d: "M12 15v5s3.03-.55 4-2c1.08-1.62 0-5 0-5"
}));
const IconGitHub = () => /*#__PURE__*/React.createElement(Svg, {
  sw: 1.8
}, /*#__PURE__*/React.createElement("path", {
  d: "M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"
}));
const wrap = {
  maxWidth: "var(--container-content)",
  margin: "0 auto",
  padding: "0 24px"
};
function Header() {
  const links = ["Features", "Vision", "Comparison", "Story"];
  return /*#__PURE__*/React.createElement("header", {
    style: {
      position: "sticky",
      top: 24,
      zIndex: 50,
      width: "100%",
      display: "flex",
      justifyContent: "center",
      padding: "0 16px"
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      alignItems: "center",
      justifyContent: "space-between",
      gap: 16,
      width: "100%",
      maxWidth: 768,
      borderRadius: "var(--radius-pill)",
      border: "1px solid var(--border-soft)",
      background: "color-mix(in srgb, var(--background) 65%, transparent)",
      backdropFilter: "blur(var(--blur-glass))",
      padding: "8px 8px 8px 22px",
      boxShadow: "var(--shadow-sm)"
    }
  }, /*#__PURE__*/React.createElement(Wordmark, {
    brand: "osf",
    dot: "circle",
    size: 15
  }), /*#__PURE__*/React.createElement("nav", {
    style: {
      display: "flex",
      gap: 24,
      fontSize: "var(--text-sm)",
      fontWeight: 500,
      color: "var(--text-muted)"
    }
  }, links.map(l => /*#__PURE__*/React.createElement("a", {
    key: l,
    href: "#" + l.toLowerCase(),
    style: {
      color: "inherit",
      textDecoration: "none"
    },
    onMouseEnter: e => e.currentTarget.style.color = "var(--foreground)",
    onMouseLeave: e => e.currentTarget.style.color = "var(--text-muted)"
  }, l))), /*#__PURE__*/React.createElement(Button, {
    size: "sm"
  }, "Get Started")));
}
function Hero() {
  return /*#__PURE__*/React.createElement("section", {
    style: {
      padding: "120px 24px 80px"
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      maxWidth: 960,
      margin: "0 auto",
      display: "flex",
      flexDirection: "column",
      gap: 24
    }
  }, /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement(Badge, {
    variant: "soft",
    live: true
  }, "Introducing Osprey")), /*#__PURE__*/React.createElement("h1", {
    style: {
      fontSize: "clamp(3rem, 7vw, 5.5rem)",
      fontWeight: 500,
      letterSpacing: "var(--tracking-tighter)",
      lineHeight: 1.05,
      maxWidth: 900,
      margin: 0
    }
  }, "Transaction monitoring for ", /*#__PURE__*/React.createElement("span", {
    style: {
      fontFamily: "var(--font-serif)",
      fontStyle: "italic",
      fontWeight: 400,
      color: "var(--primary)",
      paddingRight: "0.08em"
    }
  }, "everyone"), " who isn't a bank."), /*#__PURE__*/React.createElement("p", {
    style: {
      fontSize: "var(--text-2xl)",
      color: "var(--text-muted)",
      maxWidth: 640,
      lineHeight: "var(--leading-relaxed)",
      margin: 0
    }
  }, "The open-source infrastructure that deploys in 60 seconds. Single binary. Built in Go. No enterprise bloat."), /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      flexWrap: "wrap",
      gap: 16,
      paddingTop: 8
    }
  }, /*#__PURE__*/React.createElement(Button, {
    size: "lg"
  }, "Get Started"), /*#__PURE__*/React.createElement(Button, {
    size: "lg",
    variant: "outline"
  }, "View on GitHub\xA0\u2192"))));
}
function Credibility() {
  return /*#__PURE__*/React.createElement("section", {
    style: {
      padding: "48px 0",
      borderTop: "1px solid var(--border-soft)",
      borderBottom: "1px solid var(--border-soft)"
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      ...wrap,
      display: "flex",
      flexWrap: "wrap",
      gap: 32,
      alignItems: "center",
      justifyContent: "space-between"
    }
  }, /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement(Eyebrow, null, "Engineered by the"), /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      flexDirection: "column",
      marginTop: 6
    }
  }, /*#__PURE__*/React.createElement("span", {
    style: {
      fontSize: "var(--text-2xl)",
      fontWeight: 700,
      letterSpacing: "var(--tracking-tight)"
    }
  }, "Founding Team of Tazama"), /*#__PURE__*/React.createElement("span", {
    style: {
      fontSize: "var(--text-sm)",
      color: "var(--text-muted)"
    }
  }, "The original real-time transaction monitoring platform"))), /*#__PURE__*/React.createElement("div", {
    style: {
      width: 1,
      height: 48,
      background: "var(--border)"
    }
  }), /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement(Eyebrow, null, "Heritage & Validation"), /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      flexDirection: "column",
      gap: 4,
      marginTop: 6
    }
  }, /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement("span", {
    style: {
      fontWeight: 600
    }
  }, "Bill & Melinda Gates Foundation"), " ", /*#__PURE__*/React.createElement("span", {
    style: {
      fontSize: "var(--text-xs)",
      color: "var(--text-muted)"
    }
  }, "(Original Funding)")), /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement("span", {
    style: {
      fontWeight: 600
    }
  }, "Linux Foundation"), " ", /*#__PURE__*/React.createElement("span", {
    style: {
      fontSize: "var(--text-xs)",
      color: "var(--text-muted)"
    }
  }, "(Current Steward)"))))));
}
function Features() {
  const items = [{
    icon: /*#__PURE__*/React.createElement(IconBox, null),
    title: "Single Binary",
    body: "Compiled Go. No JVM. No node_modules. Just one standard binary that runs anywhere."
  }, {
    icon: /*#__PURE__*/React.createElement(IconZap, null),
    title: "60 Second Deploy",
    body: "From download to monitoring live transactions in less time than it takes to brew coffee."
  }, {
    icon: /*#__PURE__*/React.createElement(IconLayers, null),
    title: "Universal Adapters",
    body: "ISO 20022, JSON, GraphQL, gRPC. Whatever your payment rail speaks, we speak it."
  }, {
    icon: /*#__PURE__*/React.createElement(IconFeather, null),
    title: "No Bloat",
    body: "We stripped out the enterprise complexity. You don't need a Kubernetes cluster to run fraud checks."
  }];
  return /*#__PURE__*/React.createElement("section", {
    id: "features",
    style: {
      padding: "112px 24px"
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      maxWidth: "var(--container-content)",
      margin: "0 auto",
      display: "grid",
      gridTemplateColumns: "repeat(auto-fit,minmax(300px,1fr))",
      gap: 32
    }
  }, items.map(f => /*#__PURE__*/React.createElement(FeatureCard, {
    key: f.title,
    icon: f.icon,
    title: f.title
  }, f.body))));
}
function Footer() {
  return /*#__PURE__*/React.createElement("footer", {
    style: {
      padding: "48px 0",
      borderTop: "1px solid var(--border)"
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      ...wrap,
      display: "flex",
      flexWrap: "wrap",
      gap: 24,
      justifyContent: "space-between",
      alignItems: "center"
    }
  }, /*#__PURE__*/React.createElement(Wordmark, {
    brand: "osf",
    size: 18
  }), /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      gap: 32,
      fontSize: "var(--text-sm)",
      fontWeight: 500,
      color: "var(--text-muted)"
    }
  }, /*#__PURE__*/React.createElement("a", {
    href: "#",
    style: {
      color: "inherit",
      textDecoration: "none",
      display: "inline-flex",
      alignItems: "center",
      gap: 6
    }
  }, /*#__PURE__*/React.createElement(IconGitHub, null), " GitHub"), /*#__PURE__*/React.createElement("a", {
    href: "#",
    style: {
      color: "inherit",
      textDecoration: "none"
    }
  }, "Docs"), /*#__PURE__*/React.createElement("a", {
    href: "#",
    style: {
      color: "inherit",
      textDecoration: "none"
    }
  }, "Platform")), /*#__PURE__*/React.createElement("div", {
    style: {
      fontSize: "var(--text-sm)",
      color: "var(--text-muted)",
      opacity: 0.6
    }
  }, "\xA9 2026 Open Source Finance.")));
}
Object.assign(window, {
  Header,
  Hero,
  Credibility,
  Features,
  Footer,
  wrap
});
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/website/sections.jsx", error: String((e && e.message) || e) }); }

__ds_ns.OspreyMark = __ds_scope.OspreyMark;

__ds_ns.Wordmark = __ds_scope.Wordmark;

__ds_ns.Badge = __ds_scope.Badge;

__ds_ns.Button = __ds_scope.Button;

__ds_ns.Eyebrow = __ds_scope.Eyebrow;

__ds_ns.Avatar = __ds_scope.Avatar;

__ds_ns.Terminal = __ds_scope.Terminal;

__ds_ns.Field = __ds_scope.Field;

__ds_ns.Input = __ds_scope.Input;

__ds_ns.Textarea = __ds_scope.Textarea;

__ds_ns.Card = __ds_scope.Card;

__ds_ns.FeatureCard = __ds_scope.FeatureCard;

})();
