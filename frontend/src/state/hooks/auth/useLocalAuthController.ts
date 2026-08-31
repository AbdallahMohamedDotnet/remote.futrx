import { useState } from "preact/hooks";
import { localAuthApi, twoFactorApi } from "../../../api/authApi";
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

  // A password or Google login can come back asking for a second factor
  // instead of completing outright. The Google callback signals the same
  // thing via a `?twoFactorRequired=1` redirect, since it has no JSON
  // response to branch on.
  const [pendingTwoFactor, setPendingTwoFactor] = useState(
    mode === "login" && params.get("twoFactorRequired") === "1"
  );
  const [twoFactorCode, setTwoFactorCode] = useState("");
  const [twoFactorError, setTwoFactorError] = useState<string | null>(null);
  const [twoFactorSubmitting, setTwoFactorSubmitting] = useState(false);

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
      if (setup) {
        await localAuthApi.claim(normalizedEmail, password);
        await onSuccess();
        return;
      }
      const result = await localAuthApi.login(normalizedEmail, password);
      if (result.twoFactorRequired) {
        setPendingTwoFactor(true);
        return;
      }
      await onSuccess();
      if (returnTo) location.assign(returnTo);
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setSubmitting(false);
    }
  }

  async function submitTwoFactorCode(event: Event) {
    event.preventDefault();
    if (!twoFactorCode.trim()) {
      setTwoFactorError("Enter a code from your authenticator app or a recovery code.");
      return;
    }
    setTwoFactorSubmitting(true);
    setTwoFactorError(null);
    try {
      await twoFactorApi.verify(twoFactorCode.trim());
      setPendingTwoFactor(false);
      await onSuccess();
      if (returnTo) location.assign(returnTo);
    } catch (cause) {
      setTwoFactorError((cause as Error).message);
    } finally {
      setTwoFactorSubmitting(false);
    }
  }

  async function cancelTwoFactor() {
    setTwoFactorCode("");
    setTwoFactorError(null);
    setPendingTwoFactor(false);
    try {
      await twoFactorApi.cancel();
    } catch {
      // Best-effort: the pending cookie also just expires on its own.
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
    pendingTwoFactor,
    twoFactorCode,
    setTwoFactorCode,
    twoFactorError,
    twoFactorSubmitting,
    submitTwoFactorCode,
    cancelTwoFactor,
  };
}
