<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import {
		api,
		toErrorMessage,
		type HistoryEntry,
		type ScaffoldRequest,
		type Settings
	} from '$lib/api';
	import Icon from '$lib/components/icons/Icon.svelte';
	import { Badge, Button, EmptyState } from '$lib/components/ui';
	import { createDefaultAnswers } from '$lib/form';
	import { wizard } from '$lib/stores';

	let entries = $state<HistoryEntry[]>([]);
	let settings = $state<Settings | null>(null);
	let loading = $state(true);
	let clearing = $state(false);
	let busyId = $state('');
	let error = $state('');
	let loadError = $state('');
	let notice = $state('');
	const historySkeletons = [0, 1, 2, 3, 4] as const;

	function formatDate(value: string): string {
		const date = new Date(value);
		return Number.isNaN(date.valueOf())
			? 'Unknown date'
			: new Intl.DateTimeFormat(undefined, {
					dateStyle: 'medium',
					timeStyle: 'short'
				}).format(date);
	}

	function formatDuration(milliseconds: number): string {
		if (milliseconds <= 0) return '—';
		if (milliseconds < 1000) return `${milliseconds} ms`;
		if (milliseconds < 60_000) return `${(milliseconds / 1000).toFixed(1)} s`;
		return `${Math.floor(milliseconds / 60_000)}m ${Math.round((milliseconds % 60_000) / 1000)}s`;
	}

	function statusTone(state: string): 'success' | 'danger' | 'warning' | 'neutral' {
		if (state === 'success') return 'success';
		if (state === 'failed') return 'danger';
		if (state === 'cancelled') return 'warning';
		return 'neutral';
	}

	async function load() {
		loading = true;
		error = '';
		loadError = '';
		try {
			[entries, settings] = await Promise.all([
				api.store.listHistory(100),
				api.store.getSettings()
			]);
		} catch (caught) {
			loadError = toErrorMessage(caught);
		} finally {
			loading = false;
		}
	}

	async function reopen(entry: HistoryEntry) {
		if (!settings) return;
		busyId = entry.id;
		error = '';
		notice = '';
		try {
			await api.scaffold.openInEditor(entry.projectDir, settings.editor);
			notice = `Opened ${entry.projectName} in ${settings.editor}.`;
		} catch (caught) {
			error = toErrorMessage(caught);
		} finally {
			busyId = '';
		}
	}

	async function reveal(entry: HistoryEntry) {
		busyId = entry.id;
		error = '';
		notice = '';
		try {
			await api.scaffold.revealInFileManager(entry.projectDir);
			notice = `Revealed ${entry.projectName}.`;
		} catch (caught) {
			error = toErrorMessage(caught);
		} finally {
			busyId = '';
		}
	}

	async function rerun(entry: HistoryEntry) {
		busyId = entry.id;
		error = '';
		try {
			const [recipe, currentSettings] = await Promise.all([
				api.recipes.get(entry.recipeId),
				api.store.getSettings()
			]);
			const request: ScaffoldRequest = {
				recipeId: recipe.id,
				projectName: '',
				parentDir: currentSettings.defaultParentDir,
				packageManager: recipe.requires.packageManagers.includes(entry.packageManager)
					? entry.packageManager
					: recipe.requires.packageManagers.at(0) || '',
				installDeps: true,
				gitInit: true,
				answers: createDefaultAnswers(recipe.fields)
			};
			wizard.selectRecipe(recipe);
			wizard.loadRequest(request);
			await goto(resolve('/configure'));
		} catch (caught) {
			error = toErrorMessage(caught);
		} finally {
			busyId = '';
		}
	}

	async function clearHistory() {
		clearing = true;
		error = '';
		notice = '';
		try {
			await api.store.clearHistory();
			entries = [];
			notice = 'Project history cleared.';
		} catch (caught) {
			error = toErrorMessage(caught);
		} finally {
			clearing = false;
		}
	}

	onMount(() => {
		void load();
	});
</script>

<svelte:head>
	<title>Project history · Nodesmith</title>
</svelte:head>

<header
	class="flex flex-col gap-5 border-b border-line pb-6 lg:flex-row lg:items-end lg:justify-between"
>
	<div>
		<div
			class="flex items-center gap-2 text-xs font-bold tracking-[0.14em] text-brand-strong uppercase"
		>
			<span class="h-px w-7 bg-brand/60"></span>
			Local activity
		</div>
		<h1 class="mt-3 text-3xl font-bold tracking-[-0.04em] text-ink">Project history</h1>
		<p class="mt-2 max-w-2xl text-sm leading-6 text-ink-muted">
			Re-open generated projects or start the same recipe again through a fresh mandatory
			configuration and review.
		</p>
	</div>
	{#if entries.length > 0}
		<Button variant="danger" onclick={clearHistory} loading={clearing}>
			<Icon name="x" class="size-4" />
			Clear history
		</Button>
	{/if}
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
	<div class="mt-6 grid gap-3" aria-busy="true">
		{#each historySkeletons as index (index)}
			<div class="h-28 animate-pulse rounded-panel border border-line bg-panel/70"></div>
		{/each}
	</div>
{:else if entries.length === 0}
	{#if loadError}
		<div class="mt-6">
			<EmptyState title="Project history couldn’t be loaded" description={loadError}>
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
				title="No projects in history"
				description="Completed, failed, and cancelled project runs will appear here."
			>
				{#snippet icon()}<Icon name="history" class="size-5" />{/snippet}
				{#snippet action()}<Button onclick={() => goto(resolve('/'))}>Create a project</Button
					>{/snippet}
			</EmptyState>
		</div>
	{/if}
{:else}
	<section
		class="mt-6 overflow-hidden rounded-panel border border-line bg-panel/70"
		aria-label="Project history"
	>
		<ul class="divide-y divide-line">
			{#each entries as entry (entry.id)}
				<li class="p-4 transition-colors hover:bg-panel-raised/45 sm:p-5">
					<div class="flex flex-col gap-4 xl:flex-row xl:items-center">
						<div class="flex min-w-0 flex-1 items-start gap-3">
							<span
								class={[
									'flex size-10 shrink-0 items-center justify-center rounded-xl border',
									entry.state === 'success' && 'border-success/25 bg-success-soft text-success',
									entry.state === 'failed' && 'border-danger/30 bg-danger-soft text-danger',
									entry.state === 'cancelled' && 'border-warning/30 bg-warning-soft text-warning',
									!['success', 'failed', 'cancelled'].includes(entry.state) &&
										'border-line bg-panel-raised text-ink-faint'
								]}
							>
								<Icon
									name={entry.state === 'success'
										? 'circleCheck'
										: entry.state === 'failed'
											? 'circleX'
											: 'history'}
									class="size-4.5"
								/>
							</span>
							<div class="min-w-0">
								<div class="flex flex-wrap items-center gap-2">
									<h2 class="truncate text-sm font-bold text-ink">{entry.projectName}</h2>
									<Badge tone={statusTone(entry.state)} size="sm">{entry.state}</Badge>
									<Badge size="sm">{entry.recipeName}</Badge>
								</div>
								<p class="mt-1 truncate font-mono text-xs text-ink-faint" title={entry.projectDir}>
									{entry.projectDir}
								</p>
								<div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[0.6875rem] text-ink-faint">
									<span>{formatDate(entry.createdAt)}</span>
									<span>{formatDuration(entry.durationMs)}</span>
									{#if entry.packageManager}<span>{entry.packageManager}</span>{/if}
									<span>plan {entry.planHash.slice(0, 8)}</span>
								</div>
								{#if entry.error}
									<p class="mt-2 line-clamp-2 text-xs leading-5 text-danger">{entry.error}</p>
								{/if}
							</div>
						</div>
						<div class="flex shrink-0 flex-wrap items-center justify-end gap-2">
							<Button
								variant="ghost"
								size="sm"
								onclick={() => reveal(entry)}
								disabled={busyId === entry.id}
							>
								<Icon name="folder" class="size-3.5" />
								Reveal
							</Button>
							<Button
								variant="secondary"
								size="sm"
								onclick={() => reopen(entry)}
								loading={busyId === entry.id}
							>
								<Icon name="external" class="size-3.5" />
								Re-open
							</Button>
							<Button size="sm" onclick={() => rerun(entry)} loading={busyId === entry.id}>
								<Icon name="refresh" class="size-3.5" />
								Run recipe again
							</Button>
						</div>
					</div>
				</li>
			{/each}
		</ul>
	</section>
{/if}
