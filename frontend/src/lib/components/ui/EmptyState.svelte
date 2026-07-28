<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from '../icons/Icon.svelte';

	interface Props {
		title: string;
		description: string;
		icon?: Snippet;
		action?: Snippet;
		compact?: boolean;
		class?: string;
	}

	const uid = $props.id();

	let {
		title,
		description,
		icon,
		action,
		compact = false,
		class: className = ''
	}: Props = $props();
</script>

<section
	class={`flex min-w-0 flex-col items-center justify-center rounded-panel border border-dashed border-line-strong bg-panel/55 px-6 text-center ${compact ? 'min-h-52 py-8' : 'min-h-80 py-12'} ${className}`}
	aria-labelledby={`${uid}-title`}
>
	<div
		class="mb-4 flex size-11 items-center justify-center rounded-xl border border-line bg-panel-raised text-brand-strong shadow-sm"
		aria-hidden="true"
	>
		{#if icon}
			{@render icon()}
		{:else}
			<Icon name="sparkles" class="size-5" />
		{/if}
	</div>
	<h2 id={`${uid}-title`} class="text-base font-semibold tracking-[-0.01em] text-ink">
		{title}
	</h2>
	<p class="mt-1.5 max-w-md text-sm leading-6 text-ink-muted">{description}</p>
	{#if action}<div class="mt-5">{@render action()}</div>{/if}
</section>
