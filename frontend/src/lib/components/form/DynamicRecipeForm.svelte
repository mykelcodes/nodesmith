<script lang="ts">
	import { resolve } from '$app/paths';
	import type { AnswerValue, Recipe, ScaffoldRequest } from '$lib/api';
	import Icon from '$lib/components/icons/Icon.svelte';
	import { Badge, Button, Field, Select, TextInput, Toggle } from '$lib/components/ui';
	import { visibleFields, type FieldErrors } from '$lib/form';
	import ManifestField from './ManifestField.svelte';

	interface Props {
		recipe: Recipe;
		request: ScaffoldRequest;
		errors?: FieldErrors;
		packageManagers?: readonly string[];
		pickingDirectory?: boolean;
		onPickDirectory: () => void;
		onRequestChange?: (request: ScaffoldRequest) => void;
	}

	let {
		recipe,
		request = $bindable(),
		errors = {},
		packageManagers = [],
		pickingDirectory = false,
		onPickDirectory,
		onRequestChange
	}: Props = $props();

	const shownFields = $derived(visibleFields(recipe.fields, request.answers));
	const packageManagerOptions = $derived(
		packageManagers.map((manager) => ({
			value: manager,
			label: manager
		}))
	);
	const installRequired = $derived(recipe.installPolicy === 'required');

	function updateBuiltIn<Key extends keyof ScaffoldRequest>(key: Key, value: ScaffoldRequest[Key]) {
		request = { ...request, [key]: value };
		onRequestChange?.(request);
	}

	function updateAnswer(fieldId: string, value: AnswerValue) {
		request = {
			...request,
			answers: { ...request.answers, [fieldId]: value }
		};
		onRequestChange?.(request);
	}
</script>

<div class="grid gap-6 xl:grid-cols-[minmax(0,1.35fr)_minmax(19rem,0.65fr)]">
	<section class="rounded-panel border border-line bg-panel/80 p-5 shadow-sm sm:p-6">
		<div class="flex items-start justify-between gap-4 border-b border-line pb-5">
			<div>
				<p class="text-xs font-bold tracking-[0.12em] text-brand-strong uppercase">Project</p>
				<h2 class="mt-1 text-lg font-bold tracking-[-0.025em] text-ink">Name and destination</h2>
				<p class="mt-1 text-sm leading-6 text-ink-muted">
					Choose where Nodesmith will create the project directory.
				</p>
			</div>
			<Badge tone={recipe.available ? 'success' : 'warning'} dot>
				{recipe.available ? 'Toolchain ready' : 'Setup needed'}
			</Badge>
		</div>

		<div class="mt-6 grid gap-6">
			<Field
				label="Project name"
				help="A portable folder name. Spaces and Unicode are supported."
				error={errors.projectName}
				required
			>
				{#snippet children(context)}
					<TextInput
						id={context.controlId}
						value={request.projectName}
						invalid={context.invalid}
						aria-describedby={context.describedBy}
						placeholder="my-new-project"
						autocomplete="off"
						spellcheck="false"
						required
						aria-required="true"
						oninput={(event) => updateBuiltIn('projectName', event.currentTarget.value)}
					/>
				{/snippet}
			</Field>

			<Field
				label="Parent directory"
				help="The project folder will be created inside this directory."
				error={errors.parentDir}
				required
			>
				{#snippet children(context)}
					<div class="flex min-w-0 gap-2">
						<TextInput
							id={context.controlId}
							value={request.parentDir}
							invalid={context.invalid}
							aria-describedby={context.describedBy}
							placeholder="/path/to/projects"
							inputClass="font-mono"
							required
							aria-required="true"
							oninput={(event) => updateBuiltIn('parentDir', event.currentTarget.value)}
						/>
						<Button
							variant="secondary"
							onclick={onPickDirectory}
							loading={pickingDirectory}
							loadingLabel="Opening directory picker"
							aria-label="Choose parent directory"
						>
							<Icon name="folder" class="size-4" />
							<span class="hidden sm:inline">Choose</span>
						</Button>
					</div>
				{/snippet}
			</Field>

			{#if recipe.requires.packageManagers.length > 0}
				<Field
					label="Package manager"
					help={packageManagerOptions.length > 0
						? 'Used by install steps in the resolved plan.'
						: 'Install a supported package manager, then rescan in the toolchain doctor.'}
					error={errors.packageManager}
					required
				>
					{#snippet children(context)}
						<Select
							id={context.controlId}
							value={request.packageManager}
							options={packageManagerOptions}
							placeholder={packageManagerOptions.length > 0
								? 'Choose a package manager'
								: 'No usable package manager detected'}
							disabled={packageManagerOptions.length === 0}
							required
							aria-required="true"
							invalid={context.invalid}
							aria-describedby={context.describedBy}
							onchange={(event) => updateBuiltIn('packageManager', event.currentTarget.value)}
						/>
					{/snippet}
				</Field>
			{/if}
		</div>
	</section>

	<aside class="rounded-panel border border-line bg-panel/65 p-5 shadow-sm sm:p-6">
		<p class="text-xs font-bold tracking-[0.12em] text-brand-strong uppercase">Finishing steps</p>
		<h2 class="mt-1 text-lg font-bold tracking-[-0.025em] text-ink">After scaffolding</h2>
		<div class="mt-5 grid gap-5">
			<Toggle
				checked={installRequired ? true : request.installDeps}
				label="Install dependencies"
				description={installRequired
					? 'This generator installs dependencies during scaffolding, so installation is required.'
					: 'Run the selected package manager after the generator.'}
				disabled={installRequired}
				onchange={(event) => updateBuiltIn('installDeps', event.currentTarget.checked)}
			/>
			<div class="h-px bg-line"></div>
			<Toggle
				checked={request.gitInit}
				label="Initialise Git"
				description="Create a local repository after setup."
				onchange={(event) => updateBuiltIn('gitInit', event.currentTarget.checked)}
			/>
		</div>

		{#if !recipe.available}
			<div class="mt-6 rounded-control border border-warning/30 bg-warning-soft p-3.5">
				<div class="flex gap-2.5">
					<Icon name="triangleAlert" class="mt-0.5 size-4 shrink-0 text-warning" />
					<div>
						<p class="text-xs font-semibold text-warning">Toolchain action required</p>
						<ul class="mt-1.5 space-y-1 text-xs leading-5 text-ink-muted">
							{#each recipe.unavailableReasons as reason (reason)}
								<li>{reason}</li>
							{/each}
						</ul>
						<a
							href={resolve('/toolchain')}
							class="mt-2 inline-flex rounded-md text-xs font-semibold text-warning underline decoration-warning/40 underline-offset-4 focus-visible:ring-3 focus-visible:ring-warning/20 focus-visible:outline-none"
						>
							Open toolchain doctor
						</a>
					</div>
				</div>
			</div>
		{/if}
	</aside>
</div>

{#if shownFields.length > 0}
	<section class="mt-6 rounded-panel border border-line bg-panel/80 p-5 shadow-sm sm:p-6">
		<div class="border-b border-line pb-5">
			<p class="text-xs font-bold tracking-[0.12em] text-brand-strong uppercase">Recipe options</p>
			<h2 class="mt-1 text-lg font-bold tracking-[-0.025em] text-ink">
				Configure {recipe.name}
			</h2>
			<p class="mt-1 text-sm leading-6 text-ink-muted">
				These controls come directly from the recipe manifest.
			</p>
		</div>

		<div class="mt-6 grid gap-6 lg:grid-cols-2">
			{#each shownFields as field (field.id)}
				<div class={field.type === 'multiselect' ? 'lg:col-span-2' : ''}>
					<ManifestField
						{field}
						value={request.answers[field.id]}
						error={errors[field.id]}
						onChange={(value) => updateAnswer(field.id, value)}
					/>
				</div>
			{/each}
		</div>
	</section>
{/if}
