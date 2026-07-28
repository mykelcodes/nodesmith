<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { FieldContext } from './types';

	interface Props {
		label: string;
		children: Snippet<[FieldContext]>;
		controlId?: string;
		help?: string;
		error?: string;
		required?: boolean;
		optional?: boolean;
		asGroup?: boolean;
		action?: Snippet;
		class?: string;
	}

	const uid = $props.id();

	let {
		label,
		children,
		controlId,
		help,
		error,
		required = false,
		optional = false,
		asGroup = false,
		action,
		class: className = ''
	}: Props = $props();

	const resolvedControlId = $derived(controlId ?? `${uid}-control`);
	const helpId = $derived(help ? `${uid}-help` : undefined);
	const errorId = $derived(error ? `${uid}-error` : undefined);
	const describedBy = $derived([helpId, errorId].filter(Boolean).join(' ') || undefined);
	const context = $derived({
		controlId: resolvedControlId,
		describedBy,
		invalid: Boolean(error)
	});
</script>

{#if asGroup}
	<fieldset class={`min-w-0 space-y-2.5 ${className}`} aria-describedby={describedBy}>
		<legend class="text-sm font-semibold text-ink">
			{label}
			{#if required}<span class="ml-0.5 text-danger" aria-hidden="true">*</span>{/if}
			{#if optional}<span class="ml-1.5 text-xs font-normal text-ink-faint">Optional</span>{/if}
		</legend>
		{#if action}<div class="flex justify-end">{@render action()}</div>{/if}
		{@render children(context)}
		{#if help}
			<p id={helpId} class="text-xs leading-5 text-ink-faint">{help}</p>
		{/if}
		{#if error}
			<p id={errorId} class="flex items-start gap-1.5 text-xs leading-5 text-danger" role="alert">
				<span aria-hidden="true">•</span>
				<span>{error}</span>
			</p>
		{/if}
	</fieldset>
{:else}
	<div class={`min-w-0 space-y-2.5 ${className}`}>
		<div class="flex min-w-0 items-baseline justify-between gap-3">
			<label class="text-sm font-semibold text-ink" for={resolvedControlId}>
				{label}
				{#if required}<span class="ml-0.5 text-danger" aria-hidden="true">*</span>{/if}
				{#if optional}<span class="ml-1.5 text-xs font-normal text-ink-faint">Optional</span>{/if}
			</label>
			{#if action}<div class="shrink-0">{@render action()}</div>{/if}
		</div>
		{@render children(context)}
		{#if help}
			<p id={helpId} class="text-xs leading-5 text-ink-faint">{help}</p>
		{/if}
		{#if error}
			<p id={errorId} class="flex items-start gap-1.5 text-xs leading-5 text-danger" role="alert">
				<span aria-hidden="true">•</span>
				<span>{error}</span>
			</p>
		{/if}
	</div>
{/if}
