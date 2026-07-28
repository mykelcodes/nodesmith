<script lang="ts">
	import {
		fallbackRecipeIcon,
		isBundledRecipeIconKey,
		recipeIconRegistry
	} from './recipe-icon-data';

	interface Props {
		icon: string;
		label?: string;
		class?: string;
	}

	let { icon, label, class: className = 'size-6' }: Props = $props();

	const knownIcon = $derived(isBundledRecipeIconKey(icon));
	const definition = $derived(
		isBundledRecipeIconKey(icon) ? recipeIconRegistry[icon] : fallbackRecipeIcon
	);
</script>

<svg
	viewBox="0 0 24 24"
	fill="none"
	stroke="currentColor"
	stroke-width="1.7"
	stroke-linecap="round"
	stroke-linejoin="round"
	class={className}
	role={label ? 'img' : undefined}
	aria-hidden={label ? undefined : 'true'}
	data-recipe-icon={knownIcon ? icon : 'fallback'}
>
	{#if label}<title>{label}</title>{/if}
	{#each definition.paths as path (path)}
		<path d={path}></path>
	{/each}
</svg>
