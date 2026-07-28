const PORTABLE_NAME_MAX_BYTES = 255;
const WINDOWS_INVALID_CHARACTERS = /[<>:"|?*]/;
const WINDOWS_RESERVED_BASE_NAMES = new Set([
	'CON',
	'PRN',
	'AUX',
	'NUL',
	'CLOCK$',
	'CONIN$',
	'CONOUT$',
	'COM¹',
	'COM²',
	'COM³',
	'LPT¹',
	'LPT²',
	'LPT³'
]);

function isWindowsReservedName(name: string): boolean {
	const base = name
		.split('.', 1)[0]
		.replace(/[ .]+$/u, '')
		.toUpperCase();
	if (WINDOWS_RESERVED_BASE_NAMES.has(base)) return true;
	return /^(?:COM|LPT)[1-9]$/.test(base);
}

export function validatePortableProjectName(name: string): string {
	if (name === '') return 'Project name is required.';
	if (name !== name.trim()) return 'Project name cannot start or end with whitespace.';
	if (name === '.' || name === '..') return 'Project name cannot be “.” or “..”.';
	if (name.startsWith('.')) return 'Project name cannot start with a dot.';
	if (/[\\/]/.test(name)) return 'Project name cannot contain path separators.';
	if (WINDOWS_INVALID_CHARACTERS.test(name)) {
		return 'Project name contains a character that is not portable across operating systems.';
	}
	for (const character of name) {
		const codePoint = character.codePointAt(0) ?? 0;
		if (codePoint === 0 || codePoint < 0x20 || codePoint === 0x7f) {
			return 'Project name cannot contain control characters.';
		}
	}
	if (name.endsWith('.')) return 'Project name cannot end with a dot.';
	if (new TextEncoder().encode(name).byteLength > PORTABLE_NAME_MAX_BYTES) {
		return `Project name must be ${PORTABLE_NAME_MAX_BYTES} UTF-8 bytes or fewer.`;
	}
	if (isWindowsReservedName(name)) {
		return 'Project name is reserved by Windows; choose a different name.';
	}
	return '';
}
