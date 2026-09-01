import { chatQuestionStorageService } from "../../../../services/chat/chatQuestionStorageService.ts";

export function readAnswered(chatId: string, toolUseId: string): string | null {
  return chatQuestionStorageService.readAnswered(chatId, toolUseId);
}

export function writeAnswered(chatId: string, toolUseId: string, summary: string): void {
  chatQuestionStorageService.writeAnswered(chatId, toolUseId, summary);
}

export function clearAnswered(chatId: string, toolUseId: string): void {
  chatQuestionStorageService.clearAnswered(chatId, toolUseId);
}
