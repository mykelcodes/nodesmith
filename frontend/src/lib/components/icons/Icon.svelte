<script lang="ts">
	import { iconPaths, type IconName } from './icon-data';

	interface Props {
		name: IconName;
		class?: string;
		size?: number | string;
		strokeWidth?: number;
		title?: string;
	}

	let {
		name,
		class: className = 'size-5 shrink-0',
		size,
		strokeWidth = 1.8,
		title
	}: Props = $props();
</script>

<svg
	xmlns="http://www.w3.org/2000/svg"
	viewBox="0 0 24 24"
	width={size}
	height={size}
	fill="none"
	stroke="currentColor"
	stroke-width={strokeWidth}
	stroke-linecap="round"
	stroke-linejoin="round"
	class={className}
	role={title ? 'img' : undefined}
	aria-hidden={title ? undefined : true}
	focusable="false"
>
	{#if title}<title>{title}</title>{/if}
	{#each iconPaths[name] as path (path)}
		<path d={path}></path>
	{/each}
</svg>
