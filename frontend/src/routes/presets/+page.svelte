<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import { api, toErrorMessage, type Preset } from '$lib/api';
	import Icon from '$lib/components/icons/Icon.svelte';
	import { Badge, Button, EmptyState } from '$lib/components/ui';
	import { formatReleaseAge } from '$lib/settings/releaseAge';
	import { wizard } from '$lib/stores';

	let presets = $state<Preset[]>([]);
	let loading = $state(true);
	let error = $state('');
	let loadError = $state('');
	let busyId = $state('');
	let notice = $state('');
	const presetSkeletons = [0, 1, 2, 3] as const;

	function formatDate(value: string): string {
		const date = new Date(value);
		return Number.isNaN(date.valueOf())
			? 'Unknown date'
			: new Intl.DateTimeFormat(undefined, {
					dateStyle: 'medium',
					timeStyle: 'short'
				}).format(date);
	}

	async function load() {
		loading = true;
		error = '';
		loadError = '';
		try {
			presets = await api.store.listPresets();
		} catch (caught) {
			loadError = toErrorMessage(caught);
		} finally {
			loading = false;
		}
	}

	async function usePreset(preset: Preset) {
		busyId = preset.id;
		error = '';
		try {
			const recipe = await api.recipes.get(preset.request.recipeId);
			wizard.selectRecipe(recipe);
			wizard.loadRequest(preset.request);
			await goto(resolve('/configure'));
		} catch (caught) {
			error = toErrorMessage(caught);
		} finally {
			busyId = '';
		}
	}

	async function deletePreset(preset: Preset) {
		busyId = preset.id;
		error = '';
		notice = '';
		try {
			await api.store.deletePreset(preset.id);
			presets = presets.filter((item) => item.id !== preset.id);
			notice = `Deleted “${preset.name}”.`;
		} catch (caught) {
			error = toErrorMessage(caught);
		} finally {
			busyId = '';
		}
	}

	onMount(() => {
		void load();
	});
</script>

<svelte:head>
	<title>Presets · Nodesmith</title>
</svelte:head>

<header
	class="flex flex-col gap-5 border-b border-line pb-6 lg:flex-row lg:items-end lg:justify-between"
>
	<div>
		<div
			class="flex items-center gap-2 text-xs font-bold tracking-[0.14em] text-brand-strong uppercase"
		>
			<span class="h-px w-7 bg-brand/60"></span>
			Reusable setups
		</div>
		<h1 class="mt-3 text-3xl font-bold tracking-[-0.04em] text-ink">Presets</h1>
		<p class="mt-2 max-w-2xl text-sm leading-6 text-ink-muted">
			Save configured recipe answers once, then load them here whenever you start a similar project.
		</p>
	</div>
	<Button onclick={() => goto(resolve('/'))}>
		<Icon name="plus" class="size-4" />
		Create from catalogue
	</Button>
</header>

{#if notice}
	<p
		class="mt-5 rounded-control border border-success/25 bg-success-soft px-4 py-3 text-sm text-success"
		role="status"
	>
		{notice}
	</p>
{/if}
{#if error}
	<p
		class="mt-5 rounded-control border border-danger/30 bg-danger-soft px-4 py-3 text-sm text-danger"
		role="alert"
	>
		{error}
	</p>
{/if}

{#if loading}
	<div class="mt-6 grid gap-4 lg:grid-cols-2" aria-busy="true">
		{#each presetSkeletons as index (index)}
			<div class="h-52 animate-pulse rounded-panel border border-line bg-panel/70"></div>
		{/each}
	</div>
{:else if presets.length === 0}
	{#if loadError}
		<div class="mt-6">
			<EmptyState title="Presets couldn’t be loaded" description={loadError}>
				{#snippet icon()}<Icon name="triangleAlert" class="size-5 text-danger" />{/snippet}
				{#snippet action()}
					<Button variant="secondary" onclick={load}>
						<Icon name="refresh" class="size-4" />
						Try again
					</Button>
				{/snippet}
			</EmptyState>
		</div>
	{:else}
		<div class="mt-6">
			<EmptyState
				title="No presets saved yet"
				description="Configure a recipe and use “Save this setup” before reviewing its commands."
			>
				{#snippet icon()}<Icon name="bookmark" class="size-5" />{/snippet}
				{#snippet action()}<Button onclick={() => goto(resolve('/'))}>Choose a recipe</Button
					>{/snippet}
			</EmptyState>
		</div>
	{/if}
{:else}
	<section class="mt-6 grid gap-4 lg:grid-cols-2" aria-label="Saved presets">
		{#each presets as preset (preset.id)}
			<article
				class="flex min-w-0 flex-col rounded-panel border border-line bg-panel/75 p-5 shadow-sm"
			>
				<div class="flex items-start justify-between gap-4">
					<div class="flex min-w-0 items-start gap-3">
						<span
							class="flex size-10 shrink-0 items-center justify-center rounded-xl border border-brand/25 bg-brand-soft text-brand-strong"
						>
							<Icon name="bookmark" class="size-4.5" />
						</span>
						<div class="min-w-0">
							<h2 class="truncate text-base font-bold text-ink">{preset.name}</h2>
							<p class="mt-0.5 text-xs text-ink-faint">Updated {formatDate(preset.updatedAt)}</p>
						</div>
					</div>
					<Badge tone="accent">{preset.request.recipeId}</Badge>
				</div>

				<dl class="mt-5 grid gap-2 rounded-control border border-line bg-canvas/35 p-3 text-xs">
					<div class="grid grid-cols-[7rem_minmax(0,1fr)] gap-2">
						<dt class="text-ink-faint">Project name</dt>
						<dd class="truncate font-mono text-ink">{preset.request.projectName || 'Not set'}</dd>
					</div>
					<div class="grid grid-cols-[7rem_minmax(0,1fr)] gap-2">
						<dt class="text-ink-faint">Package manager</dt>
						<dd class="font-mono text-ink">{preset.request.packageManager || 'Not required'}</dd>
					</div>
					<div class="grid grid-cols-[7rem_minmax(0,1fr)] gap-2">
						<dt class="text-ink-faint">Release age</dt>
						<dd class="text-ink">
							{preset.request.minimumReleaseAge === null
								? 'Inherit'
								: formatReleaseAge(preset.request.minimumReleaseAge)}
						</dd>
					</div>
					<div class="grid grid-cols-[7rem_minmax(0,1fr)] gap-2">
						<dt class="text-ink-faint">Recipe answers</dt>
						<dd class="text-ink">{Object.keys(preset.request.answers).length}</dd>
					</div>
				</dl>

				<div class="mt-5 flex items-center justify-end gap-2 border-t border-line pt-4">
					<Button
						variant="danger"
						size="sm"
						onclick={() => deletePreset(preset)}
						disabled={busyId === preset.id}
					>
						Delete
					</Button>
					<Button size="sm" onclick={() => usePreset(preset)} loading={busyId === preset.id}>
						Use preset
						{#if busyId !== preset.id}<Icon name="arrowRight" class="size-3.5" />{/if}
					</Button>
				</div>
			</article>
		{/each}
	</section>
{/if}
