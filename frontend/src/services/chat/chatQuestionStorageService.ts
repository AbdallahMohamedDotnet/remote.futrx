import { STORAGE_KEYS } from "../../config/storageKeys.ts";
import { browserStorageService } from "../platform/browserStorageService.ts";

class ChatQuestionStorageService {
  readAnswered(chatId: string, toolUseId: string): string | null {
    return browserStorageService.readString(this.answeredKey(chatId, toolUseId));
  }

  writeAnswered(chatId: string, toolUseId: string, summary: string): void {
    browserStorageService.writeString(this.answeredKey(chatId, toolUseId), summary);
  }

  clearAnswered(chatId: string, toolUseId: string): void {
    browserStorageService.remove(this.answeredKey(chatId, toolUseId));
  }

  private answeredKey(chatId: string, toolUseId: string): string {
    return `${STORAGE_KEYS.answeredQuestionPrefix}${chatId}:${toolUseId}`;
  }
}

export const chatQuestionStorageService = new ChatQuestionStorageService();
