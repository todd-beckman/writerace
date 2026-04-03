import { describe, it, expect } from 'vitest';
import {
  fixQuotes,
  countWords,
  formatDate,
  escHtml,
  escAttr,
  sortParticipants,
  validateUsername,
} from './utils';
import type { ServerParticipant } from './types';

describe('fixQuotes', () => {
  it('converts opening double quote before a word', () => {
    expect(fixQuotes('"hello')).toBe('\u201Chello');
  });

  it('converts closing double quote after a word', () => {
    expect(fixQuotes('hello"')).toBe('hello\u201D');
  });

  it('converts both double quotes in a sentence', () => {
    expect(fixQuotes('"hello world"')).toBe('\u201Chello world\u201D');
  });

  it('converts contraction apostrophe', () => {
    expect(fixQuotes("don't")).toBe('don\u2019t');
  });

  it('converts opening single quote before a word', () => {
    expect(fixQuotes("'hello")).toBe('\u2018hello');
  });

  it('converts closing single quote after a word', () => {
    expect(fixQuotes("hello'")).toBe('hello\u2019');
  });

  it('converts mixed single and double quotes in a sentence', () => {
    const result = fixQuotes('"She said \'hi\'"');
    expect(result).toBe('\u201CShe said \u2018hi\u2019\u201D');
  });

  it('converts nested quotes', () => {
    const result = fixQuotes('"She said \'hello\'"');
    expect(result).toBe('\u201CShe said \u2018hello\u2019\u201D');
  });

  it('returns empty string unchanged', () => {
    expect(fixQuotes('')).toBe('');
  });

  it('returns string with no quotes unchanged', () => {
    expect(fixQuotes('hello world')).toBe('hello world');
  });
});

describe('countWords', () => {
  it('returns 0 for empty string', () => {
    expect(countWords('')).toBe(0);
  });

  it('returns 0 for whitespace-only string', () => {
    expect(countWords('   ')).toBe(0);
    expect(countWords('\t\n')).toBe(0);
  });

  it('returns 1 for a single word', () => {
    expect(countWords('hello')).toBe(1);
  });

  it('counts multiple words with single spaces', () => {
    expect(countWords('one two three')).toBe(3);
  });

  it('counts words with varied whitespace', () => {
    expect(countWords('one\ttwo\nthree  four')).toBe(4);
  });

  it('ignores leading and trailing whitespace', () => {
    expect(countWords('  hello world  ')).toBe(2);
  });
});

describe('formatDate', () => {
  it('returns a string containing the year for a known timestamp', () => {
    const timestamp = new Date('2024-03-15T12:00:00Z').getTime();
    const result = formatDate(timestamp);
    expect(result).toContain('2024');
  });

  it('returns a non-empty string for any valid timestamp', () => {
    expect(formatDate(0).length).toBeGreaterThan(0);
    expect(formatDate(Date.now()).length).toBeGreaterThan(0);
  });
});

describe('escHtml', () => {
  it('escapes ampersands', () => {
    expect(escHtml('a&b')).toBe('a&amp;b');
  });

  it('escapes less-than signs', () => {
    expect(escHtml('a<b')).toBe('a&lt;b');
  });

  it('escapes greater-than signs', () => {
    expect(escHtml('a>b')).toBe('a&gt;b');
  });

  it('leaves normal text unchanged', () => {
    expect(escHtml('hello world')).toBe('hello world');
  });

  it('handles a string with all three characters', () => {
    expect(escHtml('<a&b>')).toBe('&lt;a&amp;b&gt;');
  });

  it('handles empty string', () => {
    expect(escHtml('')).toBe('');
  });
});

describe('escAttr', () => {
  it('escapes ampersands', () => {
    expect(escAttr('a&b')).toBe('a&amp;b');
  });

  it('escapes double quotes', () => {
    expect(escAttr('say "hi"')).toBe('say &quot;hi&quot;');
  });

  it('escapes less-than signs', () => {
    expect(escAttr('a<b')).toBe('a&lt;b');
  });

  it('leaves normal text unchanged', () => {
    expect(escAttr('hello world')).toBe('hello world');
  });

  it('handles empty string', () => {
    expect(escAttr('')).toBe('');
  });
});

function makeParticipant(overrides: Partial<ServerParticipant>): ServerParticipant {
  return {
    id: 'p1',
    username: 'user',
    wordCount: 0,
    finishOrder: 0,
    joinOrder: 1,
    connected: true,
    completed: false,
    ...overrides,
  };
}

describe('sortParticipants', () => {
  it('places finishers before non-finishers', () => {
    const nonFinisher = makeParticipant({ id: 'a', joinOrder: 1, finishOrder: 0 });
    const finisher = makeParticipant({ id: 'b', joinOrder: 2, finishOrder: 1 });
    const result = sortParticipants([nonFinisher, finisher]);
    expect(result[0].id).toBe('b');
    expect(result[1].id).toBe('a');
  });

  it('sorts finishers by finishOrder ascending', () => {
    const first = makeParticipant({ id: 'a', finishOrder: 1, joinOrder: 2 });
    const second = makeParticipant({ id: 'b', finishOrder: 2, joinOrder: 1 });
    const result = sortParticipants([second, first]);
    expect(result[0].id).toBe('a');
    expect(result[1].id).toBe('b');
  });

  it('sorts non-finishers by joinOrder ascending', () => {
    const late = makeParticipant({ id: 'a', joinOrder: 3, finishOrder: 0 });
    const early = makeParticipant({ id: 'b', joinOrder: 1, finishOrder: 0 });
    const result = sortParticipants([late, early]);
    expect(result[0].id).toBe('b');
    expect(result[1].id).toBe('a');
  });

  it('sorts mixed list: finishers first by rank, then non-finishers by join order', () => {
    const p1 = makeParticipant({ id: 'p1', finishOrder: 2, joinOrder: 1 });
    const p2 = makeParticipant({ id: 'p2', finishOrder: 0, joinOrder: 2 });
    const p3 = makeParticipant({ id: 'p3', finishOrder: 1, joinOrder: 3 });
    const p4 = makeParticipant({ id: 'p4', finishOrder: 0, joinOrder: 1 });
    const result = sortParticipants([p1, p2, p3, p4]);
    expect(result.map((p) => p.id)).toEqual(['p3', 'p1', 'p4', 'p2']);
  });

  it('returns empty array for empty input', () => {
    expect(sortParticipants([])).toEqual([]);
  });

  it('does not mutate the input array', () => {
    const p1 = makeParticipant({ id: 'a', finishOrder: 0, joinOrder: 2 });
    const p2 = makeParticipant({ id: 'b', finishOrder: 0, joinOrder: 1 });
    const input = [p1, p2];
    sortParticipants(input);
    expect(input[0].id).toBe('a');
    expect(input[1].id).toBe('b');
  });
});

describe('validateUsername', () => {
  it('returns null for a valid username with letters', () => {
    expect(validateUsername('alice')).toBeNull();
  });

  it('returns null for a valid username with numbers', () => {
    expect(validateUsername('user123')).toBeNull();
  });

  it('returns null for a valid username with spaces, underscores, and hyphens', () => {
    expect(validateUsername('my user_name-1')).toBeNull();
  });

  it('returns an error for empty string', () => {
    expect(validateUsername('')).not.toBeNull();
  });

  it('returns an error for whitespace-only string', () => {
    expect(validateUsername('   ')).not.toBeNull();
  });

  it('returns an error for characters outside the allowed set', () => {
    expect(validateUsername('user@name')).not.toBeNull();
    expect(validateUsername('user!')).not.toBeNull();
    expect(validateUsername('user.name')).not.toBeNull();
  });

  it('trims leading and trailing whitespace before validation', () => {
    expect(validateUsername('  alice  ')).toBeNull();
  });
});
