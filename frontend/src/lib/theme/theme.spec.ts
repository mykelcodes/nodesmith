import { describe, expect, it } from 'vitest';
import { resolveTheme } from './theme';

describe('theme resolution', () => {
	it.each([
		['dark', false, 'dark'],
		['dark', true, 'dark'],
		['light', false, 'light'],
		['light', true, 'light'],
		['system', false, 'dark'],
		['system', true, 'light']
	] as const)('resolves %s with prefers-light=%s to %s', (theme, prefersLight, expected) => {
		expect(resolveTheme(theme, prefersLight)).toBe(expected);
	});
});
