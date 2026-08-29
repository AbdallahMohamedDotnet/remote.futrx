import type { ChatEvent, ChatEventPage, ChatStatus } from "../../../models/chat";
import type { ChatMessageBlock } from "../../../models/chatMessage";
import type { ChatUsageTotals } from "../../../models/chatUsage";
import { chatMessageBlockBuilder } from "./chatMessageBlockBuilder.ts";
import { chatUsageAccumulator, EMPTY_USAGE_TOTALS } from "./chatUsageAccumulator.ts";

export interface ChatRenderState {
  events: ChatEvent[];
  blocks: ChatMessageBlock[];
  usageTotals: ChatUsageTotals;
  eventCount: number;
  hasOlder: boolean;
  nextBefore: number;
}

class ChatEventStateProjector {
  empty(): ChatRenderState {
    return {
      events: [],
      blocks: [],
      usageTotals: EMPTY_USAGE_TOTALS,
      eventCount: 0,
      hasOlder: false,
      nextBefore: 0,
    };
  }

  fromEvents(
    events: ChatEvent[],
    page: Pick<ChatEventPage, "hasMore" | "nextBefore">
  ): ChatRenderState {
    return {
      events,
      blocks: chatMessageBlockBuilder.fromEvents(events),
      usageTotals: chatUsageAccumulator.totalFor(events),
      eventCount: events.length,
      hasOlder: page.hasMore,
      nextBefore: page.nextBefore ?? 0,
    };
  }

  append(state: ChatRenderState, events: ChatEvent[]): ChatRenderState {
    if (events.length === 0) return state;
    const merged = this.mergeEvents(state.events, events);
    return this.fromEvents(merged, {
      hasMore: state.hasOlder,
      nextBefore: state.nextBefore,
    });
  }

  prepend(state: ChatRenderState, page: ChatEventPage): ChatRenderState {
    return this.fromEvents(this.mergeEvents(page.events, state.events), page);
  }

  latestSequence(events: ChatEvent[]): number {
    return events.reduce((max, event) => Math.max(max, event.seq || 0), 0);
  }

  statusAfter(event: ChatEvent, current: ChatStatus): ChatStatus {
    if (event.type === "complete" || event.type === "error") {
      // The backend clears the run lock in a later sync event. Keep streaming
      // until sync running=false so queued prompts are not sent into a locked run.
      return current === "streaming" ? "streaming" : "ready";
    }
    return "streaming";
  }

  private mergeEvents(first: ChatEvent[], second: ChatEvent[]): ChatEvent[] {
    const merged = [...first];
    const seenSequences = new Set<number>();
    for (const event of merged) {
      if (event.seq) seenSequences.add(event.seq);
    }
    for (const event of second) {
      if (event.seq && seenSequences.has(event.seq)) continue;
      merged.push(event);
      if (event.seq) seenSequences.add(event.seq);
    }
    return merged.sort((left, right) => this.eventOrder(left) - this.eventOrder(right));
  }

  private eventOrder(event: ChatEvent): number {
    return event.seq || event.t;
  }
}

export const chatEventStateProjector = new ChatEventStateProjector();
