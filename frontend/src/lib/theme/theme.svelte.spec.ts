import { afterEach, describe, expect, it, vi } from 'vitest';
import { applyTheme, stopThemeSync } from './theme';

describe('live system theme', () => {
	const originalMatchMedia = window.matchMedia;

	afterEach(() => {
		stopThemeSync();
		Object.defineProperty(window, 'matchMedia', {
			configurable: true,
			value: originalMatchMedia
		});
		document.documentElement.dataset.theme = 'dark';
	});

	it('updates the document when the system preference changes', () => {
		let changeListener: ((event: MediaQueryListEvent) => void) | undefined;
		const mediaQuery = {
			matches: true,
			media: '(prefers-color-scheme: light)',
			onchange: null,
			addEventListener: vi.fn((_name: string, listener: (event: MediaQueryListEvent) => void) => {
				changeListener = listener;
			}),
			removeEventListener: vi.fn(),
			addListener: vi.fn(),
			removeListener: vi.fn(),
			dispatchEvent: vi.fn()
		} as unknown as MediaQueryList;
		Object.defineProperty(window, 'matchMedia', {
			configurable: true,
			value: vi.fn(() => mediaQuery)
		});

		applyTheme('system');
		expect(document.documentElement.dataset.theme).toBe('light');

		changeListener?.({ matches: false } as MediaQueryListEvent);
		expect(document.documentElement.dataset.theme).toBe('dark');
	});
});
