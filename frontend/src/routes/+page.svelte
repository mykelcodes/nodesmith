<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import { api, toErrorMessage, type RecipeSummary, type ScaffoldRequest } from '$lib/api';
	import { RecipeCard } from '$lib/components/catalogue';
	import Icon from '$lib/components/icons/Icon.svelte';
	import { Button, EmptyState, Select, TextInput } from '$lib/components/ui';
	import { createDefaultAnswers } from '$lib/form';
	import { wizard } from '$lib/stores';

	let recipes = $state<RecipeSummary[]>([]);
	let loading = $state(true);
	let error = $state('');
	let search = $state('');
	let category = $state('');
	let tag = $state('');
	let selectingId = $state('');
	let reloading = $state(false);
	let notice = $state('');
	const catalogueSkeletons = [0, 1, 2, 3, 4, 5] as const;

	const categoryOptions = $derived([
		{ value: '', label: 'All categories' },
		...[...new Set(recipes.map((recipe) => recipe.category))]
			.sort()
			.map((value) => ({ value, label: value.charAt(0).toUpperCase() + value.slice(1) }))
	]);
	const tagOptions = $derived([
		{ value: '', label: 'All tags' },
		...[...new Set(recipes.flatMap((recipe) => recipe.tags))]
			.sort()
			.map((value) => ({ value, label: value }))
	]);
	const filteredRecipes = $derived.by(() => {
		const query = search.trim().toLocaleLowerCase();
		return recipes.filter((recipe) => {
			if (category && recipe.category !== category) return false;
			if (tag && !recipe.tags.includes(tag)) return false;
			if (!query) return true;
			return [recipe.name, recipe.description, ...recipe.tags].some((value) =>
				value.toLocaleLowerCase().includes(query)
			);
		});
	});
	const readyCount = $derived(recipes.filter((recipe) => recipe.available).length);

	async function loadRecipes() {
		loading = true;
		error = '';
		try {
			recipes = await api.recipes.list();
		} catch (caught) {
			error = toErrorMessage(caught);
		} finally {
			loading = false;
		}
	}

	async function reloadRecipes() {
		reloading = true;
		notice = '';
		try {
			const result = await api.recipes.reload();
			await loadRecipes();
			notice =
				result.warnings.length > 0
					? `${result.count} recipes loaded. ${result.warnings.join(' ')}`
					: `${result.count} recipes loaded successfully.`;
		} catch (caught) {
			error = toErrorMessage(caught);
		} finally {
			reloading = false;
		}
	}

	async function selectRecipe(summary: RecipeSummary) {
		selectingId = summary.id;
		error = '';
		try {
			const recipe = await api.recipes.get(summary.id);
			const current = wizard.snapshot;
			wizard.selectRecipe(recipe);

			if (current.recipe?.id !== recipe.id || !current.request) {
				const settings = await api.store.getSettings();
				const request: ScaffoldRequest = {
					recipeId: recipe.id,
					projectName: '',
					parentDir: settings.defaultParentDir,
					packageManager:
						summary.defaultPackageManager || recipe.requires.packageManagers.at(0) || '',
					installDeps: true,
					gitInit: true,
					minimumReleaseAge: null,
					answers: createDefaultAnswers(recipe.fields)
				};
				wizard.setRequest(request);
			}
			await goto(resolve('/configure'));
		} catch (caught) {
			error = toErrorMessage(caught);
		} finally {
			selectingId = '';
		}
	}

	async function openRecipeDirectory() {
		error = '';
		try {
			await api.recipes.openRecipeDir();
		} catch (caught) {
			error = toErrorMessage(caught);
		}
	}

	onMount(() => {
		void loadRecipes();

		// The registry is loaded before this interface exists, so the backend
		// replays its startup report once the DOM is ready. Without this a user
		// recipe that failed to validate is skipped in silence at launch.
		let unsubscribe: (() => void) | undefined;
		try {
			unsubscribe = api.events.onRecipesReloaded((result) => {
				if (result.warnings.length === 0 && result.overrides.length === 0) return;
				notice =
					result.warnings.length > 0
						? `${result.count} recipes loaded. ${result.warnings.join(' ')}`
						: `${result.count} recipes loaded. Overrides applied: ${result.overrides.join(', ')}.`;
			});
		} catch {
			// Outside the desktop runtime there is no event bridge to attach to.
		}
		return () => unsubscribe?.();
	});
</script>

<svelte:head>
	<title>Recipe catalogue · Nodesmith</title>
</svelte:head>

<header class="flex flex-col gap-6 xl:flex-row xl:items-end xl:justify-between">
	<div class="max-w-3xl">
		<div
			class="flex items-center gap-2 text-xs font-bold tracking-[0.14em] text-brand-strong uppercase"
		>
			<span class="h-px w-7 bg-brand/60"></span>
			Project recipes
		</div>
		<h1 class="mt-3 text-3xl font-bold tracking-[-0.04em] text-ink sm:text-4xl">
			Build the right starting point.
		</h1>
		<p class="mt-3 max-w-2xl text-sm leading-6 text-ink-muted sm:text-base sm:leading-7">
			Pick a stack, configure its supported options, then review every command before anything runs
			on your machine.
		</p>
	</div>

	<div class="flex flex-wrap items-center gap-2 text-xs text-ink-muted">
		<span class="rounded-lg border border-line bg-panel px-3 py-2">
			<span class="font-bold text-ink">{recipes.length}</span> recipes
		</span>
		<span class="rounded-lg border border-line bg-panel px-3 py-2">
			<span class="font-bold text-success">{readyCount}</span> ready locally
		</span>
		<Button variant="secondary" size="sm" onclick={reloadRecipes} loading={reloading}>
			{#snippet icon()}<Icon name="refresh" class="size-3.5" />{/snippet}
			Reload
		</Button>
	</div>
</header>

<section
	class="mt-8 flex flex-col gap-3 rounded-panel border border-line bg-panel/65 p-3 shadow-sm sm:flex-row sm:items-center"
	aria-label="Catalogue filters"
>
	<div class="min-w-0 flex-1">
		<TextInput
			type="search"
			value={search}
			placeholder="Search recipes, tags, or descriptions"
			aria-label="Search recipes"
			oninput={(event) => (search = event.currentTarget.value)}
		>
			{#snippet leading()}<Icon name="search" class="size-4" />{/snippet}
		</TextInput>
	</div>
	<Select
		value={category}
		options={categoryOptions}
		class="sm:w-48"
		aria-label="Filter recipes by category"
		onchange={(event) => (category = event.currentTarget.value)}
	/>
	<Select
		value={tag}
		options={tagOptions}
		class="sm:w-44"
		aria-label="Filter recipes by tag"
		onchange={(event) => (tag = event.currentTarget.value)}
	/>
</section>

{#if notice}
	<div
		class="mt-4 rounded-control border border-success/25 bg-success-soft px-4 py-3 text-sm text-success"
		role="status"
	>
		{notice}
	</div>
{/if}

{#if loading}
	<div
		class="mt-6 grid gap-4 md:grid-cols-2 xl:grid-cols-3"
		aria-label="Loading recipes"
		aria-busy="true"
	>
		{#each catalogueSkeletons as index (index)}
			<div class="min-h-64 animate-pulse rounded-panel border border-line bg-panel/70 p-5">
				<div class="size-11 rounded-xl bg-overlay"></div>
				<div class="mt-5 h-4 w-2/5 rounded bg-overlay"></div>
				<div class="mt-4 h-3 w-full rounded bg-overlay"></div>
				<div class="mt-2 h-3 w-4/5 rounded bg-overlay"></div>
			</div>
		{/each}
	</div>
{:else if error}
	<div class="mt-6">
		<EmptyState title="Couldn’t load the recipe catalogue" description={error}>
			{#snippet icon()}<Icon name="triangleAlert" class="size-5 text-danger" />{/snippet}
			{#snippet action()}
				<Button variant="secondary" onclick={loadRecipes}>
					<Icon name="refresh" class="size-4" />
					Try again
				</Button>
			{/snippet}
		</EmptyState>
	</div>
{:else if recipes.length === 0}
	<div class="mt-6">
		<EmptyState
			title="No recipes are installed"
			description="Reload the bundled and local recipe directories, or open the local folder to add a manifest."
		>
			{#snippet action()}
				<div class="flex flex-wrap justify-center gap-2">
					<Button variant="secondary" onclick={reloadRecipes}>Reload recipes</Button>
					<Button variant="ghost" onclick={openRecipeDirectory}>
						<Icon name="folder" class="size-4" />
						Open recipe folder
					</Button>
				</div>
			{/snippet}
		</EmptyState>
	</div>
{:else if filteredRecipes.length === 0}
	<div class="mt-6">
		<EmptyState
			compact
			title="No recipes match"
			description="Try a broader search or clear the category filter."
		>
			{#snippet icon()}<Icon name="search" class="size-5" />{/snippet}
			{#snippet action()}
				<Button
					variant="secondary"
					onclick={() => {
						search = '';
						category = '';
						tag = '';
					}}>Clear filters</Button
				>
			{/snippet}
		</EmptyState>
	</div>
{:else}
	<div class="mt-6 grid auto-rows-fr gap-4 md:grid-cols-2 xl:grid-cols-3">
		{#each filteredRecipes as recipe (recipe.id)}
			<div class={`h-full ${selectingId === recipe.id ? 'pointer-events-none opacity-60' : ''}`}>
				<RecipeCard {recipe} onSelect={selectRecipe} />
			</div>
		{/each}
	</div>
{/if}
