import { createApp } from "vue";
import { createPinia } from "pinia";
import naive from "naive-ui";
import App from "./App.vue";
import { router } from "./router";
import "@/assets/styles/main.css";

// The token lives in core's HttpOnly session cookie now; a token or password
// that earlier versions kept here is wiped.
for (const key of ["ruto_admin_token", "ruto_admin_credentials"]) {
  localStorage.removeItem(key);
}

const app = createApp(App);

app.use(createPinia());
app.use(router);
app.use(naive);

app.mount("#app");
