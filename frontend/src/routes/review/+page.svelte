<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import {
		api,
		copyText,
		toErrorMessage,
		type Plan,
		type Recipe,
		type ScaffoldRequest
	} from '$lib/api';
	import { WizardProgress } from '$lib/components/form';
	import Icon from '$lib/components/icons/Icon.svelte';
	import { PlanStepCard } from '$lib/components/plan';
	import { Badge, Button, EmptyState } from '$lib/components/ui';
	import { visibleAnswers } from '$lib/form';
	import { jobRuntime, wizard } from '$lib/stores';

	let recipe = $state<Recipe | null>(null);
	let request = $state<ScaffoldRequest | null>(null);
	let executionRequest = $state<ScaffoldRequest | null>(null);
	let plan = $state<Plan | null>(null);
	let loading = $state(true);
	let error = $state('');
	let starting = $state(false);
	let copiedStepId = $state('');
	let copyError = $state('');
	const reviewSkeletons = [0, 1, 2] as const;

	async function buildPlan() {
		const snapshot = wizard.snapshot;
		recipe = snapshot.recipe;
		request = snapshot.request;
		if (!recipe || !request) {
			error = 'Complete the configuration step before reviewing commands.';
			loading = false;
			return;
		}

		executionRequest = {
			...request,
			installDeps: recipe.installPolicy === 'required' ? true : request.installDeps,
			answers: visibleAnswers(recipe.fields, request.answers)
		};
		loading = true;
		error = '';
		try {
			plan = await api.scaffold.plan(executionRequest);
			wizard.setPlan(plan);
		} catch (caught) {
			error = toErrorMessage(caught);
		} finally {
			loading = false;
		}
	}

	async function copyCommand(stepId: string, display: string) {
		copyError = '';
		try {
			await copyText(display);
			copiedStepId = stepId;
			window.setTimeout(() => {
				if (copiedStepId === stepId) copiedStepId = '';
			}, 1500);
		} catch (caught) {
			copyError = toErrorMessage(caught);
		}
	}

	async function startJob() {
		if (!executionRequest || !plan) return;
		starting = true;
		error = '';
		try {
			const job = await api.scaffold.start(executionRequest);
			wizard.setJob(job);
			jobRuntime.begin(job.id);
			await goto(resolve('/run'));
		} catch (caught) {
			error = toErrorMessage(caught);
		} finally {
			starting = false;
		}
	}

	onMount(() => {
		void buildPlan();
	});
</script>

<svelte:head>
	<title>Review commands · Nodesmith</title>
</svelte:head>

<WizardProgress current="review" />

<header
	class="mt-8 flex flex-col gap-4 border-b border-line pb-6 lg:flex-row lg:items-end lg:justify-between"
>
	<div>
		<a
			href={resolve('/configure')}
			class="inline-flex items-center gap-1.5 rounded-md text-xs font-semibold text-ink-faint transition-colors hover:text-ink focus-visible:ring-3 focus-visible:ring-brand/25 focus-visible:outline-none"
		>
			<span aria-hidden="true">←</span> Edit configuration
		</a>
		<p class="mt-5 text-xs font-bold tracking-[0.12em] text-brand-strong uppercase">
			Mandatory dry run
		</p>
		<h1 class="mt-1 text-3xl font-bold tracking-[-0.04em] text-ink">Review every command</h1>
		<p class="mt-2 max-w-2xl text-sm leading-6 text-ink-muted">
			This is the exact execution plan from the backend. Nodesmith will run these commands in order
			and will not invoke a shell.
		</p>
	</div>
	{#if plan}
		<div class="flex flex-wrap items-center gap-2">
			<Badge tone="accent">{plan.steps.length} {plan.steps.length === 1 ? 'step' : 'steps'}</Badge>
			<Badge>{plan.hash.slice(0, 10)}</Badge>
		</div>
	{/if}
</header>

{#if loading}
	<div class="mt-6 grid gap-4" aria-label="Resolving project commands" aria-busy="true">
		{#each reviewSkeletons as index (index)}
			<div class="h-40 animate-pulse rounded-panel border border-line bg-panel/70"></div>
		{/each}
	</div>
{:else if error && !plan}
	<div class="mt-6">
		<EmptyState title="The plan couldn’t be resolved" description={error}>
			{#snippet icon()}<Icon name="triangleAlert" class="size-5 text-danger" />{/snippet}
			{#snippet action()}
				<div class="flex flex-wrap justify-center gap-2">
					<Button variant="secondary" onclick={buildPlan}>
						<Icon name="refresh" class="size-4" />
						Try again
					</Button>
					<Button variant="ghost" onclick={() => goto(resolve('/configure'))}
						>Edit configuration</Button
					>
				</div>
			{/snippet}
		</EmptyState>
	</div>
{:else if plan}
	{#if plan.warnings.length > 0}
		<section
			class="mt-6 rounded-panel border border-warning/30 bg-warning-soft p-4"
			aria-label="Plan warnings"
		>
			<div class="flex items-start gap-3">
				<Icon name="triangleAlert" class="mt-0.5 size-5 shrink-0 text-warning" />
				<div>
					<h2 class="text-sm font-semibold text-warning">Review these warnings</h2>
					<ul class="mt-2 space-y-1 text-sm leading-6 text-ink-muted">
						{#each plan.warnings as warning (warning)}
							<li>{warning}</li>
						{/each}
					</ul>
				</div>
			</div>
		</section>
	{/if}

	<section class="mt-6 grid gap-4" aria-label="Resolved command plan">
		{#each plan.steps as step, index (step.id)}
			<PlanStepCard
				{step}
				{index}
				copied={copiedStepId === step.id}
				onCopy={(display) => copyCommand(step.id, display)}
			/>
		{/each}
	</section>

	<section
		class="sticky bottom-0 z-10 mt-6 flex flex-col gap-4 rounded-panel border border-line-strong bg-overlay/95 p-4 shadow-float backdrop-blur-xl sm:flex-row sm:items-center sm:justify-between"
	>
		<div class="min-w-0">
			<p class="text-xs font-semibold text-ink">Target directory</p>
			<p class="mt-1 truncate font-mono text-xs text-ink-muted" title={plan.projectDir}>
				{plan.projectDir}
			</p>
			{#if copyError}<p class="mt-1 text-xs text-danger" role="alert">{copyError}</p>{/if}
			{#if error}<p class="mt-1 text-xs text-danger" role="alert">{error}</p>{/if}
		</div>
		<div class="flex shrink-0 items-center justify-end gap-2">
			<Button variant="ghost" onclick={() => goto(resolve('/configure'))}>Back</Button>
			<Button onclick={startJob} loading={starting}>
				{#snippet icon()}<Icon name="play" class="size-4" />{/snippet}
				Run {plan.steps.length}
				{plan.steps.length === 1 ? 'step' : 'steps'}
			</Button>
		</div>
	</section>
{/if}
