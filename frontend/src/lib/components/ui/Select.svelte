<script lang="ts">
	import type { HTMLSelectAttributes } from 'svelte/elements';
	import Icon from '../icons/Icon.svelte';
	import type { SelectOption } from './types';

	type SelectSize = 'sm' | 'md' | 'lg';

	interface Props extends Omit<
		HTMLSelectAttributes,
		'children' | 'class' | 'multiple' | 'size' | 'value'
	> {
		options: readonly SelectOption[];
		value?: string;
		placeholder?: string;
		size?: SelectSize;
		invalid?: boolean;
		class?: string;
	}

	const sizes: Record<SelectSize, string> = {
		sm: 'h-8 pl-3 pr-9 text-xs',
		md: 'h-10 pl-3.5 pr-10 text-sm',
		lg: 'h-11 pl-3.5 pr-10 text-sm'
	};

	let {
		options,
		value = $bindable(''),
		placeholder,
		size = 'md',
		invalid = false,
		class: className = '',
		...rest
	}: Props = $props();
</script>

<div class={`relative w-full ${className}`}>
	<select
		{...rest}
		bind:value
		aria-invalid={invalid || undefined}
		class={`w-full appearance-none rounded-control border bg-panel-raised font-medium text-ink shadow-sm transition-[border-color,box-shadow,background-color] duration-150 outline-none hover:border-ink-faint/70 focus:border-brand focus:ring-3 focus:ring-brand/20 disabled:cursor-not-allowed disabled:opacity-50 ${invalid ? 'border-danger/70 ring-3 ring-danger/10' : 'border-line-strong'} ${sizes[size]} ${value === '' ? 'text-ink-muted' : ''}`}
	>
		{#if placeholder}<option value="" disabled>{placeholder}</option>{/if}
		{#each options as option (option.value)}
			<option value={option.value} disabled={option.disabled}>{option.label}</option>
		{/each}
	</select>
	<Icon
		name="chevronDown"
		class="pointer-events-none absolute top-1/2 right-3 size-4 -translate-y-1/2 text-ink-faint"
	/>
</div>
