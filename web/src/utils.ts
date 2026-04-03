import type { ServerParticipant } from './types';

/**
 * Replace straight quotation marks with their typographic (curly) equivalents.
 *
 * Rules applied in order:
 *  1. Double quote before a word character → left double quotation mark "
 *  2. Double quote after a word character or closing punctuation → right double quotation mark "
 *  3. Single quote / apostrophe between two word characters (contraction) → right single quotation mark '
 *  4. Single quote before a word character → left single quotation mark '
 *  5. Single quote after a word character or closing punctuation → right single quotation mark '
 */
export function fixQuotes(text: string): string {
  return text
    // 1. Opening double quote: " followed by a word character
    .replace(/"(?=\w)/g, '\u201C')
    // 2. Closing double quote: " preceded by a word character or closing punctuation
    .replace(/(?<=[\w.!?,;:])"/, '\u201D')
    // Remaining straight double quotes that are before whitespace/start → open,
    // everything else → close
    .replace(/"/g, (_, offset, str) => {
      const next = str[offset + 1];
      return next && /\S/.test(next) ? '\u201C' : '\u201D';
    })
    // 3. Contraction apostrophe: between two word characters
    .replace(/(?<=\w)'(?=\w)/g, '\u2019')
    // 4. Opening single quote: ' followed by a word character
    .replace(/'(?=\w)/g, '\u2018')
    // 5. Closing single quote: ' preceded by a word character or closing punctuation
    .replace(/(?<=[\w.!?,;:])'/, '\u2019')
    // Remaining straight single quotes → closing by default
    .replace(/'/g, '\u2019');
}

export function countWords(text: string): number {
  const trimmed = text.trim();
  return trimmed === '' ? 0 : trimmed.split(/\s+/).length;
}

export function formatDate(timestamp: number): string {
  return new Date(timestamp).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function escHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

export function escAttr(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;');
}

/** Sort: finishers ascending by finishOrder, then non-finishers by joinOrder. */
export function sortParticipants(participants: ServerParticipant[]): ServerParticipant[] {
  return [...participants].sort((a, b) => {
    const aFinished = a.finishOrder > 0;
    const bFinished = b.finishOrder > 0;
    if (aFinished && bFinished) return a.finishOrder - b.finishOrder;
    if (aFinished) return -1;
    if (bFinished) return 1;
    return a.joinOrder - b.joinOrder;
  });
}

const USERNAME_RE = /^[a-zA-Z0-9 _-]+$/;

/**
 * Validates a username value. Trims whitespace before checking.
 * Returns an error message string if invalid, or null if valid.
 */
export function validateUsername(value: string): string | null {
  const trimmed = value.trim();
  if (!trimmed) return 'Username is required.';
  if (!USERNAME_RE.test(trimmed)) return 'Only letters, numbers, spaces, underscores, and hyphens are allowed.';
  return null;
}
