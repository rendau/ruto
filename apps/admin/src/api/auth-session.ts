const storageKeys = {
  token: "ruto_admin_token",
  // Earlier versions kept the password here to log in again on their own;
  // it is wiped from browsers that still have it.
  legacyCredentials: "ruto_admin_credentials"
};

localStorage.removeItem(storageKeys.legacyCredentials);

let token = localStorage.getItem(storageKeys.token) || "";

export function getToken(): string {
  return token;
}

export function setToken(value: string): void {
  token = value.trim();
  if (token) {
    localStorage.setItem(storageKeys.token, token);
  } else {
    localStorage.removeItem(storageKeys.token);
  }
}

export function clearSession(): void {
  setToken("");
}
