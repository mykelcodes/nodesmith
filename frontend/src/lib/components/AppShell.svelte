<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { RouteId } from '$app/types';
	import { onMount, type Snippet } from 'svelte';
	import {
		WindowFullscreen,
		WindowIsFullscreen,
		WindowUnfullscreen
	} from '$lib/wailsjs/runtime/runtime.js';
	import Icon from './icons/Icon.svelte';
	import type { IconName } from './icons/icon-data';
	import Button from './ui/Button.svelte';

	interface NavItem {
		label: string;
		href: RouteId;
		icon: IconName;
		exact?: boolean;
	}

	interface NavSection {
		label: string;
		items: readonly NavItem[];
	}

	interface Props {
		children: Snippet;
	}

	type FullscreenRuntimeWindow = Window & {
		runtime?: {
			WindowFullscreen?: unknown;
			WindowIsFullscreen?: unknown;
			WindowUnfullscreen?: unknown;
		};
	};

	const sections: readonly NavSection[] = [
		{
			label: 'Workspace',
			items: [
				{ label: 'Catalogue', href: '/', icon: 'grid', exact: true },
				{ label: 'Presets', href: '/presets', icon: 'bookmark' },
				{ label: 'History', href: '/history', icon: 'history' }
			]
		},
		{
			label: 'System',
			items: [
				{ label: 'Toolchain', href: '/toolchain', icon: 'activity' },
				{ label: 'Settings', href: '/settings', icon: 'settings' }
			]
		}
	];

	let { children }: Props = $props();
	let fullscreen = $state(false);
	let fullscreenAvailable = $state(true);

	function hasNativeFullscreen(): boolean {
		const runtime = (window as FullscreenRuntimeWindow).runtime;
		return (
			typeof runtime?.WindowFullscreen === 'function' &&
			typeof runtime.WindowIsFullscreen === 'function' &&
			typeof runtime.WindowUnfullscreen === 'function'
		);
	}

	async function syncFullscreenState() {
		if (hasNativeFullscreen()) {
			try {
				fullscreen = await WindowIsFullscreen();
			} catch {
				fullscreenAvailable = false;
			}
			return;
		}
		fullscreen = Boolean(document.fullscreenElement);
	}

	async function toggleFullscreen() {
		if (hasNativeFullscreen()) {
			const active = await WindowIsFullscreen();
			if (active) WindowUnfullscreen();
			else WindowFullscreen();
			fullscreen = !active;
			return;
		}

		if (document.fullscreenElement) {
			await document.exitFullscreen();
		} else {
			await document.documentElement.requestFullscreen();
		}
		fullscreen = Boolean(document.fullscreenElement);
	}

	function isActive(item: NavItem) {
		const routeId = page.route.id;
		if (!routeId) return false;
		if (item.exact) return routeId === item.href;
		return routeId === item.href || routeId.startsWith(`${item.href}/`);
	}

	onMount(() => {
		fullscreenAvailable = hasNativeFullscreen() || document.fullscreenEnabled;
		const sync = () => void syncFullscreenState();
		const handleKeydown = (event: KeyboardEvent) => {
			const shortcut =
				event.key === 'F11' || (event.key.toLowerCase() === 'f' && event.ctrlKey && event.metaKey);
			if (!shortcut || !fullscreenAvailable) return;
			event.preventDefault();
			void toggleFullscreen();
		};

		window.addEventListener('focus', sync);
		window.addEventListener('resize', sync);
		window.addEventListener('keydown', handleKeydown);
		document.addEventListener('fullscreenchange', sync);
		void syncFullscreenState();

		return () => {
			window.removeEventListener('focus', sync);
			window.removeEventListener('resize', sync);
			window.removeEventListener('keydown', handleKeydown);
			document.removeEventListener('fullscreenchange', sync);
		};
	});
</script>

<div class="relative h-dvh min-h-[34rem] overflow-hidden bg-canvas text-ink">
	<div
		class="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_20%_-10%,color-mix(in_srgb,var(--ns-brand)_14%,transparent),transparent_34rem)]"
		aria-hidden="true"
	></div>
	<a
		href={resolve(`${page.route.id ?? '/'}#main-content`)}
		class="fixed top-3 left-3 z-50 -translate-y-20 rounded-control bg-brand px-3 py-2 text-sm font-semibold text-white shadow-float transition-transform focus:translate-y-0 focus:outline-none"
	>
		Skip to content
	</a>

	<div class="relative grid h-full grid-rows-[3.5rem_minmax(0,1fr)]">
		<header
			class="flex items-center justify-between border-b border-line bg-panel/80 px-4 backdrop-blur-xl"
			data-wails-drag
		>
			<a
				href={resolve('/')}
				class="flex items-center gap-2.5 rounded-lg focus-visible:ring-3 focus-visible:ring-brand/25 focus-visible:outline-none"
				data-wails-no-drag
				aria-label="Nodesmith catalogue"
			>
				<span
					class="flex size-8 items-center justify-center rounded-[0.65rem] border border-brand/35 bg-brand-soft text-brand-strong shadow-sm"
				>
					<Icon name="logo" class="size-5" strokeWidth={1.7} />
				</span>
				<span class="text-[0.9375rem] font-bold tracking-[-0.02em] text-ink">Nodesmith</span>
				<span
					class="hidden rounded-md border border-line bg-panel-raised px-1.5 py-0.5 text-[0.625rem] font-bold tracking-[0.08em] text-ink-faint uppercase sm:inline-flex"
					>Desktop</span
				>
			</a>

			<div class="flex items-center gap-2 text-xs font-medium text-ink-faint" data-wails-no-drag>
				<Button
					variant="ghost"
					size="icon"
					class="size-8 rounded-lg"
					onclick={toggleFullscreen}
					disabled={!fullscreenAvailable}
					aria-label={fullscreen ? 'Exit full screen' : 'Enter full screen'}
					title={fullscreen ? 'Exit full screen (F11)' : 'Enter full screen (F11)'}
				>
					<Icon
						name={fullscreen ? 'fullscreenExit' : 'fullscreen'}
						class="size-4"
						strokeWidth={1.7}
					/>
				</Button>
				<span class="mx-0.5 h-4 w-px bg-line" aria-hidden="true"></span>
				<span class="size-1.5 rounded-full bg-success shadow-[0_0_0_3px_var(--ns-success-soft)]"
				></span>
				<span class="hidden sm:inline">Local only</span>
			</div>
		</header>

		<div class="grid min-h-0 grid-cols-[4.25rem_minmax(0,1fr)] md:grid-cols-[15rem_minmax(0,1fr)]">
			<aside
				class="flex min-h-0 flex-col border-r border-line bg-panel/65 px-2 py-4 backdrop-blur-xl md:px-3"
			>
				<nav class="space-y-5" aria-label="Primary navigation">
					{#each sections as section (section.label)}
						<div>
							<p
								class="mb-1.5 hidden px-2.5 text-[0.625rem] font-bold tracking-[0.14em] text-ink-faint uppercase md:block"
							>
								{section.label}
							</p>
							<ul class="space-y-1">
								{#each section.items as item (item.href)}
									<li>
										<a
											href={resolve(item.href)}
											aria-current={isActive(item) ? 'page' : undefined}
											class={`group relative flex h-10 items-center justify-center gap-3 rounded-control border text-sm font-semibold transition-[color,background-color,border-color] duration-150 focus-visible:ring-3 focus-visible:ring-brand/25 focus-visible:outline-none md:justify-start md:px-2.5 ${
												isActive(item)
													? 'border-brand/20 bg-brand-soft text-brand-strong'
													: 'border-transparent text-ink-muted hover:bg-overlay hover:text-ink'
											}`}
											title={item.label}
										>
											<Icon
												name={item.icon}
												class={`size-[1.125rem] shrink-0 ${isActive(item) ? 'text-brand-strong' : 'text-ink-faint group-hover:text-ink-muted'}`}
											/>
											<span class="hidden truncate md:block">{item.label}</span>
											{#if isActive(item)}
												<span
													class="absolute top-2 bottom-2 left-0 w-0.5 rounded-full bg-brand md:-left-0.5"
													aria-hidden="true"
												></span>
											{/if}
										</a>
									</li>
								{/each}
							</ul>
						</div>
					{/each}
				</nav>

				<div
					class="mt-auto hidden rounded-control border border-line bg-panel-raised/70 p-3 md:block"
				>
					<div class="flex items-center gap-2 text-xs font-semibold text-ink-muted">
						<Icon name="terminal" class="size-3.5 text-brand-strong" />
						Transparent by default
					</div>
					<p class="mt-1.5 text-[0.6875rem] leading-4 text-ink-faint">
						Every command is shown before it runs.
					</p>
				</div>
			</aside>

			<main id="main-content" class="min-w-0 overflow-y-auto overscroll-contain">
				<div class="mx-auto w-full max-w-[94rem] px-5 py-6 sm:px-7 sm:py-8 lg:px-10">
					{@render children()}
				</div>
			</main>
		</div>
	</div>
</div>
