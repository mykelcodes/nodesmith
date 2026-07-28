<script lang="ts">
	import type { MultiSelectOption } from './types';

	interface Props {
		options: readonly MultiSelectOption[];
		value?: string[];
		name?: string;
		disabled?: boolean;
		invalid?: boolean;
		ariaLabel?: string;
		ariaLabelledby?: string;
		ariaDescribedby?: string;
		class?: string;
	}

	const uid = $props.id();

	let {
		options,
		value = $bindable([]),
		name,
		disabled = false,
		invalid = false,
		ariaLabel,
		ariaLabelledby,
		ariaDescribedby,
		class: className = ''
	}: Props = $props();
</script>

<div
	class={`grid gap-2 sm:grid-cols-2 ${className}`}
	role={ariaLabel || ariaLabelledby ? 'group' : undefined}
	aria-label={ariaLabel}
	aria-labelledby={ariaLabelledby}
	aria-describedby={ariaDescribedby}
	data-invalid={invalid || undefined}
>
	{#each options as option, index (option.value)}
		<label
			class={`group relative flex min-h-12 cursor-pointer items-start gap-3 rounded-control border bg-panel-raised px-3 py-2.5 text-left shadow-sm transition-[border-color,background-color,box-shadow] duration-150 hover:border-ink-faint/70 has-[:checked]:border-brand/65 has-[:checked]:bg-brand-soft has-[:focus-visible]:border-brand has-[:focus-visible]:ring-3 has-[:focus-visible]:ring-brand/20 ${invalid ? 'border-danger/70' : 'border-line'} ${disabled || option.disabled ? 'pointer-events-none opacity-45' : ''}`}
			for={`${uid}-${index}`}
		>
			<input
				id={`${uid}-${index}`}
				type="checkbox"
				class="peer sr-only"
				{name}
				value={option.value}
				bind:group={value}
				disabled={disabled || option.disabled}
			/>
			<span
				class="mt-0.5 flex size-4 shrink-0 items-center justify-center rounded-[0.3rem] border border-line-strong bg-canvas text-transparent transition-colors peer-checked:border-brand peer-checked:bg-brand peer-checked:text-white"
				aria-hidden="true"
			>
				<svg viewBox="0 0 16 16" class="size-3" fill="none" stroke="currentColor" stroke-width="2">
					<path d="m3.5 8.5 2.7 2.7 6.3-6.4"></path>
				</svg>
			</span>
			<span class="min-w-0">
				<span class="block text-sm font-semibold text-ink">{option.label}</span>
				{#if option.description}
					<span class="mt-0.5 block text-xs leading-4 text-ink-faint">{option.description}</span>
				{/if}
			</span>
		</label>
	{/each}
</div>
