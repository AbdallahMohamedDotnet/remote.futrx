import type { ChatEventPage, ChatTranscriptPage } from "../../models/chat";

export function transcriptPageToEventPage(page: ChatTranscriptPage): ChatEventPage {
  return {
    events: page.turns.flatMap((turn) => turn.events),
    nextBefore: page.nextBefore,
    lastSeq: page.lastSeq,
    hasMore: page.hasMore,
  };
}
