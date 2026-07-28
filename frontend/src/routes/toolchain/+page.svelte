<script lang="ts">
	import { onMount } from 'svelte';
	import { api, toErrorMessage, type Tool, type Toolchain } from '$lib/api';
	import Icon from '$lib/components/icons/Icon.svelte';
	import { Badge, Button, EmptyState } from '$lib/components/ui';

	const installGuidance: Record<string, string> = {
		node: 'Install Node.js 20 or newer from nodejs.org, fnm, nvm, or Volta.',
		npm: 'npm is included with Node.js. Reinstall Node if npm is missing.',
		npx: 'npx is included with npm. Confirm your Node installation is on PATH.',
		pnpm: 'Install pnpm with Corepack or the official standalone installer.',
		yarn: 'Enable Yarn through Corepack or install it from yarnpkg.com.',
		bun: 'Install Bun from bun.sh and make sure its bin directory is on PATH.',
		git: 'Install Git from git-scm.com or your operating system package manager.',
		go: 'Install the current Go toolchain from go.dev/dl.',
		cargo: 'Install Rust and Cargo with rustup from rustup.rs.',
		gh: 'Install GitHub CLI from cli.github.com.',
		wails: 'Install the Wails v2 CLI, then ensure the wails binary is on PATH.'
	};

	let toolchain = $state<Toolchain | null>(null);
	let pathOverride = $state('');
	let loading = $state(true);
	let rescanning = $state(false);
	let saving = $state(false);
	let error = $state('');
	let notice = $state('');
	const toolSkeletons = [0, 1, 2, 3, 4, 5, 6, 7, 8] as const;

	const presentCount = $derived(toolchain?.tools.filter((tool) => tool.present).length ?? 0);
	const missingCount = $derived((toolchain?.tools.length ?? 0) - presentCount);

	function toolLabel(tool: Tool): string {
		if (tool.name.toLocaleLowerCase() === 'wails') return 'Wails v2';
		return tool.name;
	}

	async function load(force = false) {
		if (force) rescanning = true;
		else loading = true;
		error = '';
		notice = '';
		try {
			const [detected, settings] = await Promise.all([
				api.toolchain.detect(force),
				api.store.getSettings()
			]);
			toolchain = detected;
			pathOverride = settings.pathOverride;
			if (force)
				notice = `Rescan complete: ${detected.tools.filter((tool) => tool.present).length} tools found.`;
		} catch (caught) {
			error = toErrorMessage(caught);
		} finally {
			loading = false;
			rescanning = false;
		}
	}

	async function savePathOverride() {
		saving = true;
		error = '';
		notice = '';
		try {
			await api.toolchain.setPathOverride(pathOverride);
			toolchain = await api.toolchain.detect(false);
			notice = pathOverride.trim()
				? 'PATH override saved and toolchain rescanned.'
				: 'PATH override cleared. Nodesmith is using the resolved login-shell PATH.';
		} catch (caught) {
			error = toErrorMessage(caught);
		} finally {
			saving = false;
		}
	}

	onMount(() => {
		void load();
	});
</script>

<svelte:head>
	<title>Toolchain doctor · Nodesmith</title>
</svelte:head>

<header
	class="flex flex-col gap-5 border-b border-line pb-6 lg:flex-row lg:items-end lg:justify-between"
>
	<div>
		<div
			class="flex items-center gap-2 text-xs font-bold tracking-[0.14em] text-brand-strong uppercase"
		>
			<span class="h-px w-7 bg-brand/60"></span>
			System doctor
		</div>
		<h1 class="mt-3 text-3xl font-bold tracking-[-0.04em] text-ink">Local toolchain</h1>
		<p class="mt-2 max-w-2xl text-sm leading-6 text-ink-muted">
			Nodesmith scans the exact PATH used by the desktop runner and checks the tools required by
			each recipe.
		</p>
	</div>
	<Button variant="secondary" onclick={() => load(true)} loading={rescanning}>
		<Icon name="refresh" class="size-4" />
		Rescan tools
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
	<div class="mt-6 grid gap-4 sm:grid-cols-2 xl:grid-cols-3" aria-busy="true">
		{#each toolSkeletons as index (index)}
			<div class="h-36 animate-pulse rounded-panel border border-line bg-panel/70"></div>
		{/each}
	</div>
{:else if !toolchain}
	<div class="mt-6">
		<EmptyState
			title="Toolchain scan unavailable"
			description={error || 'Run the toolchain scan from the Wails v2 desktop application.'}
		>
			{#snippet action()}<Button variant="secondary" onclick={() => load()}>Try again</Button
				>{/snippet}
		</EmptyState>
	</div>
{:else}
	<section class="mt-6 grid gap-4 sm:grid-cols-2 xl:grid-cols-3" aria-label="Detected tools">
		{#each toolchain.tools as tool (tool.name)}
			<article class="rounded-panel border border-line bg-panel/75 p-4 shadow-sm">
				<div class="flex items-start justify-between gap-3">
					<div class="flex min-w-0 items-center gap-3">
						<span
							class={[
								'flex size-9 shrink-0 items-center justify-center rounded-xl border',
								tool.present
									? 'border-success/25 bg-success-soft text-success'
									: 'border-warning/30 bg-warning-soft text-warning'
							]}
						>
							<Icon name={tool.present ? 'check' : 'triangleAlert'} class="size-4" />
						</span>
						<div class="min-w-0">
							<h2 class="font-mono text-sm font-bold text-ink">{toolLabel(tool)}</h2>
							<p class="mt-0.5 truncate text-xs text-ink-faint">
								{tool.present ? tool.version || 'Detected' : 'Not found'}
							</p>
						</div>
					</div>
					<Badge tone={tool.present ? 'success' : 'warning'} size="sm">
						{tool.present ? 'Ready' : 'Missing'}
					</Badge>
				</div>
				{#if tool.present}
					<p class="mt-4 truncate font-mono text-[0.6875rem] text-ink-faint" title={tool.path}>
						{tool.path}
					</p>
				{:else}
					<p class="mt-4 text-xs leading-5 text-ink-muted">
						{tool.error || installGuidance[tool.name] || `Install ${tool.name} and add it to PATH.`}
					</p>
				{/if}
			</article>
		{/each}
	</section>

	<section class="mt-6 rounded-panel border border-line bg-panel/75 p-5 shadow-sm sm:p-6">
		<div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
			<div>
				<p class="text-xs font-bold tracking-[0.12em] text-brand-strong uppercase">
					PATH resolution
				</p>
				<h2 class="mt-1 text-lg font-bold text-ink">Executable search path</h2>
				<p class="mt-1 max-w-2xl text-sm leading-6 text-ink-muted">
					Leave the override empty to use Nodesmith’s login-shell discovery. Use an override only
					when GUI launches cannot see your version manager.
				</p>
			</div>
			<div class="flex shrink-0 gap-2">
				<Badge tone="success">{presentCount} found</Badge>
				{#if missingCount > 0}<Badge tone="warning">{missingCount} missing</Badge>{/if}
			</div>
		</div>

		<div class="mt-5 grid gap-5">
			<div>
				<label for="resolved-path" class="text-xs font-semibold text-ink">Resolved PATH</label>
				<textarea
					id="resolved-path"
					readonly
					value={toolchain.path}
					rows="3"
					class="mt-2 w-full resize-none rounded-control border border-line bg-canvas/65 px-3 py-2.5 font-mono text-xs leading-5 text-ink-muted outline-none focus:border-brand focus:ring-3 focus:ring-brand/20"
				></textarea>
			</div>
			<div>
				<label for="path-override" class="text-xs font-semibold text-ink">Manual override</label>
				<textarea
					id="path-override"
					bind:value={pathOverride}
					rows="3"
					spellcheck="false"
					placeholder="/custom/bin:/usr/local/bin:/usr/bin"
					class="mt-2 w-full resize-none rounded-control border border-line-strong bg-panel-raised px-3 py-2.5 font-mono text-xs leading-5 text-ink transition-[border-color,box-shadow] outline-none placeholder:text-ink-faint focus:border-brand focus:ring-3 focus:ring-brand/20"
				></textarea>
			</div>
			<div class="flex justify-end">
				<Button onclick={savePathOverride} loading={saving}>Save PATH and rescan</Button>
			</div>
		</div>
	</section>
{/if}
