<script lang="ts">
	import '../../../routes/layout.css';
	import type { RecipeSummary } from '$lib/api';
	import RecipeCard from './RecipeCard.svelte';

	function recipe(overrides: Partial<RecipeSummary>): RecipeSummary {
		return {
			id: 'r',
			name: 'Vite',
			category: 'tooling',
			description: 'A fast, modern frontend project powered by an official Vite starter template.',
			docsUrl: 'https://vite.dev',
			tags: ['frontend', 'vite'],
			icon: 'vite',
			verifiedAt: '2026-07-28',
			installPolicy: 'optional',
			minimumReleaseAge: null,
			available: true,
			unavailableReasons: [],
			defaultPackageManager: 'pnpm',
			...overrides
		};
	}

	// Deliberately uneven: the shortest and longest realistic cards, one gated
	// recipe with a reason line, and one with an extra footer note.
	const recipes: RecipeSummary[] = [
		recipe({ id: 'a', name: 'Vite', description: 'Short one.', tags: [] }),
		recipe({
			id: 'b',
			name: 'Next.js',
			description:
				'A long description that will certainly wrap across three whole lines because it keeps going and going without stopping.',
			tags: ['frontend', 'react', 'ssr', 'fullstack']
		}),
		recipe({
			id: 'c',
			name: 'NestJS',
			available: false,
			unavailableReasons: ['required tool node has no usable version'],
			defaultPackageManager: ''
		}),
		recipe({ id: 'd', name: 'Electron', installPolicy: 'required' })
	];
</script>

<div class="bg-canvas p-4">
	<a
		id="focus-probe"
		href="#probe"
		class="rounded-lg focus-visible:ring-3 focus-visible:ring-brand/25 focus-visible:outline-none"
	>
		Focus probe
	</a>
	<div data-testid="grid" class="mt-4 grid auto-rows-fr gap-4 md:grid-cols-2 xl:grid-cols-3">
		{#each recipes as item (item.id)}
			<div class="h-full">
				<RecipeCard recipe={item} onSelect={() => {}} />
			</div>
		{/each}
	</div>
</div>
