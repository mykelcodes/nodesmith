export interface RecipeIconDefinition {
	paths: readonly string[];
}

export const recipeIconRegistry = {
	astro: {
		paths: [
			'M7.4 17.5 10.2 6a1.9 1.9 0 0 1 3.6 0l2.8 11.5',
			'M5.5 17.5c3.9-1.5 9.1-1.5 13 0',
			'M9.2 20.5c1.7-1.1 3.9-1.1 5.6 0'
		]
	},
	electron: {
		paths: [
			'M12 12h.01',
			'M16.6 8.1c2.8 4.8 3.2 9.5.9 10.8-2.4 1.4-6.5-1.2-9.3-6S4 2.2 6.5.8c2-1.2 5.3.5 8 3.8',
			'M7.4 8.1C4.6 13 4.2 17.6 6.5 19c2.4 1.4 6.5-1.2 9.3-6s4.2-10.7 1.7-12.1c-2-1.2-5.3.5-8 3.8',
			'M12 17.3c5.6 0 9.7-2 9.7-4.7 0-2.8-4.1-4.7-9.7-4.7s-9.7 2-9.7 4.7c0 2.3 3 4 7.2 4.6'
		]
	},
	expo: {
		paths: ['M4.5 17.8 9.9 7.2c.9-1.7 3.3-1.7 4.2 0l5.4 10.6', 'm8.1 13.1 3.9-7.4 3.9 7.4']
	},
	express: {
		paths: ['M3.5 7.5h5l4.1 9h-5z', 'm13.2 7.5 3.2 4.5 4.1-4.5', 'm16.4 12 4.1 4.5']
	},
	hono: {
		paths: [
			'M13.3 2.5c.5 3.4-2.5 4.3-2.1 7.2.2 1.3 1.1 2.1 2 2.6-.3-2.3 1.2-3.8 2.5-5.1 2.1 1.8 3.8 4.7 3.8 7.6A8.1 8.1 0 0 1 4 14.2c0-4.1 2.2-7.1 5.5-9.5-.2 2 .1 3.2.8 4.1.2-3.2 1.1-4.8 3-6.3Z',
			'M9.1 16.4c0-1.7 1.1-3 2.9-4.4 1.8 1.4 2.9 2.7 2.9 4.4a2.9 2.9 0 0 1-5.8 0Z'
		]
	},
	nestjs: {
		paths: ['M12 2.8 20 7v8.6L12 21l-8-5.4V7z', 'M8 16V8l8 8V8', 'M8 8h8']
	},
	next: {
		paths: ['M4 18V6l11.5 12V6', 'm14 15 6 6', 'M19.5 5.5v8']
	},
	react: {
		paths: [
			'M12 12h.01',
			'M12 16.5c5.2 0 9.5-2 9.5-4.5S17.2 7.5 12 7.5s-9.5 2-9.5 4.5 4.3 4.5 9.5 4.5Z',
			'M8.1 14.3c2.6 4.5 6.5 7.2 8.7 5.9s1.8-6-0.8-10.5-6.5-7.2-8.7-5.9-1.8 6 .8 10.5Z',
			'M15.9 14.3c-2.6 4.5-6.5 7.2-8.7 5.9s-1.8-6 .8-10.5 6.5-7.2 8.7-5.9 1.8 6-.8 10.5Z'
		]
	},
	solid: {
		paths: ['m4 8 8-4 8 4-8 4z', 'm4 12 8 4 8-4', 'm4 16 8 4 8-4']
	},
	svelte: {
		paths: [
			'M16.8 5.2a4.4 4.4 0 0 0-6.1-1.5L6.9 6.1a4 4 0 0 0-.9 5.8 4.2 4.2 0 0 0 1.4 1.1 4 4 0 0 0-.2 4.8 4.4 4.4 0 0 0 6.1 1.5l3.8-2.4a4 4 0 0 0 .9-5.8A4.2 4.2 0 0 0 16.6 10a4 4 0 0 0 .2-4.8Z',
			'm8.4 14.1 5.7-3.6',
			'm9.9 8.7 5.7-3.6'
		]
	},
	sveltekit: {
		paths: [
			'M14.9 5.1a4 4 0 0 0-5.5-1.3L6.2 5.9a3.6 3.6 0 0 0 .4 6.3 3.6 3.6 0 0 0 .5 5 4 4 0 0 0 5.5 1.3l3.2-2.1a3.6 3.6 0 0 0-.4-6.3 3.6 3.6 0 0 0-.5-5Z',
			'm8.2 14 5-3.2',
			'M18.5 5.5v4',
			'M16.5 7.5h4'
		]
	},
	tauri: {
		paths: [
			'M12 3.2a8.8 8.8 0 1 0 8.8 8.8',
			'M12 20.8A8.8 8.8 0 1 0 3.2 12',
			'M8.7 12a3.3 3.3 0 1 0 6.6 0 3.3 3.3 0 1 0-6.6 0Z',
			'M7.4 7.4 9.7 9.7',
			'm14.3 14.3 2.3 2.3'
		]
	},
	vite: {
		paths: ['m4 4 7 17 9-17-8 3z', 'm12 7-1 7 5-8z']
	},
	wails: {
		paths: [
			'M3 8.5c2.2-2 4.4-2 6.6 0s4.4 2 6.6 0 4.4-2 4.8-1.6',
			'M3 12c2.2-2 4.4-2 6.6 0s4.4 2 6.6 0 4.4-2 4.8-1.6',
			'M3 15.5c2.2-2 4.4-2 6.6 0s4.4 2 6.6 0 4.4-2 4.8-1.6'
		]
	}
} as const satisfies Record<string, RecipeIconDefinition>;

export type BundledRecipeIconKey = keyof typeof recipeIconRegistry;

export const bundledRecipeIconKeys = Object.keys(recipeIconRegistry) as BundledRecipeIconKey[];

export const fallbackRecipeIcon: RecipeIconDefinition = {
	paths: ['m12 2.8 8 4.4v9.6l-8 4.4-8-4.4V7.2z', 'm4 7.2 8 4.5 8-4.5', 'M12 11.7v9.5']
};

export function isBundledRecipeIconKey(value: string): value is BundledRecipeIconKey {
	return Object.prototype.hasOwnProperty.call(recipeIconRegistry, value);
}
