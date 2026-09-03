import type { RegisteredSkill } from "../../../models/skill";

const SLASH_PATTERN = /^\/(\S*)$/;

class SlashCommandMenuPolicy {
  resolve(text: string, skills: RegisteredSkill[]) {
    const match = SLASH_PATTERN.exec(text);
    if (!match) return null;

    const query = match[1];
    const term = query.trim().toLowerCase();
    if (!term) return { query, items: skills };

    return {
      query,
      items: skills.filter((skill) =>
        `${skill.name} ${skill.command || ""} ${skill.description || ""} ${skill.source || ""}`
          .toLowerCase()
          .includes(term)
      ),
    };
  }

  clampHighlight(highlight: number, itemCount: number): number {
    return itemCount ? Math.min(highlight, itemCount - 1) : 0;
  }

  moveHighlight(highlight: number, step: -1 | 1, itemCount: number): number {
    return itemCount ? (highlight + step + itemCount) % itemCount : 0;
  }
}

export const slashCommandMenuPolicy = new SlashCommandMenuPolicy();
