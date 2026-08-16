import { useEffect, useRef } from "preact/hooks";
import { toDataURL } from "qrcode";

// The one place the `qrcode` dependency is used, isolated behind a single
// wrapper component per the plan's Patterns section, so the dependency can
// be swapped without touching enrollment logic.
export function QrCode({ value, size = 200, class: className }: {
  value: string;
  size?: number;
  class?: string;
}) {
  const imgRef = useRef<HTMLImageElement>(null);

  useEffect(() => {
    let cancelled = false;
    toDataURL(value, { width: size, margin: 1 })
      .then((dataUrl) => {
        if (!cancelled && imgRef.current) imgRef.current.src = dataUrl;
      })
      .catch(() => {
        // Rendering failure just leaves the QR image blank; the enrollment
        // UI always shows the secret/otpauth URI as a manual-entry fallback.
      });
    return () => {
      cancelled = true;
    };
  }, [value, size]);

  return (
    <img
      ref={imgRef}
      width={size}
      height={size}
      alt="Two-factor authentication QR code"
      class={className}
    />
  );
}
