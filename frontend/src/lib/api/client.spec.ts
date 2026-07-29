import { describe, expect, it } from 'vitest';
import { api, toErrorMessage } from '.';
import { parseLogLine, parseRecipeSummary, parseSettings } from './parse';

describe('Wails v2 API boundary', () => {
	it('provides an actionable error outside the desktop runtime', async () => {
		await expect(api.recipes.list()).rejects.toThrow(
			/Wails v2 desktop bridge is unavailable outside the desktop application/
		);
	});

	it('rejects malformed binding results at the boundary', () => {
		expect(() =>
			parseLogLine({ seq: 1, stream: 'console', text: 'hello', stepId: 'create' })
		).toThrow(/Invalid Nodesmith bridge response at LogLine\.stream/);
	});

	it('validates generator install policy at the bridge boundary', () => {
		const summary = {
			id: 'hono',
			name: 'Hono',
			category: 'backend',
			description: 'A web framework.',
			docsUrl: 'https://hono.dev',
			tags: ['backend'],
			icon: 'hono',
			verifiedAt: '2026-07-28',
			installPolicy: 'required',
			available: true,
			unavailableReasons: [],
			defaultPackageManager: 'pnpm'
		};

		expect(parseRecipeSummary(summary).installPolicy).toBe('required');
		expect(() => parseRecipeSummary({ ...summary, installPolicy: 'sometimes' })).toThrow(
			/RecipeSummary\.installPolicy/
		);
	});

	it('preserves a custom editor executable path from settings', () => {
		const editor = '/Applications/My Editor.app/Contents/MacOS/Editor';
		const settings = parseSettings({
			defaultParentDir: '/projects',
			pathOverride: '',
			editor,
			theme: 'system',
			openAfterCreate: true
		});

		expect(settings.editor).toBe(editor);
	});

	it('saves a new preset with the Go zero timestamp instead of an empty string', async () => {
		let sent: { createdAt?: unknown; updatedAt?: unknown } | undefined;
		const stub = globalThis as { window?: unknown };
		stub.window = {
			go: {
				services: {
					StoreService: {
						SavePreset: (preset: { createdAt?: unknown; updatedAt?: unknown }) => {
							sent = preset;
							return Promise.resolve();
						}
					}
				}
			}
		};

		try {
			await api.store.savePreset({
				id: '',
				name: 'Hono API',
				request: {
					recipeId: 'hono',
					projectName: 'api',
					parentDir: '/projects',
					packageManager: 'pnpm',
					installDeps: true,
					gitInit: true,
					minimumReleaseAge: 4320,
					answers: {}
				},
				createdAt: '',
				updatedAt: ''
			});
		} finally {
			delete stub.window;
		}

		expect(sent).toMatchObject({
			request: { minimumReleaseAge: 4320 },
			createdAt: '0001-01-01T00:00:00Z',
			updatedAt: '0001-01-01T00:00:00Z'
		});
	});

	it('turns unknown rejection values into useful display text', () => {
		expect(toErrorMessage(new Error('pnpm is missing'))).toBe('pnpm is missing');
		expect(toErrorMessage(null)).toBe('An unexpected error occurred. Please try again.');
	});
});
