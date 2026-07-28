<script lang="ts">
	import type { RecipeSummary } from '$lib/api';
	import Icon from '$lib/components/icons/Icon.svelte';
	import { Badge, Button } from '$lib/components/ui';
	import RecipeIcon from './RecipeIcon.svelte';

	interface Props {
		recipe: RecipeSummary;
		onSelect: (recipe: RecipeSummary) => void;
	}

	let { recipe, onSelect }: Props = $props();

	function formatVerifiedDate(value: string) {
		const date = new Date(`${value}T00:00:00Z`);
		if (Number.isNaN(date.getTime())) return value;
		return new Intl.DateTimeFormat(undefined, {
			month: 'short',
			year: 'numeric',
			timeZone: 'UTC'
		}).format(date);
	}
</script>

<article
	class="group flex min-h-64 min-w-0 flex-col rounded-panel border border-line bg-panel/80 p-5 shadow-sm transition-[transform,border-color,box-shadow,background-color] duration-200 ease-out-smooth hover:-translate-y-0.5 hover:border-line-strong hover:bg-panel hover:shadow-panel"
>
	<div class="flex items-start justify-between gap-4">
		<div
			class="flex size-11 shrink-0 items-center justify-center rounded-xl border border-line bg-panel-raised text-brand-strong shadow-sm"
			aria-hidden="true"
		>
			<RecipeIcon icon={recipe.icon} class="size-7" />
		</div>
		<Badge tone={recipe.available ? 'success' : 'warning'} size="sm" dot>
			{recipe.available ? 'Ready' : 'Needs setup'}
		</Badge>
	</div>

	<div class="mt-5 min-w-0 flex-1">
		<div class="flex items-center gap-2">
			<h2 class="truncate text-base font-bold tracking-[-0.02em] text-ink">{recipe.name}</h2>
			<span class="text-[0.625rem] font-bold tracking-[0.12em] text-ink-faint uppercase">
				{recipe.category}
			</span>
		</div>
		<p class="mt-2 line-clamp-3 text-sm leading-6 text-ink-muted">{recipe.description}</p>

		{#if recipe.tags.length > 0}
			<ul class="mt-4 flex flex-wrap gap-1.5" aria-label={`${recipe.name} tags`}>
				{#each recipe.tags.slice(0, 4) as tag (tag)}
					<li>
						<Badge size="sm">{tag}</Badge>
					</li>
				{/each}
			</ul>
		{/if}
	</div>

	{#if !recipe.available && recipe.unavailableReasons.length > 0}
		<p class="mt-4 line-clamp-2 text-xs leading-5 text-warning">
			{recipe.unavailableReasons.join(' · ')}
		</p>
	{/if}

	<div class="mt-5 flex items-center justify-between gap-3 border-t border-line pt-4">
		<div class="min-w-0">
			<span class="block truncate text-xs text-ink-faint">
				{recipe.defaultPackageManager || 'Toolchain required'}
			</span>
			<time
				datetime={recipe.verifiedAt}
				class="mt-0.5 block text-[0.625rem] text-ink-faint"
				title={`Recipe verified on ${recipe.verifiedAt}`}
			>
				Verified {formatVerifiedDate(recipe.verifiedAt)}
			</time>
			{#if recipe.installPolicy === 'required'}
				<span class="mt-0.5 block text-[0.625rem] font-semibold text-warning">
					Generator installs dependencies
				</span>
			{/if}
		</div>
		<Button
			size="sm"
			variant={recipe.available ? 'primary' : 'secondary'}
			onclick={() => onSelect(recipe)}
			aria-label={`Configure ${recipe.name}`}
		>
			{recipe.available ? 'Configure' : 'View setup'}
			<Icon name="arrowRight" class="size-3.5" />
		</Button>
	</div>
</article>
