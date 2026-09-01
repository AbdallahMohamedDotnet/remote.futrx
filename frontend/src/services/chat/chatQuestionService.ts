import type {
  Question,
  QuestionAnswerSubmission,
} from "../../models/chat.ts";

class ChatQuestionService {
  private readonly responseReceived = "Response received";
  private readonly secretResponseReceived = "Secret response received";
  private readonly safeResolutionMessages = new Set([
    "No response before the agent continued",
    "Agent interaction cancelled",
  ]);

  hasValidIds(questions: Question[]): boolean {
    const ids = questions.map((question) => question.id?.trim() ?? "");
    return ids.every(Boolean) && new Set(ids).size === ids.length;
  }

  summarizeAnswers(
    questions: Question[],
    answersForQuestion: (index: number) => string[],
  ): QuestionAnswerSubmission {
    const parts: string[] = [];
    const preview: string[] = [];
    const answers: Record<string, string[]> = {};

    for (let index = 0; index < questions.length; index++) {
      const question = questions[index];
      const chosen = answersForQuestion(index);
      parts.push(`Q: ${question.question}\nA: ${chosen.join("; ")}`);
      preview.push(question.isSecret
        ? `${question.header ?? "Answer"}: ${this.secretResponseReceived}`
        : `${question.header ?? "Answer"}: ${chosen.join(", ")}`);
      const questionId = question.id?.trim();
      if (questionId) answers[questionId] = chosen;
    }

    return {
      text: parts.join("\n\n"),
      preview: preview.join(" · "),
      answers,
      sensitive: questions.some((question) => question.isSecret),
    };
  }

  resolvedPreview(questions: Question[], output?: string): string {
    if (questions.some((question) => question.isSecret)) {
      return this.secretResponseReceived;
    }
    if (!output) return this.responseReceived;

    try {
      const payload = JSON.parse(output) as Record<string, unknown>;
      const rawAnswers = payload.answers ?? payload.Answers;
      if (!rawAnswers || typeof rawAnswers !== "object" || Array.isArray(rawAnswers)) {
        const decision = payload.decision ?? payload.Decision;
        return typeof decision === "string" && decision.trim()
          ? `Decision: ${decision.trim()}`
          : "No answers provided";
      }

      const answers = Object.entries(rawAnswers).reduce<Record<string, string[]>>(
        (valid, [id, value]) => {
          if (Array.isArray(value) && value.every((answer) => typeof answer === "string")) {
            valid[id] = value;
          }
          return valid;
        },
        {},
      );
      const used = new Set<string>();
      const preview: string[] = [];
      for (const question of questions) {
        const id = question.id?.trim();
        if (!id || !(id in answers)) continue;
        used.add(id);
        preview.push(
          `${question.header ?? question.question}: ${answers[id].join(", ") || "No answer"}`,
        );
      }
      for (const [id, values] of Object.entries(answers)) {
        if (!used.has(id)) preview.push(`${id}: ${values.join(", ") || "No answer"}`);
      }
      return preview.length > 0 ? preview.join(" · ") : "No answers provided";
    } catch {
      const message = output.trim();
      if (this.safeResolutionMessages.has(message)) return message;
      return this.responseReceived;
    }
  }
}

export const chatQuestionService = new ChatQuestionService();
