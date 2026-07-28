import { describe, expect, it } from 'vitest';
import { isAbsoluteExecutablePath, isBuiltInEditor, validateCustomEditorPath } from './editor';

describe('custom editor settings', () => {
	it('recognises only the built-in editor identifiers', () => {
		expect(isBuiltInEditor('code')).toBe(true);
		expect(isBuiltInEditor('cursor')).toBe(true);
		expect(isBuiltInEditor('zed')).toBe(true);
		expect(isBuiltInEditor('/opt/My Editor/editor')).toBe(false);
	});

	it('accepts absolute executable paths without changing paths that contain spaces', () => {
		expect(isAbsoluteExecutablePath('/Applications/My Editor.app/Contents/MacOS/Editor')).toBe(
			true
		);
		expect(isAbsoluteExecutablePath('C:\\Program Files\\Editor\\Editor.exe')).toBe(true);
		expect(isAbsoluteExecutablePath('\\\\server\\apps\\Editor.exe')).toBe(true);
		expect(validateCustomEditorPath('/Applications/My Editor.app/Contents/MacOS/Editor')).toBe('');
	});

	it('rejects empty, relative, padded, multiline, and non-native Windows paths', () => {
		expect(validateCustomEditorPath('')).toMatch(/absolute path/i);
		expect(validateCustomEditorPath('./editor')).toMatch(/absolute executable path/i);
		expect(validateCustomEditorPath(' /usr/bin/editor')).toMatch(/before or after/i);
		expect(validateCustomEditorPath('/usr/bin/editor\n--flag')).toMatch(
			/absolute executable path/i
		);
		expect(validateCustomEditorPath('C:\\Apps\\Editor')).toMatch(/\.exe or \.com/i);
	});
});
