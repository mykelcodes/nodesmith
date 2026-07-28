<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLInputAttributes } from 'svelte/elements';

	type InputType = 'text' | 'email' | 'password' | 'search' | 'tel' | 'url';
	type InputSize = 'sm' | 'md' | 'lg';

	interface Props extends Omit<HTMLInputAttributes, 'class' | 'size' | 'type' | 'value'> {
		value?: string;
		type?: InputType;
		size?: InputSize;
		invalid?: boolean;
		leading?: Snippet;
		trailing?: Snippet;
		class?: string;
		inputClass?: string;
	}

	const sizes: Record<InputSize, string> = {
		sm: 'h-8 text-xs',
		md: 'h-10 text-sm',
		lg: 'h-11 text-sm'
	};

	let {
		value = $bindable(''),
		type = 'text',
		size = 'md',
		invalid = false,
		leading,
		trailing,
		class: className = '',
		inputClass = '',
		...rest
	}: Props = $props();
</script>

<div
	class={`flex w-full items-center rounded-control border bg-panel-raised shadow-sm transition-[border-color,box-shadow,background-color] duration-150 focus-within:border-brand focus-within:ring-3 focus-within:ring-brand/20 ${invalid ? 'border-danger/70 ring-3 ring-danger/10' : 'border-line-strong hover:border-ink-faint/70'} ${sizes[size]} ${className}`}
>
	{#if leading}
		<span class="ml-3 flex shrink-0 items-center text-ink-faint" aria-hidden="true">
			{@render leading()}
		</span>
	{/if}
	<input
		{...rest}
		{type}
		bind:value
		aria-invalid={invalid || undefined}
		class={`h-full min-w-0 flex-1 border-0 bg-transparent px-3 text-ink outline-none placeholder:text-ink-faint disabled:cursor-not-allowed disabled:opacity-50 ${leading ? 'pl-2' : ''} ${trailing ? 'pr-2' : ''} ${inputClass}`}
	/>
	{#if trailing}
		<span class="mr-3 flex shrink-0 items-center text-ink-faint">
			{@render trailing()}
		</span>
	{/if}
</div>
