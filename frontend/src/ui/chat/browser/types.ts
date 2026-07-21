export interface BrowserElementCapture {
  url: string;
  title?: string;
  selector: string;
  tag: string;
  id?: string;
  classes?: string[];
  role?: string;
  ariaLabel?: string;
  text?: string;
  html?: string;
  rect?: { x: number; y: number; width: number; height: number };
  viewport?: { width: number; height: number };
  styles?: Record<string, string>;
  parents?: string[];
}
