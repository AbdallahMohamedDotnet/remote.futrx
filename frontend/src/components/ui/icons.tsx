// Lightweight inline SVG icons — keeps bundle small (no icon-library dep).
import type { JSX } from "preact";

type P = JSX.SVGAttributes<SVGSVGElement>;

const base = { fill: "none", stroke: "currentColor", "stroke-width": 2, "stroke-linecap": "round" as const, "stroke-linejoin": "round" as const, viewBox: "0 0 24 24" };

export const Plus = (p: P) => (<svg {...base} {...p}><path d="M12 5v14M5 12h14"/></svg>);
export const X = (p: P) => (<svg {...base} {...p}><path d="M18 6 6 18M6 6l12 12"/></svg>);
export const Menu = (p: P) => (<svg {...base} {...p}><path d="M3 6h18M3 12h18M3 18h18"/></svg>);
export const Search = (p: P) => (<svg {...base} {...p}><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>);
export const Send = (p: P) => (<svg {...base} {...p}><path d="M22 2 11 13M22 2l-7 20-4-9-9-4 20-7Z"/></svg>);
export const Folder = (p: P) => (<svg {...base} {...p}><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>);
export const Upload = (p: P) => (<svg {...base} {...p}><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M17 8l-5-5-5 5M12 3v12"/></svg>);
export const Terminal = (p: P) => (<svg {...base} {...p}><path d="m4 17 6-6-6-6M12 19h8"/></svg>);
export const MessageSquare = (p: P) => (<svg {...base} {...p}><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>);
export const ArrowUp = (p: P) => (<svg {...base} {...p}><path d="M12 19V5M5 12l7-7 7 7"/></svg>);
export const Square = (p: P) => (<svg {...base} {...p}><rect x="6" y="6" width="12" height="12" rx="1"/></svg>);
export const ChevronDown = (p: P) => (<svg {...base} {...p}><path d="m6 9 6 6 6-6"/></svg>);
export const ChevronRight = (p: P) => (<svg {...base} {...p}><path d="m9 6 6 6-6 6"/></svg>);
export const ChevronLeft = (p: P) => (<svg {...base} {...p}><path d="m15 18-6-6 6-6"/></svg>);
export const File = (p: P) => (<svg {...base} {...p}><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/></svg>);
export const Edit = (p: P) => (<svg {...base} {...p}><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="m18.5 2.5 3 3L12 15l-4 1 1-4z"/></svg>);
export const TerminalIcon = (p: P) => (<svg {...base} {...p}><path d="m4 9 4 4-4 4M10 17h4"/></svg>);
export const AlertCircle = (p: P) => (<svg {...base} {...p}><circle cx="12" cy="12" r="10"/><path d="M12 8v4M12 16h.01"/></svg>);
export const Loader = (p: P) => (<svg {...base} {...p}><path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83"/></svg>);
export const Check = (p: P) => (<svg {...base} {...p}><path d="M20 6 9 17l-5-5"/></svg>);
export const LogOut = (p: P) => (<svg {...base} {...p}><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><path d="m16 17 5-5-5-5M21 12H9"/></svg>);
export const Clock = (p: P) => (<svg {...base} {...p}><circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/></svg>);
export const Activity = (p: P) => (<svg {...base} {...p}><path d="M22 12h-4l-3 8L9 4l-3 8H2"/></svg>);
export const ArrowDown = (p: P) => (<svg {...base} {...p}><path d="M12 5v14M19 12l-7 7-7-7"/></svg>);
export const RotateCcw = (p: P) => (<svg {...base} {...p}><path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5"/></svg>);
export const Settings = (p: P) => (<svg {...base} {...p}><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/></svg>);
export const Eye = (p: P) => (<svg {...base} {...p}><path d="M2 12s4-7 10-7 10 7 10 7-4 7-10 7-10-7-10-7Z"/><circle cx="12" cy="12" r="3"/></svg>);
export const EyeOff = (p: P) => (<svg {...base} {...p}><path d="M17.94 17.94A10.43 10.43 0 0 1 12 19c-6 0-10-7-10-7a18.5 18.5 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c6 0 10 7 10 7a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><path d="m1 1 22 22"/></svg>);
export const ExternalLink = (p: P) => (<svg {...base} {...p}><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><path d="M15 3h6v6M10 14 21 3"/></svg>);
export const Key = (p: P) => (<svg {...base} {...p}><path d="m21 2-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0 3 3L22 7l-3-3m-3.5 3.5L19 4"/></svg>);
export const Monitor = (p: P) => (<svg {...base} {...p}><rect x="3" y="4" width="18" height="12" rx="2"/><path d="M8 20h8M12 16v4"/></svg>);
export const Moon = (p: P) => (<svg {...base} {...p}><path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8Z"/></svg>);
export const Sun = (p: P) => (<svg {...base} {...p}><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41"/></svg>);
export const Code = (p: P) => (<svg {...base} {...p}><path d="m16 18 6-6-6-6M8 6l-6 6 6 6"/></svg>);
export const Crosshair = (p: P) => (<svg {...base} {...p}><circle cx="12" cy="12" r="10"/><path d="M22 12h-4M6 12H2M12 6V2M12 22v-4"/><circle cx="12" cy="12" r="2"/></svg>);
