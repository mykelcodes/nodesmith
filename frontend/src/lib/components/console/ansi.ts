export interface AnsiToken {
	text: string;
	classes: string;
}

interface AnsiState {
	bold: boolean;
	dim: boolean;
	foreground: string;
}

const foregroundClasses: Record<number, string> = {
	30: 'text-ink-faint',
	31: 'text-danger',
	32: 'text-success',
	33: 'text-warning',
	34: 'text-info',
	35: 'text-brand-strong',
	36: 'text-cyan-300',
	37: 'text-ink-muted',
	90: 'text-ink-faint',
	91: 'text-red-300',
	92: 'text-emerald-300',
	93: 'text-amber-300',
	94: 'text-blue-300',
	95: 'text-violet-300',
	96: 'text-cyan-200',
	97: 'text-ink'
};

function classesFor(state: AnsiState): string {
	return [state.foreground, state.bold ? 'font-semibold' : '', state.dim ? 'opacity-60' : '']
		.filter(Boolean)
		.join(' ');
}

function applyCode(state: AnsiState, code: number) {
	if (code === 0) {
		state.bold = false;
		state.dim = false;
		state.foreground = '';
		return;
	}
	if (code === 1) state.bold = true;
	if (code === 2) state.dim = true;
	if (code === 22) {
		state.bold = false;
		state.dim = false;
	}
	if (code === 39) state.foreground = '';
	if (foregroundClasses[code]) state.foreground = foregroundClasses[code];
}

export function tokenizeAnsi(input: string): AnsiToken[] {
	const tokens: AnsiToken[] = [];
	const state: AnsiState = { bold: false, dim: false, foreground: '' };
	const expression = new RegExp(`${String.fromCharCode(27)}\\[([0-9;]*)m`, 'g');
	let cursor = 0;

	for (const match of input.matchAll(expression)) {
		const index = match.index;
		if (index > cursor) {
			tokens.push({ text: input.slice(cursor, index), classes: classesFor(state) });
		}
		const codes = match[1] === '' ? [0] : match[1].split(';').map(Number);
		for (const code of codes) applyCode(state, Number.isFinite(code) ? code : 0);
		cursor = index + match[0].length;
	}

	if (cursor < input.length) {
		tokens.push({ text: input.slice(cursor), classes: classesFor(state) });
	}

	return tokens.length > 0 ? tokens : [{ text: '', classes: '' }];
}
