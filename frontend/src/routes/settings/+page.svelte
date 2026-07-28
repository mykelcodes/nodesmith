<script lang="ts">
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import { api, toErrorMessage, type Settings } from '$lib/api';
	import Icon from '$lib/components/icons/Icon.svelte';
	import { Button, EmptyState, Field, Select, TextInput, Toggle } from '$lib/components/ui';
	import {
		builtInEditorOptions,
		CUSTOM_EDITOR_OPTION,
		isBuiltInEditor,
		validateCustomEditorPath
	} from '$lib/settings/editor';
	import { applyTheme } from '$lib/theme/theme';

	const editorOptions = [
		...builtInEditorOptions,
		{ value: CUSTOM_EDITOR_OPTION, label: 'Custom executable…' }
	] as const;
	const themeOptions = [
		{ value: 'dark', label: 'Dark' },
		{ value: 'light', label: 'Light' },
		{ value: 'system', label: 'Follow system' }
	] as const;

	let settings = $state<Settings | null>(null);
	let loading = $state(true);
	let saving = $state(false);
	let pickingDirectory = $state(false);
	let error = $state('');
	let notice = $state('');
	let editorChoice = $state<string>('code');
	let customEditorPath = $state('');

	const customEditorError = $derived(
		editorChoice === CUSTOM_EDITOR_OPTION ? validateCustomEditorPath(customEditorPath) : ''
	);

	function update<Key extends keyof Settings>(key: Key, value: Settings[Key]) {
		if (!settings) return;
		settings = { ...settings, [key]: value };
		if (key === 'theme') applyTheme(value as Settings['theme']);
	}

	function setEditorChoice(value: string) {
		editorChoice = value;
		update('editor', value === CUSTOM_EDITOR_OPTION ? customEditorPath : value);
	}

	function setCustomEditorPath(value: string) {
		customEditorPath = value;
		update('editor', value);
	}

	async function load() {
		loading = true;
		error = '';
		try {
			settings = await api.store.getSettings();
			editorChoice = isBuiltInEditor(settings.editor) ? settings.editor : CUSTOM_EDITOR_OPTION;
			customEditorPath = isBuiltInEditor(settings.editor) ? '' : settings.editor;
			applyTheme(settings.theme, true);
		} catch (caught) {
			error = toErrorMessage(caught);
		} finally {
			loading = false;
		}
	}

	async function pickDefaultDirectory() {
		if (!settings) return;
		pickingDirectory = true;
		error = '';
		try {
			const selected = await api.scaffold.pickDirectory(settings.defaultParentDir);
			if (selected) update('defaultParentDir', selected);
		} catch (caught) {
			error = toErrorMessage(caught);
		} finally {
			pickingDirectory = false;
		}
	}

	async function save() {
		if (!settings) return;
		error = '';
		notice = '';
		if (customEditorError) {
			error = customEditorError;
			return;
		}
		saving = true;
		try {
			await api.store.saveSettings(settings);
			applyTheme(settings.theme, true);
			if (settings.pathOverride.trim()) {
				await api.toolchain.setPathOverride(settings.pathOverride);
			}
			notice = 'Settings saved.';
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
	<title>Settings · Nodesmith</title>
</svelte:head>

<header class="border-b border-line pb-6">
	<div
		class="flex items-center gap-2 text-xs font-bold tracking-[0.14em] text-brand-strong uppercase"
	>
		<span class="h-px w-7 bg-brand/60"></span>
		Preferences
	</div>
	<h1 class="mt-3 text-3xl font-bold tracking-[-0.04em] text-ink">Settings</h1>
	<p class="mt-2 max-w-2xl text-sm leading-6 text-ink-muted">
		Choose the defaults Nodesmith uses for new projects. Everything is stored locally on this
		machine.
	</p>
</header>

{#if loading}
	<div class="mt-6 grid gap-5 lg:grid-cols-2" aria-busy="true">
		<div class="h-80 animate-pulse rounded-panel border border-line bg-panel/70"></div>
		<div class="h-80 animate-pulse rounded-panel border border-line bg-panel/70"></div>
	</div>
{:else if !settings}
	<div class="mt-6">
		<EmptyState title="Settings unavailable" description={error}>
			{#snippet action()}<Button variant="secondary" onclick={load}>Try again</Button>{/snippet}
		</EmptyState>
	</div>
{:else}
	{@const activeSettings = settings}
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

	<form
		class="mt-6 grid gap-5 lg:grid-cols-2"
		onsubmit={(event) => {
			event.preventDefault();
			void save();
		}}
	>
		<section class="rounded-panel border border-line bg-panel/75 p-5 shadow-sm sm:p-6">
			<p class="text-xs font-bold tracking-[0.12em] text-brand-strong uppercase">
				Project defaults
			</p>
			<h2 class="mt-1 text-lg font-bold text-ink">New project behaviour</h2>
			<div class="mt-6 grid gap-6">
				<Field
					label="Default parent directory"
					help="Pre-filled when you start configuring a recipe."
					required
				>
					{#snippet children(context)}
						<div class="flex gap-2">
							<TextInput
								id={context.controlId}
								value={activeSettings.defaultParentDir}
								aria-describedby={context.describedBy}
								inputClass="font-mono"
								required
								aria-required="true"
								oninput={(event) => update('defaultParentDir', event.currentTarget.value)}
							/>
							<Button
								variant="secondary"
								onclick={pickDefaultDirectory}
								loading={pickingDirectory}
								aria-label="Choose default parent directory"
							>
								<Icon name="folder" class="size-4" />
								Choose
							</Button>
						</div>
					{/snippet}
				</Field>

				<Field label="Preferred editor" help="Used by result and history actions.">
					{#snippet children(context)}
						<Select
							id={context.controlId}
							value={editorChoice}
							options={editorOptions}
							aria-describedby={context.describedBy}
							onchange={(event) => setEditorChoice(event.currentTarget.value)}
						/>
					{/snippet}
				</Field>

				{#if editorChoice === CUSTOM_EDITOR_OPTION}
					<Field
						label="Custom executable path"
						help="Enter one executable only. Paths with spaces are supported; flags and shell commands are not."
						error={customEditorError}
						required
					>
						{#snippet children(context)}
							<TextInput
								id={context.controlId}
								value={customEditorPath}
								placeholder="/usr/local/bin/editor"
								autocomplete="off"
								spellcheck="false"
								inputClass="font-mono"
								invalid={Boolean(customEditorError)}
								aria-describedby={context.describedBy}
								required
								aria-required="true"
								oninput={(event) => setCustomEditorPath(event.currentTarget.value)}
							/>
						{/snippet}
					</Field>
				{/if}

				<Toggle
					checked={activeSettings.openAfterCreate}
					label="Open successful projects automatically"
					description="Launch the preferred editor when the result screen confirms success."
					onchange={(event) => update('openAfterCreate', event.currentTarget.checked)}
				/>
			</div>
		</section>

		<section class="rounded-panel border border-line bg-panel/75 p-5 shadow-sm sm:p-6">
			<p class="text-xs font-bold tracking-[0.12em] text-brand-strong uppercase">
				Appearance & environment
			</p>
			<h2 class="mt-1 text-lg font-bold text-ink">Desktop experience</h2>
			<div class="mt-6 grid gap-6">
				<Field label="Theme" help="System mode follows the operating system appearance.">
					{#snippet children(context)}
						<Select
							id={context.controlId}
							value={activeSettings.theme}
							options={themeOptions}
							aria-describedby={context.describedBy}
							onchange={(event) => update('theme', event.currentTarget.value as Settings['theme'])}
						/>
					{/snippet}
				</Field>

				<div class="rounded-control border border-line bg-panel-raised/65 p-4">
					<div class="flex items-start gap-3">
						<Icon name="activity" class="mt-0.5 size-4 shrink-0 text-brand-strong" />
						<div>
							<p class="text-sm font-semibold text-ink">Executable PATH</p>
							<p class="mt-1 text-xs leading-5 text-ink-muted">
								PATH discovery and manual overrides live in the toolchain doctor so changes can be
								rescanned immediately.
							</p>
							<a
								href={resolve('/toolchain')}
								class="mt-3 inline-flex rounded-md text-xs font-semibold text-brand-strong underline decoration-brand/35 underline-offset-4 focus-visible:ring-3 focus-visible:ring-brand/25 focus-visible:outline-none"
							>
								Open toolchain doctor
							</a>
						</div>
					</div>
				</div>

				<div class="rounded-control border border-line bg-canvas/45 p-4">
					<p class="text-xs font-semibold text-ink">Privacy</p>
					<p class="mt-1 text-xs leading-5 text-ink-muted">
						Nodesmith has no account and sends no telemetry. Recipe commands and project history
						stay on this machine.
					</p>
				</div>
			</div>
		</section>

		<footer class="flex justify-end lg:col-span-2">
			<Button type="submit" loading={saving}>
				<Icon name="check" class="size-4" />
				Save settings
			</Button>
		</footer>
	</form>
{/if}
