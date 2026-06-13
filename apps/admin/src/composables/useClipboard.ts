import { useMessage } from "naive-ui";

export function useClipboard() {
  const message = useMessage();

  async function copy(value: string, successText = "Copied"): Promise<void> {
    if (!value) return;
    try {
      await navigator.clipboard.writeText(value);
      message.success(successText);
    } catch {
      message.error("Clipboard unavailable");
    }
  }

  async function readSilently(): Promise<string> {
    try {
      return await navigator.clipboard.readText();
    } catch {
      return "";
    }
  }

  return { copy, readSilently };
}
