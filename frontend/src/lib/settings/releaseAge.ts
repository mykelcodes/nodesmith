/** One year, matching recipe.MaxMinimumReleaseAge in the Go backend. */
export const MAX_RELEASE_AGE_MINUTES = 525600;

const MINUTES_PER_HOUR = 60;
const MINUTES_PER_DAY = 1440;

/**
 * Which layer supplied an effective cooldown, ordered most specific first.
 */
export type ReleaseAgeSource = 'request' | 'recipe' | 'global' | 'unset';

export interface ResolvedReleaseAge {
	minutes: number | null;
	source: ReleaseAgeSource;
}

/**
 * Picks the cooldown that applies to one catalogue configuration. The backend
 * performs the same resolution when it builds a plan; this copy exists so the
 * interface can explain an inherited value without a round trip.
 *
 * An explicit zero counts as a value, so a recipe or override can switch a
 * cooldown off rather than inheriting a longer one.
 */
export function resolveReleaseAge(
	requestMinutes: number | null,
	recipeMinutes: number | null,
	globalMinutes: number | null
): ResolvedReleaseAge {
	if (requestMinutes !== null) return { minutes: requestMinutes, source: 'request' };
	if (recipeMinutes !== null) return { minutes: recipeMinutes, source: 'recipe' };
	if (globalMinutes !== null) return { minutes: globalMinutes, source: 'global' };
	return { minutes: null, source: 'unset' };
}

function plural(count: number, unit: string): string {
	return `${count} ${unit}${count === 1 ? '' : 's'}`;
}

/** Renders a cooldown as the largest whole unit that divides it exactly. */
export function formatReleaseAge(minutes: number | null): string {
	if (minutes === null) return 'Not set';
	if (minutes === 0) return 'No cooldown';
	if (minutes % MINUTES_PER_DAY === 0) return plural(minutes / MINUTES_PER_DAY, 'day');
	if (minutes % MINUTES_PER_HOUR === 0) return plural(minutes / MINUTES_PER_HOUR, 'hour');
	return plural(minutes, 'minute');
}

const sourceLabels: Record<ReleaseAgeSource, string> = {
	request: 'this configuration',
	recipe: 'the recipe',
	global: 'the global default',
	unset: 'nothing'
};

/** Explains an inherited value, e.g. "3 days, from the recipe". */
export function describeReleaseAge(resolved: ResolvedReleaseAge): string {
	if (resolved.source === 'unset') {
		return 'Not set — each package manager uses its own configuration.';
	}
	return `${formatReleaseAge(resolved.minutes)}, from ${sourceLabels[resolved.source]}.`;
}

/**
 * Validates a minutes value typed into the interface. Returns an empty string
 * when the value is usable, matching the editor-path validator's convention.
 */
export function validateReleaseAgeInput(value: string): string {
	const trimmed = value.trim();
	if (trimmed === '') return 'Enter a number of minutes.';
	if (!/^\d+$/.test(trimmed)) return 'Use a whole number of minutes, such as 1440 for one day.';
	const minutes = Number(trimmed);
	if (minutes > MAX_RELEASE_AGE_MINUTES) {
		return `Use at most ${MAX_RELEASE_AGE_MINUTES} minutes (one year).`;
	}
	return '';
}

/** Common cooldowns offered as a shortcut, in minutes. */
export const releaseAgePresets = [
	{ value: 0, label: 'No cooldown' },
	{ value: 60, label: '1 hour' },
	{ value: 720, label: '12 hours' },
	{ value: 1440, label: '1 day' },
	{ value: 4320, label: '3 days' },
	{ value: 10080, label: '7 days' },
	{ value: 43200, label: '30 days' }
] as const;
