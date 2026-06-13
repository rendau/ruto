import { useDialog } from "naive-ui";

interface ConfirmOptions {
  title?: string;
  content: string;
  positiveText?: string;
  negativeText?: string;
  onConfirm: () => void | Promise<void>;
}

export function useConfirm() {
  const dialog = useDialog();

  function confirmDelete(options: ConfirmOptions): void {
    dialog.error({
      title: options.title ?? "Delete",
      content: options.content,
      positiveText: options.positiveText ?? "Delete",
      negativeText: options.negativeText ?? "Cancel",
      onPositiveClick: options.onConfirm
    });
  }

  function confirmAction(options: ConfirmOptions): void {
    dialog.warning({
      title: options.title ?? "Confirm",
      content: options.content,
      positiveText: options.positiveText ?? "Confirm",
      negativeText: options.negativeText ?? "Cancel",
      onPositiveClick: options.onConfirm
    });
  }

  return { dialog, confirmDelete, confirmAction };
}
