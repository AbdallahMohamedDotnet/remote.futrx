import { requestJson } from "../apiRequest";
import { sendHttpRequest } from "../../transport/http";
import {
  DirtyWorkingTreeError,
  type GitHistoryCheckoutResponse,
  type GitHistoryCommitsResponse,
  type GitHistoryDiffResponse,
  type GitHistoryReposResponse,
} from "../../models/history";
import { API_ROUTES } from "../../config/routes";
import {
  API_RESPONSE_STATUS,
  DEFAULT_CHAT_HISTORY_COMMIT_LIMIT,
  DIRTY_WORKING_TREE_FALLBACK_MESSAGE,
} from "../../config/api";

export const chatHistoryApi = {
  fetchHistoryRepos: (id: string) =>
    requestJson<GitHistoryReposResponse>(
      "GET",
      API_ROUTES.chats.historyRepos(id)
    ),

  fetchHistoryCommits: (
    id: string,
    repo: string,
    limit = DEFAULT_CHAT_HISTORY_COMMIT_LIMIT
  ) => {
    const search = new URLSearchParams({ repo, limit: String(limit) });
    return requestJson<GitHistoryCommitsResponse>(
      "GET",
      API_ROUTES.chats.historyCommits(id, search.toString())
    );
  },

  fetchHistoryDiff: (id: string, repo: string, sha: string) => {
    const search = new URLSearchParams({ repo, sha });
    return requestJson<GitHistoryDiffResponse>(
      "GET",
      API_ROUTES.chats.historyDiff(id, search.toString())
    );
  },

  historyCheckout: async (
    id: string,
    repo: string,
    sha: string,
    checkpointMessage = ""
  ) => {
    const response = await sendHttpRequest(
      "POST",
      API_ROUTES.chats.historyCheckout(id),
      { repo, sha, checkpointMessage }
    );
    if (response.status === API_RESPONSE_STATUS.unauthorized) {
      location.reload();
      return new Promise<GitHistoryCheckoutResponse>(() => {});
    }
    if (!response.ok) {
      let body: { error?: string; dirty?: boolean; dirtyFiles?: string[] } = {};
      try {
        body = await response.json();
      } catch {}
      if (response.status === API_RESPONSE_STATUS.conflict && body.dirty) {
        throw new DirtyWorkingTreeError(
          body.error || DIRTY_WORKING_TREE_FALLBACK_MESSAGE,
          body.dirtyFiles || []
        );
      }
      throw new Error(body.error || String(response.status));
    }
    return response.json() as Promise<GitHistoryCheckoutResponse>;
  },
};
