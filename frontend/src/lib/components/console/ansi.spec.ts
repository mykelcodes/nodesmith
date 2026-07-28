import { describe, expect, it } from 'vitest';
import { tokenizeAnsi } from './ansi';

describe('tokenizeAnsi', () => {
	it('turns supported SGR colours into inert text tokens', () => {
		expect(tokenizeAnsi('\u001b[31mfailed\u001b[0m safely')).toEqual([
			{ text: 'failed', classes: 'text-danger' },
			{ text: ' safely', classes: '' }
		]);
	});

	it('does not interpret HTML in terminal output', () => {
		expect(tokenizeAnsi('<img src=x onerror=alert(1)>')).toEqual([
			{ text: '<img src=x onerror=alert(1)>', classes: '' }
		]);
	});
});
