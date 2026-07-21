import { useEffect, useRef, useState } from "preact/hooks";
import type { ClaudeLoginPhase } from "../../../models/auth";
import { claudeAuthApi } from "../../../api/agents/auth/claudeAuthApi";

export function useClaudeLoginFlow(onDone: () => void) {
  const [phase, setPhaseState] = useState<ClaudeLoginPhase>("idle");
  const [authUrl, setAuthUrl] = useState("");
  const [code, setCode] = useState("");
  const [errorMessage, setErrorMessage] = useState("");
  const phaseRef = useRef<ClaudeLoginPhase>("idle");

  function setPhase(next: ClaudeLoginPhase) {
    phaseRef.current = next;
    setPhaseState(next);
  }

  useEffect(() => {
    return () => {
      if (phaseRef.current === "starting" || phaseRef.current === "awaiting-code") {
        claudeAuthApi.cancelLogin().catch(() => {});
      }
    };
  }, []);

  async function startLogin() {
    setPhase("starting");
    setErrorMessage("");
    try {
      const response = await claudeAuthApi.startLogin();
      setAuthUrl(response.url);
      setPhase("awaiting-code");
    } catch (error) {
      setErrorMessage((error as Error).message);
      setPhase("error");
    }
  }

  async function submitCode() {
    const trimmed = code.trim();
    if (!trimmed) return;
    setPhase("submitting");
    setErrorMessage("");
    try {
      await claudeAuthApi.submitCode(trimmed);
      setPhase("done");
      setTimeout(onDone, 700);
    } catch (error) {
      setErrorMessage((error as Error).message);
      setPhase("error");
    }
  }

  async function cancel() {
    try {
      await claudeAuthApi.cancelLogin();
    } catch {}
    reset();
  }

  function reset() {
    setPhase("idle");
    setCode("");
    setAuthUrl("");
    setErrorMessage("");
  }

  return {
    phase,
    authUrl,
    code,
    setCode,
    errorMessage,
    startLogin,
    submitCode,
    cancel,
    reset,
  };
}
