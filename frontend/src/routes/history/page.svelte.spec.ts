import { page } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';

const apiMocks = vi.hoisted(() => ({
	listHistory: vi.fn(),
	getSettings: vi.fn()
}));

vi.mock('$lib/api', () => ({
	api: {
		store: {
			listHistory: apiMocks.listHistory,
			getSettings: apiMocks.getSettings
		}
	},
	toErrorMessage: (error: unknown) => (error instanceof Error ? error.message : String(error))
}));

import HistoryPage from './+page.svelte';

describe('history route loading', () => {
	beforeEach(() => {
		apiMocks.listHistory.mockReset();
		apiMocks.getSettings.mockReset();
	});

	it('renders a load error instead of the false empty-history state', async () => {
		apiMocks.listHistory.mockRejectedValue(new Error('history database is unavailable'));
		apiMocks.getSettings.mockResolvedValue({
			defaultParentDir: '/projects',
			pathOverride: '',
			editor: 'code',
			theme: 'dark',
			openAfterCreate: false
		});

		render(HistoryPage);

		await expect
			.element(page.getByRole('heading', { name: 'Project history couldn’t be loaded' }))
			.toBeInTheDocument();
		await expect.element(page.getByText('history database is unavailable')).toBeInTheDocument();
		await expect.element(page.getByText('No projects in history')).not.toBeInTheDocument();
	});
});
