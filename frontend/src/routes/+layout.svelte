<script lang="ts">
	import './layout.css';
	import { onMount } from 'svelte';
	import favicon from '$lib/assets/favicon.svg';
	import { api } from '$lib/api';
	import AppShell from '$lib/components/AppShell.svelte';
	import { applyCachedTheme, applyTheme, stopThemeSync } from '$lib/theme/theme';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();

	applyCachedTheme();

	onMount(() => {
		void api.store
			.getSettings()
			.then((settings) => applyTheme(settings.theme, true))
			.catch(() => {
				// The cached/default theme remains usable if desktop settings cannot be loaded.
			});
		return stopThemeSync;
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>Nodesmith</title>
	<meta
		name="description"
		content="Scaffold JavaScript and TypeScript projects with transparent, previewable commands."
	/>
</svelte:head>

<AppShell>
	{@render children()}
</AppShell>
