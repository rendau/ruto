import { createApp } from "vue";
import { createPinia } from "pinia";
import {
  NAlert,
  NButton,
  NCard,
  NCheckbox,
  NConfigProvider,
  NDialogProvider,
  NDropdown,
  NIcon,
  NInput,
  NInputNumber,
  NMessageProvider,
  NModal,
  NSelect,
  NSpace,
  NSwitch,
  NTag
} from "naive-ui";
import { NTabPane, NTabs } from "naive-ui/es/tabs";
import App from "./App.vue";
import { router } from "./router";
import "./style.css";

const app = createApp(App);
const pinia = createPinia();

app.use(pinia);
app.use(router);
app.component("NAlert", NAlert);
app.component("NButton", NButton);
app.component("NCard", NCard);
app.component("NCheckbox", NCheckbox);
app.component("NConfigProvider", NConfigProvider);
app.component("NDialogProvider", NDialogProvider);
app.component("NDropdown", NDropdown);
app.component("NIcon", NIcon);
app.component("NInput", NInput);
app.component("NInputNumber", NInputNumber);
app.component("NMessageProvider", NMessageProvider);
app.component("NModal", NModal);
app.component("NSelect", NSelect);
app.component("NSpace", NSpace);
app.component("NSwitch", NSwitch);
app.component("NTabPane", NTabPane);
app.component("NTabs", NTabs);
app.component("NTag", NTag);
app.mount("#app");
