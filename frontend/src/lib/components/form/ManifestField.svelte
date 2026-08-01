<script lang="ts">
	import type { AnswerValue, RecipeField } from '$lib/api';
	import { Field, Select, TextInput, Toggle } from '$lib/components/ui';

	interface Props {
		field: RecipeField;
		value: AnswerValue | undefined;
		error?: string;
		onChange: (value: AnswerValue) => void;
	}

	let { field, value, error, onChange }: Props = $props();

	function stringValue(): string {
		return typeof value === 'string' ? value : '';
	}

	function numberValue(): number {
		return typeof value === 'number' ? value : 0;
	}

	function booleanValue(): boolean {
		return typeof value === 'boolean' ? value : false;
	}

	function arrayValue(): string[] {
		return Array.isArray(value) ? value : [];
	}

	function toggleOption(option: string, checked: boolean) {
		const current = arrayValue();
		onChange(checked ? [...current, option] : current.filter((item) => item !== option));
	}
</script>

{#if field.type === 'boolean'}
	<Field label={field.label} help={field.help} {error} asGroup>
		{#snippet children(context)}
			<Toggle
				checked={booleanValue()}
				description={field.help || `Include ${field.label.toLowerCase()} in this project.`}
				ariaLabel={field.label}
				aria-describedby={context.describedBy}
				onchange={(event) => onChange(event.currentTarget.checked)}
			/>
		{/snippet}
	</Field>
{:else if field.type === 'select'}
	<Field label={field.label} help={field.help} {error}>
		{#snippet children(context)}
			<Select
				id={context.controlId}
				value={stringValue()}
				options={field.options}
				invalid={context.invalid}
				aria-describedby={context.describedBy}
				onchange={(event) => onChange(event.currentTarget.value)}
			/>
		{/snippet}
	</Field>
{:else if field.type === 'multiselect'}
	<Field label={field.label} help={field.help} {error} asGroup>
		{#snippet children(context)}
			<div class="grid gap-2 sm:grid-cols-2" aria-describedby={context.describedBy}>
				{#each field.options as option (option.value)}
					<label
						class="group relative flex min-h-12 cursor-pointer items-start gap-3 rounded-control border border-line bg-panel-raised px-3 py-2.5 text-left shadow-sm transition-[border-color,background-color,box-shadow] duration-150 hover:border-ink-faint/70 has-[:checked]:border-brand/65 has-[:checked]:bg-brand-soft has-[:focus-visible]:ring-3 has-[:focus-visible]:ring-brand/20"
					>
						<input
							type="checkbox"
							checked={arrayValue().includes(option.value)}
							class="peer sr-only"
							onchange={(event) => toggleOption(option.value, event.currentTarget.checked)}
						/>
						<span
							class="checkbox-box mt-0.5 flex size-4 shrink-0 items-center justify-center rounded-[0.3rem] border border-line-strong bg-canvas text-white peer-checked:border-brand peer-checked:bg-brand"
							aria-hidden="true"
						>
							<svg
								viewBox="0 0 16 16"
								class="checkbox-tick size-3"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"
							>
								<path d="m3.5 8.5 2.7 2.7 6.3-6.4"></path>
							</svg>
						</span>
						<span class="text-sm font-semibold text-ink">{option.label}</span>
					</label>
				{/each}
			</div>
		{/snippet}
	</Field>
{:else if field.type === 'number'}
	<!--
		Declared bounds become native input attributes so the browser's own
		stepper and validation agree with what the planner will accept.
	-->
	<Field label={field.label} help={field.help} {error}>
		{#snippet children(context)}
			<input
				id={context.controlId}
				type="number"
				value={numberValue()}
				min={field.min ?? undefined}
				max={field.max ?? undefined}
				aria-invalid={context.invalid || undefined}
				aria-describedby={context.describedBy}
				class="h-10 w-full rounded-control border border-line-strong bg-panel-raised px-3.5 text-sm font-medium text-ink shadow-sm transition-[border-color,box-shadow] outline-none hover:border-ink-faint/70 focus:border-brand focus:ring-3 focus:ring-brand/20"
				oninput={(event) => onChange(event.currentTarget.valueAsNumber)}
			/>
		{/snippet}
	</Field>
{:else}
	<Field label={field.label} help={field.help} {error} required>
		{#snippet children(context)}
			<TextInput
				id={context.controlId}
				value={stringValue()}
				invalid={context.invalid}
				aria-describedby={context.describedBy}
				placeholder={`Enter ${field.label.toLowerCase()}`}
				required
				aria-required="true"
				minlength={field.minLength ?? undefined}
				maxlength={field.maxLength ?? undefined}
				pattern={field.pattern || undefined}
				oninput={(event) => onChange(event.currentTarget.value)}
			/>
		{/snippet}
	</Field>
{/if}

<style>
	/* Mirrors MultiSelect: the tick draws on rather than appearing instantly. */
	.checkbox-box {
		transition:
			background-color 180ms var(--ns-ease-out),
			border-color 180ms var(--ns-ease-out),
			transform 180ms var(--ns-ease-out);
	}

	.checkbox-tick path {
		stroke-dasharray: 16;
		stroke-dashoffset: 16;
		transition: stroke-dashoffset 200ms var(--ns-ease-out) 40ms;
	}

	input:checked ~ .checkbox-box .checkbox-tick path {
		stroke-dashoffset: 0;
	}

	input:checked ~ .checkbox-box {
		transform: scale(1.06);
	}

	input:active ~ .checkbox-box {
		transform: scale(0.92);
	}

	@media (prefers-reduced-motion: reduce) {
		.checkbox-tick path {
			transition: none;
		}
	}
</style>
