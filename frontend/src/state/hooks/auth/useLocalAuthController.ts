import { useState } from "preact/hooks";
import { localAuthApi } from "../../../api/authApi";
import { returnUrlPolicy } from "../../auth/returnUrlPolicy";

export type LoginMode = "claim" | "login" | "legacy-setup";

interface LocalAuthControllerOptions {
  mode: LoginMode;
  adminEmail: string;
  onSuccess: () => Promise<void>;
}

export function useLocalAuthController({
  mode,
  adminEmail,
  onSuccess,
}: LocalAuthControllerOptions) {
  const params = new URLSearchParams(location.search);
  const oauthError = params.get("error");
  const errorEmail = params.get("email") ?? "";
  const returnTo = returnUrlPolicy.safeTarget(params.get("return_to") ?? "", location.origin);
  const [email, setEmail] = useState(mode === "legacy-setup" ? adminEmail : "");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const setup = mode === "claim" || mode === "legacy-setup";

  async function submit(event: Event) {
    event.preventDefault();
    const normalizedEmail = email.trim().toLowerCase();
    if (!normalizedEmail) {
      setError("Email is required.");
      return;
    }
    if (setup && password !== confirmation) {
      setError("Passwords do not match.");
      return;
    }
    if (setup && password.length < 12) {
      setError("Use at least 12 characters.");
      return;
    }

    setSubmitting(true);
    setError(null);
    try {
      if (setup) await localAuthApi.claim(normalizedEmail, password);
      else await localAuthApi.login(normalizedEmail, password);
      await onSuccess();
      if (mode === "login" && returnTo) location.assign(returnTo);
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setSubmitting(false);
    }
  }

  return {
    confirmation,
    email,
    error,
    errorEmail,
    googleURL: `/auth/google/login${returnTo ? `?return_to=${encodeURIComponent(returnTo)}` : ""}`,
    oauthError,
    password,
    setConfirmation,
    setEmail,
    setPassword,
    setup,
    submit,
    submitting,
  };
}
