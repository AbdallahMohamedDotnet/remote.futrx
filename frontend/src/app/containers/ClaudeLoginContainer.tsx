import { useEffect, useRef } from "preact/hooks";
import { ClaudeLoginScreen } from "../../ui/auth/ClaudeLoginScreen";
import { useClaudeLoginFlow } from "../../hooks/auth/useClaudeLoginFlow";

export function ClaudeLoginContainer({ onDone }: { onDone: () => void }) {
  const codeRef = useRef<HTMLTextAreaElement>(null);
  const login = useClaudeLoginFlow(onDone);

  useEffect(() => {
    if (login.phase === "awaiting-code") {
      setTimeout(() => codeRef.current?.focus(), 50);
    }
  }, [login.phase]);

  return (
    <ClaudeLoginScreen
      phase={login.phase}
      authUrl={login.authUrl}
      code={login.code}
      errorMessage={login.errorMessage}
      codeRef={codeRef}
      onCodeChange={login.setCode}
      onStartLogin={login.startLogin}
      onSubmitCode={login.submitCode}
      onCancel={login.cancel}
      onReset={login.reset}
    />
  );
}
