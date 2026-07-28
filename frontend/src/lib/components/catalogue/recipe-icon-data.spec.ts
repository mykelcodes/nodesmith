import { describe, expect, it } from 'vitest';
import {
	bundledRecipeIconKeys,
	fallbackRecipeIcon,
	isBundledRecipeIconKey,
	recipeIconRegistry
} from './recipe-icon-data';

const manifestIconKeys = [
	'astro',
	'electron',
	'expo',
	'express',
	'hono',
	'nestjs',
	'next',
	'react',
	'solid',
	'svelte',
	'sveltekit',
	'tauri',
	'vite',
	'wails'
] as const;

describe('recipe icon registry', () => {
	it('covers every bundled recipe manifest icon exactly once', () => {
		expect(bundledRecipeIconKeys.toSorted()).toEqual([...manifestIconKeys].toSorted());
		expect(Object.keys(recipeIconRegistry)).toHaveLength(14);
	});

	it('provides drawable paths for every icon and the safe fallback', () => {
		for (const definition of Object.values(recipeIconRegistry)) {
			expect(definition.paths.length).toBeGreaterThan(0);
			expect(definition.paths.every(Boolean)).toBe(true);
		}
		expect(fallbackRecipeIcon.paths.length).toBeGreaterThan(0);
	});

	it('narrows bundled keys without accepting user-provided icon names', () => {
		expect(isBundledRecipeIconKey('wails')).toBe(true);
		expect(isBundledRecipeIconKey('my-company-recipe')).toBe(false);
		expect(isBundledRecipeIconKey('__proto__')).toBe(false);
	});
});
