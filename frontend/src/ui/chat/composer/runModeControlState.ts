export interface RunModeOption {
  value: string;
  label: string;
}

export function isUnsupportedRunMode(
  mode: string,
  modeOptions: readonly RunModeOption[],
): boolean {
  return mode !== ""
    && mode !== "default"
    && !modeOptions.some((option) => option.value === mode);
}
