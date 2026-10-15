/**
 * Utility functions and dictionaries for naming clarity rules.
 */

// Prohibited representation suffixes (Hungarian notation).
// Identifiers must convey domain purpose rather than encoding primitive data types.
// Examples:
//   - Disallowed: countInt, nameStr, validBool, activeBoolean
//   - Allowed:    count, name, isValid, active, totalBigInt
const prohibitedSuffixes: readonly string[] = ["Boolean", "Bool", "Str", "Int"] as const;

// Common acronyms that must follow standard JavaScript/TypeScript camelCase conventions
// (e.g. `userId` instead of `userID`, `serverUrl` instead of `serverURL`, like DOM `getElementById`).
const knownAcronyms: ReadonlySet<string> = new Set([
  "ID",
  "URL",
  "TTL",
  "JWT",
  "SID",
  "HTTP",
  "API",
  "UUID",
  "DB",
]);

export interface InitialismCheckResult {
  word: string;
  expected: string;
}

/**
 * Validates identifier against standard JavaScript camelCase acronym conventions.
 * Flags uppercase acronym components like `userID` -> expected `Id` (`userId`).
 */
export function checkJsInitialismCasing(name: string): InitialismCheckResult | null {
  // Ignore SCREAMING_SNAKE_CASE constants (e.g. MAX_ID, DEFAULT_URL)
  if (name.includes("_") && name === name.toUpperCase()) {
    return null;
  }

  const words = splitIdentifierWords(name);
  for (let wordIndex = 0; wordIndex < words.length; wordIndex++) {
    const word = words[wordIndex];
    if (knownAcronyms.has(word)) {
      const isLeading = wordIndex === 0;
      const expected = isLeading
        ? word.toLowerCase()
        : word[0].toUpperCase() + word.slice(1).toLowerCase();

      return { word, expected };
    }
  }

  return null;
}

// Non-common lazy abbreviations that undermine code clarity.
// Maps each disallowed abbreviation to its suggested clear replacement.
export const defaultDisallowedAbbreviations: Readonly<Record<string, string>> = {
  cfg: "Use a descriptive name. Did you mean 'config'?",
  mgr: "Use a descriptive name. Did you mean 'manager'?",
  val: "Use a descriptive name. Did you mean 'value'?",
  vals: "Use a descriptive name. Did you mean 'values'?",
  col: "Use a descriptive name. Did you mean 'column' or 'color'?",
  cols: "Use a descriptive name. Did you mean 'columns' or 'colors'?",
  obj: "Use a descriptive name. Did you mean 'object'?",
  arr: "Use a descriptive name. Did you mean 'array'?",
  elem: "Use a descriptive name. Did you mean 'element'?",
  idx: "Use a descriptive name. Did you mean 'index'?",
  cnt: "Use a descriptive name. Did you mean 'count'?",
  len: "Use a descriptive name. Did you mean 'length'?",
  str: "Use a descriptive string name",
  num: "Use a descriptive number name",
  req: "Use a descriptive name. Did you mean 'request'?",
  res: "Use a descriptive name. Did you mean 'response'?",
  resp: "Use a descriptive name. Did you mean 'response'?",
  wh: "Use a descriptive name. Did you mean 'webhook'?",
  b64: "Use a descriptive name. Did you mean 'base64'?",
  tz: "Use a descriptive name. Did you mean 'timeZone'?",
  usr: "Use a descriptive name. Did you mean 'user'?",
  btn: "Use a descriptive name. Did you mean 'button'?",
  mut: "Use a descriptive name. Did you mean 'mutation'?",
  sa: "Use a descriptive name. Did you mean 'serviceAccount'?",
  notif: "Use a descriptive name. Did you mean 'notification'?",
  msg: "Use a descriptive name. Did you mean 'message'?",
  sub: "Use a descriptive name. Did you mean 'subject' or 'subscription'?",
  subs: "Use a descriptive name. Did you mean 'subjects' or 'subscriptions'?",
};

/**
 * Splits camelCase, PascalCase, or snake_case identifiers into component words.
 * Correctly preserves consecutive acronyms (e.g. "userID" -> ["user", "ID"]).
 */
function splitIdentifierWords(name: string): string[] {
  const words: string[] = [];
  let current = "";
  const chars = Array.from(name);

  for (let charIndex = 0; charIndex < chars.length; charIndex++) {
    const char = chars[charIndex];
    const isDigit = char >= "0" && char <= "9";
    if (char === "_" || char === "$" || isDigit) {
      if (current.length > 0) {
        words.push(current);
        current = "";
      }
      continue;
    }

    const isUpper = char >= "A" && char <= "Z";
    if (isUpper) {
      if (current.length > 0) {
        const prevChar = chars[charIndex - 1];
        const prevIsUpper = prevChar >= "A" && prevChar <= "Z";
        const nextChar = charIndex + 1 < chars.length ? chars[charIndex + 1] : "";
        const nextIsLower = nextChar >= "a" && nextChar <= "z";
        if (!prevIsUpper || nextIsLower) {
          words.push(current);
          current = "";
        }
      }
    }
    current += char;
  }

  if (current.length > 0) {
    words.push(current);
  }
  return words;
}

/**
 * Checks whether an identifier ends with a prohibited Hungarian representation suffix.
 */
export function hasProhibitedSuffix(name: string): boolean {
  if (name === "" || name === "_") {
    return false;
  }

  // BigInt / BigInteger represents the JavaScript BigInt type, not Hungarian notation "Int".
  if (
    name.endsWith("BigInt") ||
    name.endsWith("bigInt") ||
    name.endsWith("BigInteger") ||
    name.endsWith("bigInteger")
  ) {
    return false;
  }

  for (const suffix of prohibitedSuffixes) {
    if (name.endsWith(suffix) && name.length > suffix.length) {
      return true;
    }
  }

  return false;
}

export interface DisallowedCheckResult {
  isDisallowed: boolean;
  word: string;
  suggestion: string;
}

/**
 * Checks whether an identifier contains disallowed lazy abbreviations.
 */
export function isDisallowedIdentifier(
  name: string,
  disallowedMap: Readonly<Record<string, string>> = defaultDisallowedAbbreviations,
): DisallowedCheckResult {
  if (!name || name === "_") {
    return { isDisallowed: false, word: "", suggestion: "" };
  }

  const words = splitIdentifierWords(name);
  for (let wordIndex = 0; wordIndex < words.length; wordIndex++) {
    const word = words[wordIndex];
    const wordLower = word.toLowerCase();
    const suggestion = disallowedMap[wordLower];

    if (suggestion) {
      return { isDisallowed: true, word, suggestion };
    }
  }

  return { isDisallowed: false, word: "", suggestion: "" };
}

/**
 * Checks if an error variable name conforms to canonical error conventions.
 * Canonical: "err", or ending with "Err", or starting with "Err" (e.g. ErrNotFound).
 * Prohibited: "error", "e", "myError".
 */
export function isErrorAllowed(name: string): boolean {
  if (name === "err" || name.endsWith("Err")) {
    return true;
  }
  return name.startsWith("Err") && name.length > 3 && name[3] >= "A" && name[3] <= "Z";
}
