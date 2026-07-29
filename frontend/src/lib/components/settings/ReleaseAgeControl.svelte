<script lang="ts">
	import { untrack } from 'svelte';
	import { Field, Select, TextInput } from '$lib/components/ui';
	import { releaseAgePresets, validateReleaseAgeInput } from '$lib/settings/releaseAge';

	interface Props {
		label: string;
		/** Label for the "no value here" choice, e.g. "Not set" or "Inherit". */
		inheritLabel: string;
		/** Explains which value applies while the inherit choice is selected. */
		inheritHint?: string;
		help?: string;
		/** Minutes, or null when this layer sets nothing. */
		value: number | null;
		/**
		 * Reports the chosen value together with a validation message. The
		 * minutes are only meaningful while the message is empty.
		 */
		onchange: (minutes: number | null, error: string) => void;
	}

	const INHERIT = '__inherit__';
	const CUSTOM = '__custom__';

	let { label, inheritLabel, inheritHint, help, value, onchange }: Props = $props();

	const presetMinutes = new Set<number>(releaseAgePresets.map((preset) => preset.value));

	// Seeded from the incoming value, then owned by the control so a half-typed
	// custom value does not collapse back to a preset on every keystroke.
	let mode = $state(
		untrack(() => (value === null ? INHERIT : presetMinutes.has(value) ? String(value) : CUSTOM))
	);
	let customText = $state(
		untrack(() => (value !== null && !presetMinutes.has(value) ? String(value) : ''))
	);

	const customError = $derived(mode === CUSTOM ? validateReleaseAgeInput(customText) : '');
	const options = $derived([
		{ value: INHERIT, label: inheritLabel },
		...releaseAgePresets.map((preset) => ({ value: String(preset.value), label: preset.label })),
		{ value: CUSTOM, label: 'Custom…' }
	]);

	function emit() {
		if (mode === INHERIT) {
			onchange(null, '');
			return;
		}
		if (mode === CUSTOM) {
			const error = validateReleaseAgeInput(customText);
			onchange(error ? null : Number(customText.trim()), error);
			return;
		}
		onchange(Number(mode), '');
	}

	function setMode(next: string) {
		mode = next;
		emit();
	}

	function setCustomText(next: string) {
		customText = next;
		emit();
	}
</script>

<Field {label} help={mode === INHERIT ? (inheritHint ?? help) : help} error={customError}>
	{#snippet children(context)}
		<div class="flex flex-col gap-2 sm:flex-row">
			<Select
				id={context.controlId}
				value={mode}
				{options}
				aria-describedby={context.describedBy}
				class="sm:max-w-56"
				onchange={(event) => setMode(event.currentTarget.value)}
			/>
			{#if mode === CUSTOM}
				<TextInput
					value={customText}
					placeholder="1440"
					inputmode="numeric"
					autocomplete="off"
					spellcheck="false"
					inputClass="font-mono"
					invalid={Boolean(customError)}
					aria-label={`${label} in minutes`}
					aria-describedby={context.describedBy}
					class="sm:max-w-40"
					oninput={(event) => setCustomText(event.currentTarget.value)}
				/>
				<span class="self-center text-xs text-ink-faint">minutes</span>
			{/if}
		</div>
	{/snippet}
</Field>
