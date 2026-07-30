<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import {
		api,
		openExternalUrl,
		toErrorMessage,
		type Preset,
		type Recipe,
		type ScaffoldRequest
	} from '$lib/api';
	import { DynamicRecipeForm, WizardProgress } from '$lib/components/form';
	import Icon from '$lib/components/icons/Icon.svelte';
	import ReleaseAgeControl from '$lib/components/settings/ReleaseAgeControl.svelte';
	import { Button, EmptyState, TextInput } from '$lib/components/ui';
	import {
		createDefaultAnswers,
		validatePortableProjectName,
		validateVisibleRequiredText,
		type FieldErrors
	} from '$lib/form';
	import { wizard } from '$lib/stores';
	import { describeReleaseAge, resolveReleaseAge } from '$lib/settings/releaseAge';
	import {
		availablePackageManagers,
		repairPackageManagerSelection
	} from '$lib/toolchain/package-managers';

	let recipe = $state<Recipe | null>(null);
	let request = $state<ScaffoldRequest | null>(null);
	let loading = $state(true);
	let error = $state('');
	let errors = $state<FieldErrors>({});
	let pickingDirectory = $state(false);
	let presetName = $state('');
	let savingPreset = $state(false);
	let presetNotice = $state('');
	let usablePackageManagers = $state<string[]>([]);
	let globalMinimumReleaseAge = $state<number | null>(null);
	let releaseAgeError = $state('');

	async function initialise() {
		loading = true;
		error = '';
		try {
			const snapshot = wizard.snapshot;
			if (snapshot.recipe) {
				recipe = snapshot.recipe;
			} else if (snapshot.request?.recipeId) {
				recipe = await api.recipes.get(snapshot.request.recipeId);
				wizard.selectRecipe(recipe);
			}

			if (!recipe) {
				throw new Error('Choose a recipe from the catalogue before configuring a project.');
			}

			const [toolchain, settings] = await Promise.all([
				api.toolchain.detect(false),
				api.store.getSettings()
			]);
			globalMinimumReleaseAge = settings.minimumReleaseAge;
			usablePackageManagers = availablePackageManagers(recipe.requires.packageManagers, toolchain);

			if (snapshot.request?.recipeId === recipe.id) {
				request = {
					...snapshot.request,
					parentDir: settings.defaultParentDir,
					installDeps: recipe.installPolicy === 'required' ? true : snapshot.request.installDeps,
					minimumReleaseAge: snapshot.request.minimumReleaseAge ?? null
				};
			} else {
				request = {
					recipeId: recipe.id,
					projectName: '',
					parentDir: settings.defaultParentDir,
					packageManager: '',
					installDeps: true,
					gitInit: true,
					minimumReleaseAge: null,
					answers: createDefaultAnswers(recipe.fields)
				};
			}
			request = {
				...request,
				packageManager: repairPackageManagerSelection(
					recipe.requires.packageManagers,
					toolchain,
					request.packageManager
				)
			};
			wizard.setRequest(request);
		} catch (caught) {
			error = toErrorMessage(caught);
		} finally {
			loading = false;
		}
	}

	function validate(): boolean {
		if (!recipe || !request) return false;
		const nextErrors = validateVisibleRequiredText(recipe.fields, request.answers);
		const projectNameError = validatePortableProjectName(request.projectName);
		if (projectNameError) nextErrors.projectName = projectNameError;
		if (request.parentDir.trim() === '') nextErrors.parentDir = 'Parent directory is required.';
		if (releaseAgeError) nextErrors.minimumReleaseAge = releaseAgeError;
		if (
			recipe.requires.packageManagers.length > 0 &&
			!usablePackageManagers.includes(request.packageManager)
		) {
			nextErrors.packageManager =
				usablePackageManagers.length === 0
					? 'No package manager supported by this recipe is currently usable.'
					: 'Choose an available package manager.';
		}
		errors = nextErrors;
		return Object.keys(nextErrors).length === 0;
	}

	async function pickDirectory() {
		if (!request) return;
		pickingDirectory = true;
		error = '';
		try {
			const selected = await api.scaffold.pickDirectory(request.parentDir);
			if (selected) request = { ...request, parentDir: selected };
		} catch (caught) {
			error = toErrorMessage(caught);
		} finally {
			pickingDirectory = false;
		}
	}

	async function continueToReview() {
		if (!request || !validate()) return;
		wizard.setRequest(request);
		await goto(resolve('/review'));
	}

	function setMinimumReleaseAge(minutes: number | null, message: string) {
		releaseAgeError = message;
		if (message || !request) return;
		request = { ...request, minimumReleaseAge: minutes };
		wizard.setRequest(request);
	}

	function inheritedReleaseAge() {
		if (!recipe) return '';
		return describeReleaseAge(
			resolveReleaseAge(null, recipe.minimumReleaseAge, globalMinimumReleaseAge)
		);
	}

	async function savePreset() {
		if (!request || !recipe || !validate()) return;
		if (presetName.trim() === '') {
			errors = { ...errors, presetName: 'Give this preset a name.' };
			return;
		}

		savingPreset = true;
		presetNotice = '';
		try {
			const preset: Preset = {
				id: '',
				name: presetName.trim(),
				request,
				createdAt: '',
				updatedAt: ''
			};
			await api.store.savePreset(preset);
			presetNotice = `Saved “${preset.name}”.`;
			presetName = '';
		} catch (caught) {
			error = toErrorMessage(caught);
		} finally {
			savingPreset = false;
		}
	}

	function openRecipeDocs() {
		if (!recipe) return;
		error = '';
		try {
			openExternalUrl(recipe.docsUrl);
		} catch (caught) {
			error = toErrorMessage(caught);
		}
	}

	onMount(() => {
		void initialise();
	});
</script>

<svelte:head>
	<title>Configure project · Nodesmith</title>
</svelte:head>

<WizardProgress current="configure" />

{#if loading}
	<div class="mt-8 grid gap-5" aria-busy="true" aria-label="Loading recipe configuration">
		<div class="h-36 animate-pulse rounded-panel border border-line bg-panel/70"></div>
		<div class="h-96 animate-pulse rounded-panel border border-line bg-panel/70"></div>
	</div>
{:else if error && (!recipe || !request)}
	<div class="mt-8">
		<EmptyState title="Configuration isn’t available" description={error}>
			{#snippet icon()}<Icon name="triangleAlert" class="size-5 text-danger" />{/snippet}
			{#snippet action()}
				<Button variant="secondary" onclick={() => goto(resolve('/'))}>
					<Icon name="grid" class="size-4" />
					Back to catalogue
				</Button>
			{/snippet}
		</EmptyState>
	</div>
{:else if recipe && request}
	<header
		class="mt-8 flex flex-col gap-4 border-b border-line pb-6 lg:flex-row lg:items-end lg:justify-between"
	>
		<div>
			<a
				href={resolve('/')}
				class="inline-flex items-center gap-1.5 rounded-md text-xs font-semibold text-ink-faint transition-colors hover:text-ink focus-visible:ring-3 focus-visible:ring-brand/25 focus-visible:outline-none"
			>
				<span aria-hidden="true">←</span> Catalogue
			</a>
			<p class="mt-5 text-xs font-bold tracking-[0.12em] text-brand-strong uppercase">
				{recipe.category} recipe
			</p>
			<h1 class="mt-1 text-3xl font-bold tracking-[-0.04em] text-ink">Configure {recipe.name}</h1>
			<p class="mt-2 max-w-2xl text-sm leading-6 text-ink-muted">{recipe.description}</p>
		</div>
		<Button variant="secondary" size="sm" class="self-start lg:self-auto" onclick={openRecipeDocs}>
			Recipe docs <Icon name="external" class="size-3.5" />
		</Button>
	</header>

	{#if error}
		<div
			class="mt-5 rounded-control border border-danger/30 bg-danger-soft px-4 py-3 text-sm text-danger"
			role="alert"
		>
			{error}
		</div>
	{/if}

	<form
		class="mt-6"
		onsubmit={(event) => {
			event.preventDefault();
			void continueToReview();
		}}
	>
		<DynamicRecipeForm
			{recipe}
			bind:request
			{errors}
			packageManagers={usablePackageManagers}
			{pickingDirectory}
			onPickDirectory={pickDirectory}
			onRequestChange={(nextRequest) => wizard.setRequest(nextRequest)}
		/>

		<section class="mt-6 rounded-panel border border-line bg-panel/80 p-5 shadow-sm sm:p-6">
			<div class="border-b border-line pb-5">
				<p class="text-xs font-bold tracking-[0.12em] text-brand-strong uppercase">Supply chain</p>
				<h2 class="mt-1 text-lg font-bold tracking-[-0.025em] text-ink">Minimum release age</h2>
				<p class="mt-1 max-w-3xl text-sm leading-6 text-ink-muted">
					Refuse package versions published more recently than this. Inherit uses the recipe default
					first, then the global default from Settings.
				</p>
			</div>

			<div class="mt-6 max-w-xl">
				<ReleaseAgeControl
					label="Minimum release age"
					inheritLabel="Inherit"
					inheritHint={inheritedReleaseAge()}
					help="This value is included when you save the configuration as a preset."
					value={request.minimumReleaseAge}
					onchange={setMinimumReleaseAge}
				/>
			</div>
		</section>

		<section
			class="mt-6 flex flex-col gap-4 rounded-panel border border-line bg-panel/65 p-4 sm:flex-row sm:items-end sm:justify-between"
		>
			<div class="w-full max-w-md">
				<label class="text-xs font-semibold text-ink" for="preset-name">Save this setup</label>
				<div class="mt-2 flex gap-2">
					<TextInput
						id="preset-name"
						value={presetName}
						placeholder="Preset name"
						invalid={Boolean(errors.presetName)}
						oninput={(event) => (presetName = event.currentTarget.value)}
					/>
					<Button
						variant="secondary"
						onclick={savePreset}
						loading={savingPreset}
						disabled={!request.projectName}
					>
						<Icon name="bookmark" class="size-4" />
						Save
					</Button>
				</div>
				{#if errors.presetName}
					<p class="mt-1.5 text-xs text-danger" role="alert">{errors.presetName}</p>
				{:else if presetNotice}
					<p class="mt-1.5 text-xs text-success" role="status">{presetNotice}</p>
				{/if}
			</div>

			<div class="flex items-center justify-end gap-2">
				<Button variant="ghost" onclick={() => goto(resolve('/'))}>Cancel</Button>
				<Button
					type="submit"
					disabled={!recipe.available}
					title={recipe.available ? undefined : 'Resolve the toolchain issues before continuing'}
				>
					Review commands
					<Icon name="arrowRight" class="size-4" />
				</Button>
			</div>
		</section>
	</form>
{/if}
