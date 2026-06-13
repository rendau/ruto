import type { DialogApi, LoadingBarApi, MessageApi, NotificationApi } from "naive-ui";

// Holds the Naive UI feedback APIs so code outside a setup() scope
// (e.g. the API layer) can still surface toasts. Inside components prefer
// the native useMessage()/useDialog() composables.

interface Feedback {
  message: MessageApi | null;
  dialog: DialogApi | null;
  notification: NotificationApi | null;
  loadingBar: LoadingBarApi | null;
}

const feedback: Feedback = {
  message: null,
  dialog: null,
  notification: null,
  loadingBar: null
};

export function registerFeedback(api: Partial<Feedback>): void {
  Object.assign(feedback, api);
}

export function notifySuccess(message: string): void {
  if (feedback.message) {
    feedback.message.success(message);
    return;
  }
  console.info(message);
}

export function notifyError(message: string): void {
  if (feedback.message) {
    feedback.message.error(message);
    return;
  }
  console.error(message);
}

export function notifyInfo(message: string): void {
  if (feedback.message) {
    feedback.message.info(message);
    return;
  }
  console.info(message);
}
