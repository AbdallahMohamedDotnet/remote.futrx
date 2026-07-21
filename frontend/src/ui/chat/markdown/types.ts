export type MarkdownBlock =
  | { type: "paragraph"; text: string }
  | { type: "heading"; level: 1 | 2 | 3 | 4 | 5 | 6; text: string }
  | { type: "code"; lang?: string; text: string }
  | { type: "blockquote"; children: MarkdownBlock[] }
  | { type: "list"; ordered: boolean; start?: number; items: ListItem[] }
  | { type: "table"; header: string[]; rows: string[][] }
  | { type: "hr" };

export interface ListItem {
  text: string;
  checked?: boolean;
}
