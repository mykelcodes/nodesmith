export type ConditionLiteral = string | number | boolean;
export type ConditionOperator = '' | '==' | '!=' | 'includes';

export interface Condition {
	identifier: string;
	operator: ConditionOperator;
	literal?: ConditionLiteral;
	negated: boolean;
}

const identifierStart = /[A-Za-z_]/;
const identifierPart = /[A-Za-z0-9_-]/;

class Parser {
	private position = 0;

	constructor(private readonly source: string) {}

	parse(): Condition {
		this.skipSpace();
		let negated = false;
		if (this.peek('!') && !this.peek('!=')) {
			negated = true;
			this.position += 1;
			this.skipSpace();
		}

		const identifier = this.parseIdentifier();
		this.skipSpace();
		if (this.done()) return { identifier, operator: '', negated };
		if (negated) this.fail('negation only accepts an identifier');

		const operator = this.parseOperator();
		this.skipSpace();
		const literalText = this.source.slice(this.position).trim();
		if (literalText === '') this.fail('missing literal');
		const literal = this.parseLiteral(literalText);
		return { identifier, operator, literal, negated: false };
	}

	private parseIdentifier(): string {
		const first = this.source[this.position];
		if (first === undefined || !identifierStart.test(first)) this.fail('expected identifier');
		const start = this.position;
		this.position += 1;
		while (this.position < this.source.length && identifierPart.test(this.source[this.position])) {
			this.position += 1;
		}
		return this.source.slice(start, this.position);
	}

	private parseOperator(): Exclude<ConditionOperator, ''> {
		if (this.peek('==')) {
			this.position += 2;
			return '==';
		}
		if (this.peek('!=')) {
			this.position += 2;
			return '!=';
		}
		if (this.peek('includes')) {
			const after = this.source[this.position + 'includes'.length];
			if (after !== undefined && identifierPart.test(after)) {
				this.fail('expected ==, !=, or includes');
			}
			this.position += 'includes'.length;
			return 'includes';
		}
		return this.fail('expected ==, !=, or includes');
	}

	private parseLiteral(text: string): ConditionLiteral {
		let value: unknown;
		try {
			value = JSON.parse(text) as unknown;
		} catch {
			return this.fail('invalid literal');
		}
		if (typeof value !== 'string' && typeof value !== 'number' && typeof value !== 'boolean') {
			return this.fail('literal must be a quoted string, number, true, or false');
		}
		return value;
	}

	private skipSpace(): void {
		while (this.position < this.source.length && /\s/.test(this.source[this.position])) {
			this.position += 1;
		}
	}

	private peek(value: string): boolean {
		return this.source.startsWith(value, this.position);
	}

	private done(): boolean {
		return this.position === this.source.length;
	}

	private fail(message: string): never {
		throw new SyntaxError(`Invalid condition ${JSON.stringify(this.source)}: ${message}.`);
	}
}

export function parseCondition(expression: string): Condition {
	return new Parser(expression).parse();
}

function truthy(value: unknown): boolean {
	if (value === null || value === undefined) return false;
	if (typeof value === 'boolean') return value;
	if (typeof value === 'string') return value !== '';
	if (typeof value === 'number') return value !== 0 && !Number.isNaN(value);
	if (Array.isArray(value)) return value.length !== 0;
	return true;
}

function equal(left: unknown, right: ConditionLiteral | undefined): boolean {
	if (typeof left === 'number' || typeof right === 'number') {
		return typeof left === 'number' && typeof right === 'number' && left === right;
	}
	if (typeof left === 'string') return typeof right === 'string' && left === right;
	if (typeof left === 'boolean') return typeof right === 'boolean' && left === right;
	return false;
}

export function evaluateCondition(
	condition: Condition | string,
	values: Readonly<Record<string, unknown>>
): boolean {
	const parsed = typeof condition === 'string' ? parseCondition(condition) : condition;
	if (!Object.prototype.hasOwnProperty.call(values, parsed.identifier)) {
		throw new ReferenceError(`Unknown condition identifier ${JSON.stringify(parsed.identifier)}.`);
	}
	const value = values[parsed.identifier];
	switch (parsed.operator) {
		case '': {
			const result = truthy(value);
			return parsed.negated ? !result : result;
		}
		case '==':
			return equal(value, parsed.literal);
		case '!=':
			return !equal(value, parsed.literal);
		case 'includes':
			if (!Array.isArray(value)) {
				throw new TypeError(
					`Condition "includes" requires ${JSON.stringify(parsed.identifier)} to be a multiselect value.`
				);
			}
			return value.some((item) => equal(item, parsed.literal));
	}
}
