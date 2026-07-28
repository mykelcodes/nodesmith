<script lang="ts">
	import type { Snippet } from 'svelte';

	type Tone = 'neutral' | 'accent' | 'success' | 'warning' | 'danger' | 'info';
	type Size = 'sm' | 'md';

	interface Props {
		children: Snippet;
		tone?: Tone;
		size?: Size;
		dot?: boolean;
		icon?: Snippet;
		class?: string;
	}

	const tones: Record<Tone, string> = {
		neutral: 'border-line bg-overlay text-ink-muted',
		accent: 'border-brand/30 bg-brand-soft text-brand-strong',
		success: 'border-success/30 bg-success-soft text-success',
		warning: 'border-warning/30 bg-warning-soft text-warning',
		danger: 'border-danger/30 bg-danger-soft text-danger',
		info: 'border-info/30 bg-info-soft text-info'
	};

	const dots: Record<Tone, string> = {
		neutral: 'bg-ink-faint',
		accent: 'bg-brand-strong',
		success: 'bg-success',
		warning: 'bg-warning',
		danger: 'bg-danger',
		info: 'bg-info'
	};

	let {
		children,
		tone = 'neutral',
		size = 'md',
		dot = false,
		icon,
		class: className = ''
	}: Props = $props();
</script>

<span
	class={`inline-flex w-fit items-center border font-semibold whitespace-nowrap ${size === 'sm' ? 'h-5 gap-1 rounded-md px-1.5 text-[0.6875rem]' : 'h-6 gap-1.5 rounded-lg px-2 text-xs'} ${tones[tone]} ${className}`}
>
	{#if dot}<span class={`size-1.5 rounded-full ${dots[tone]}`} aria-hidden="true"></span>{/if}
	{#if icon}<span class="-ml-0.5 flex items-center" aria-hidden="true">{@render icon()}</span>{/if}
	{@render children()}
</span>
