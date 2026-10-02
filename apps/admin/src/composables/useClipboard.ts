import { useMessage } from "naive-ui";

export function useClipboard() {
  const message = useMessage();

  async function copy(value: string, successText = "Copied"): Promise<boolean> {
    if (!value) return false;
    try {
      await navigator.clipboard.writeText(value);
      message.success(successText);
      return true;
    } catch {
      message.error("Clipboard unavailable");
      return false;
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
