import { useState } from "preact/hooks";
import { localAuthApi } from "../../../api/authApi";
import { MIN_LOCAL_PASSWORD_LENGTH } from "../../../config/auth";
import type { LoginMode } from "../../../models/auth";
import { returnUrlPolicy } from "./returnUrlPolicy";

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
  ////////////////
  // Local State
  ////////////////
  const [email, setEmail] = useState(mode === "legacy-setup" ? adminEmail : "");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const setup = mode === "claim" || mode === "legacy-setup";

  ////////////////
  // Global State
  ////////////////
  const params = new URLSearchParams(location.search);
  const oauthError = params.get("error");
  const errorEmail = params.get("email") ?? "";
  const returnTo = returnUrlPolicy.safeTarget(params.get("return_to") ?? "", location.origin);

  ////////////////
  // Handlers
  ////////////////
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
    if (setup && password.length < MIN_LOCAL_PASSWORD_LENGTH) {
      setError(`Use at least ${MIN_LOCAL_PASSWORD_LENGTH} characters.`);
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
