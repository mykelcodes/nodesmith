<script lang="ts">
	import type { Snippet } from 'svelte';
	import { fly } from 'svelte/transition';
	import Icon from '../icons/Icon.svelte';
	import type { IconName } from '../icons/icon-data';

	type Tone = 'info' | 'success' | 'warning' | 'danger';

	interface Props {
		open?: boolean;
		tone?: Tone;
		title: string;
		message?: string;
		duration?: number;
		dismissible?: boolean;
		action?: Snippet;
		onDismiss?: () => void;
		class?: string;
	}

	const toneStyles: Record<Tone, string> = {
		info: 'border-info/30 bg-info-soft text-info',
		success: 'border-success/30 bg-success-soft text-success',
		warning: 'border-warning/30 bg-warning-soft text-warning',
		danger: 'border-danger/30 bg-danger-soft text-danger'
	};

	const toneIcons: Record<Tone, IconName> = {
		info: 'info',
		success: 'circleCheck',
		warning: 'triangleAlert',
		danger: 'circleX'
	};

	let {
		open = $bindable(true),
		tone = 'info',
		title,
		message,
		duration = 5000,
		dismissible = true,
		action,
		onDismiss,
		class: className = ''
	}: Props = $props();

	function dismiss() {
		if (!open) return;
		open = false;
		onDismiss?.();
	}

	$effect(() => {
		if (!open || duration <= 0) return;

		const timeout = window.setTimeout(dismiss, duration);
		return () => window.clearTimeout(timeout);
	});
</script>

{#if open}
	<div
		class={`pointer-events-auto flex w-full max-w-sm items-start gap-3 rounded-panel border border-line-strong bg-overlay/95 p-3.5 text-ink shadow-float backdrop-blur-xl ${className}`}
		role={tone === 'danger' ? 'alert' : 'status'}
		aria-live={tone === 'danger' ? 'assertive' : 'polite'}
		transition:fly={{ y: 8, duration: 160 }}
	>
		<span
			class={`mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-lg border ${toneStyles[tone]}`}
			aria-hidden="true"
		>
			<Icon name={toneIcons[tone]} class="size-4" />
		</span>
		<div class="min-w-0 flex-1">
			<p class="text-sm font-semibold text-ink">{title}</p>
			{#if message}<p class="mt-0.5 text-xs leading-5 text-ink-muted">{message}</p>{/if}
			{#if action}<div class="mt-2.5">{@render action()}</div>{/if}
		</div>
		{#if dismissible}
			<button
				type="button"
				class="-mt-1 -mr-1 flex size-7 shrink-0 items-center justify-center rounded-lg text-ink-faint transition-colors hover:bg-white/5 hover:text-ink focus-visible:ring-3 focus-visible:ring-brand/25 focus-visible:outline-none"
				aria-label="Dismiss notification"
				onclick={dismiss}
			>
				<Icon name="x" class="size-4" />
			</button>
		{/if}
	</div>
{/if}
