import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const tokenSource = readFileSync(new URL('./tokens.css', import.meta.url), 'utf8');
const lightThemeSource = tokenSource.match(/\[data-theme='light'\]\s*\{(?<body>[\s\S]*?)\n\}/)
	?.groups?.body;

function token(name: string): string {
	const value = lightThemeSource?.match(new RegExp(`--ns-${name}:\\s*(#[0-9a-f]{6})`, 'i'))?.[1];
	if (!value) throw new Error(`Missing light theme token --ns-${name}`);
	return value;
}

function luminance(hex: string): number {
	const channels = [1, 3, 5].map(
		(offset) => Number.parseInt(hex.slice(offset, offset + 2), 16) / 255
	);
	const linear = channels.map((channel) =>
		channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4
	);
	return 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2];
}

function contrast(foreground: string, background: string): number {
	const values = [luminance(foreground), luminance(background)].sort((left, right) => right - left);
	return (values[0] + 0.05) / (values[1] + 0.05);
}

describe('light theme normal-text contrast', () => {
	it.each([
		['ink', 'panel'],
		['ink-muted', 'panel'],
		['ink-faint', 'panel-raised'],
		['brand-strong', 'brand-soft'],
		['success', 'success-soft'],
		['warning', 'warning-soft'],
		['danger', 'danger-soft'],
		['info', 'info-soft']
	])('%s on %s meets WCAG AA', (foreground, background) => {
		expect(contrast(token(foreground), token(background))).toBeGreaterThanOrEqual(4.5);
	});

	it('keeps white button text legible on the brand fill', () => {
		expect(contrast('#ffffff', token('brand'))).toBeGreaterThanOrEqual(4.5);
	});
});
