<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';

	interface Props extends Omit<HTMLInputAttributes, 'checked' | 'class' | 'type' | 'value'> {
		checked?: boolean;
		label?: string;
		description?: string;
		disabled?: boolean;
		ariaLabel?: string;
		class?: string;
	}

	let {
		checked = $bindable(false),
		label,
		description,
		disabled = false,
		ariaLabel,
		class: className = '',
		...rest
	}: Props = $props();
</script>

<label
	class={`group flex items-start justify-between gap-4 rounded-control ${label || description ? 'cursor-pointer' : 'w-fit'} ${disabled ? 'cursor-not-allowed opacity-45' : ''} ${className}`}
>
	{#if label || description}
		<span class="min-w-0">
			{#if label}<span class="block text-sm font-semibold text-ink">{label}</span>{/if}
			{#if description}
				<span class="mt-0.5 block text-xs leading-5 text-ink-faint">{description}</span>
			{/if}
		</span>
	{/if}
	<span class="relative mt-0.5 inline-flex shrink-0">
		<input
			{...rest}
			type="checkbox"
			role="switch"
			bind:checked
			{disabled}
			aria-label={ariaLabel}
			aria-checked={checked}
			class="peer sr-only"
		/>
		<span
			class="toggle-track h-5.5 w-10 rounded-full border border-line-strong bg-overlay shadow-inner peer-checked:border-brand peer-checked:bg-brand peer-focus-visible:ring-3 peer-focus-visible:ring-brand/25 peer-disabled:cursor-not-allowed"
			aria-hidden="true"
		></span>
		<span
			class="toggle-thumb pointer-events-none absolute top-1 left-1 size-3.5 rounded-full bg-ink-muted shadow-sm peer-checked:bg-white"
			aria-hidden="true"
		></span>
	</span>
</label>

<style>
	.toggle-track {
		transition:
			background-color 220ms var(--ns-ease-out),
			border-color 220ms var(--ns-ease-out),
			box-shadow 220ms var(--ns-ease-out);
	}

	.toggle-thumb {
		--toggle-offset: 0rem;
		transform: translateX(var(--toggle-offset));
		transition:
			transform 240ms var(--ns-ease-out),
			background-color 180ms ease-out,
			box-shadow 180ms ease-out;
	}

	input:checked ~ .toggle-thumb {
		--toggle-offset: 1.125rem;
	}

	input:active ~ .toggle-thumb {
		transform: translateX(var(--toggle-offset)) scale(0.86);
	}
</style>
