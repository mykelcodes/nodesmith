<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLButtonAttributes } from 'svelte/elements';
	import Icon from '../icons/Icon.svelte';

	type Variant = 'primary' | 'secondary' | 'ghost' | 'danger';
	type Size = 'sm' | 'md' | 'lg' | 'icon';

	interface Props extends Omit<HTMLButtonAttributes, 'children' | 'class' | 'disabled' | 'type'> {
		children: Snippet;
		variant?: Variant;
		size?: Size;
		type?: 'button' | 'submit' | 'reset';
		disabled?: boolean;
		loading?: boolean;
		loadingLabel?: string;
		class?: string;
	}

	const variants: Record<Variant, string> = {
		primary:
			'border-brand bg-brand text-white shadow-[0_8px_24px_color-mix(in_srgb,var(--ns-brand)_22%,transparent)] hover:border-brand-strong hover:bg-brand-strong active:bg-brand',
		secondary:
			'border-line-strong bg-panel-raised text-ink shadow-sm hover:border-line-strong hover:bg-overlay active:bg-panel-raised',
		ghost: 'border-transparent bg-transparent text-ink-muted hover:bg-overlay hover:text-ink',
		danger: 'border-danger/45 bg-danger-soft text-danger hover:border-danger/70 hover:bg-danger/15'
	};

	const sizes: Record<Size, string> = {
		sm: 'h-8 gap-1.5 rounded-lg px-3 text-xs',
		md: 'h-10 gap-2 rounded-control px-4 text-sm',
		lg: 'h-11 gap-2.5 rounded-control px-5 text-sm',
		icon: 'size-9 rounded-control p-0'
	};

	let {
		children,
		variant = 'primary',
		size = 'md',
		type = 'button',
		disabled = false,
		loading = false,
		loadingLabel = 'Working',
		class: className = '',
		...rest
	}: Props = $props();
</script>

<button
	{...rest}
	{type}
	disabled={disabled || loading}
	aria-busy={loading || undefined}
	class={`inline-flex shrink-0 items-center justify-center border font-semibold whitespace-nowrap transition-[color,background-color,border-color,box-shadow,transform] duration-150 ease-out-smooth select-none focus-visible:ring-3 focus-visible:ring-brand/25 focus-visible:outline-none active:translate-y-px disabled:pointer-events-none disabled:opacity-45 ${variants[variant]} ${sizes[size]} ${className}`}
>
	{#if loading}
		<Icon name="refresh" class="size-4 animate-spin" />
		<span class="sr-only">{loadingLabel}</span>
	{/if}
	{@render children()}
</button>
