import { browser } from '$app/environment';
import type { Theme } from '$lib/api';

const THEME_STORAGE_KEY = 'nodesmith:theme';
const SYSTEM_THEME_QUERY = '(prefers-color-scheme: light)';
const themes = new Set<Theme>(['dark', 'light', 'system']);

let systemThemeQuery: MediaQueryList | null = null;
let systemThemeListener: ((event: MediaQueryListEvent) => void) | null = null;

export function resolveTheme(theme: Theme, prefersLight: boolean): 'dark' | 'light' {
	return theme === 'system' ? (prefersLight ? 'light' : 'dark') : theme;
}

function detachSystemThemeListener() {
	if (!systemThemeQuery || !systemThemeListener) return;
	systemThemeQuery.removeEventListener('change', systemThemeListener);
	systemThemeQuery = null;
	systemThemeListener = null;
}

function renderTheme(theme: Theme) {
	if (!browser) return;
	detachSystemThemeListener();
	if (theme !== 'system') {
		document.documentElement.dataset.theme = theme;
		return;
	}

	systemThemeQuery = window.matchMedia(SYSTEM_THEME_QUERY);
	document.documentElement.dataset.theme = resolveTheme(theme, systemThemeQuery.matches);
	systemThemeListener = (event) => {
		document.documentElement.dataset.theme = resolveTheme('system', event.matches);
	};
	systemThemeQuery.addEventListener('change', systemThemeListener);
}

export function applyTheme(theme: Theme, persist = false) {
	renderTheme(theme);
	if (!browser || !persist) return;
	try {
		window.localStorage.setItem(THEME_STORAGE_KEY, theme);
	} catch {
		// Theme rendering must remain available when localStorage is unavailable.
	}
}

export function applyCachedTheme(): Theme | null {
	if (!browser) return null;
	try {
		const cached = window.localStorage.getItem(THEME_STORAGE_KEY);
		if (!cached || !themes.has(cached as Theme)) return null;
		const theme = cached as Theme;
		renderTheme(theme);
		return theme;
	} catch {
		return null;
	}
}

export function stopThemeSync() {
	detachSystemThemeListener();
}
