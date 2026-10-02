// No look-alikes (0/O, 1/l/I) and no symbols: the password is handed over in a
// messenger and retyped by a human, and messengers treat `*`/`_` as markup.
const ALPHABET = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789";

export function generatePassword(length = 16): string {
  // Bytes above the limit are rejected so `% ALPHABET.length` stays unbiased.
  const limit = 256 - (256 % ALPHABET.length);
  const bytes = new Uint8Array(length * 2);
  let result = "";
  while (result.length < length) {
    crypto.getRandomValues(bytes);
    for (const byte of bytes) {
      if (byte < limit && result.length < length) {
        result += ALPHABET[byte % ALPHABET.length];
      }
    }
  }
  return result;
}
