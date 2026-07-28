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

	it('turns unknown rejection values into useful display text', () => {
		expect(toErrorMessage(new Error('pnpm is missing'))).toBe('pnpm is missing');
		expect(toErrorMessage(null)).toBe('An unexpected error occurred. Please try again.');
	});
});
