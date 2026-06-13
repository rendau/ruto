import { createOptionsLookup } from "./createOptionsLookup";
import { getApp, listApps } from "@/api/app";
import type { AppMain } from "@/api/types";

export const useAppOptions = createOptionsLookup<AppMain>({
  list: () => listApps().then((rep) => rep.results ?? []),
  get: getApp,
  idOf: (app) => app.id,
  labelOf: (app) => app.name?.trim() || app.id
});
