import { describe, expect, it } from 'vitest';
import {
	describeReleaseAge,
	formatReleaseAge,
	MAX_RELEASE_AGE_MINUTES,
	resolveReleaseAge,
	validateReleaseAgeInput
} from './releaseAge';

describe('minimum release age resolution', () => {
	it('reports nothing configured when no layer sets a value', () => {
		expect(resolveReleaseAge(null, null, null)).toEqual({ minutes: null, source: 'unset' });
	});

	it('falls back to the global default when the recipe is silent', () => {
		expect(resolveReleaseAge(null, null, 1440)).toEqual({ minutes: 1440, source: 'global' });
	});

	it('prefers the recipe value over the global default', () => {
		expect(resolveReleaseAge(null, 4320, 1440)).toEqual({ minutes: 4320, source: 'recipe' });
	});

	it('prefers the configured request over both the recipe and the global default', () => {
		expect(resolveReleaseAge(10080, 4320, 1440)).toEqual({
			minutes: 10080,
			source: 'request'
		});
	});

	it('treats an explicit zero as a value rather than a missing one', () => {
		expect(resolveReleaseAge(0, 4320, 1440)).toEqual({ minutes: 0, source: 'request' });
		expect(resolveReleaseAge(null, 0, 1440)).toEqual({ minutes: 0, source: 'recipe' });
	});
});

describe('minimum release age formatting', () => {
	it('renders the largest unit that divides the value exactly', () => {
		expect(formatReleaseAge(null)).toBe('Not set');
		expect(formatReleaseAge(0)).toBe('No cooldown');
		expect(formatReleaseAge(1)).toBe('1 minute');
		expect(formatReleaseAge(90)).toBe('90 minutes');
		expect(formatReleaseAge(60)).toBe('1 hour');
		expect(formatReleaseAge(720)).toBe('12 hours');
		expect(formatReleaseAge(1440)).toBe('1 day');
		expect(formatReleaseAge(4320)).toBe('3 days');
	});

	it('names the layer an effective value came from', () => {
		expect(describeReleaseAge({ minutes: 4320, source: 'recipe' })).toBe(
			'3 days, from the recipe.'
		);
		expect(describeReleaseAge({ minutes: null, source: 'unset' })).toMatch(/own configuration/);
	});
});

describe('minimum release age input validation', () => {
	it('accepts whole minute counts inside the supported range', () => {
		expect(validateReleaseAgeInput('0')).toBe('');
		expect(validateReleaseAgeInput('1440')).toBe('');
		expect(validateReleaseAgeInput(` ${MAX_RELEASE_AGE_MINUTES} `)).toBe('');
	});

	it('rejects empty, fractional, negative, and out-of-range values', () => {
		expect(validateReleaseAgeInput('')).toMatch(/number of minutes/i);
		expect(validateReleaseAgeInput('1.5')).toMatch(/whole number/i);
		expect(validateReleaseAgeInput('-1')).toMatch(/whole number/i);
		expect(validateReleaseAgeInput(String(MAX_RELEASE_AGE_MINUTES + 1))).toMatch(/one year/i);
	});
});
