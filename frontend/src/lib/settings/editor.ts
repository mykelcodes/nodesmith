import type { Editor } from '$lib/api';

export const CUSTOM_EDITOR_OPTION = '__nodesmith_custom_editor__';

export const builtInEditorOptions = [
	{ value: 'code', label: 'Visual Studio Code' },
	{ value: 'cursor', label: 'Cursor' },
	{ value: 'zed', label: 'Zed' }
] as const;

export type BuiltInEditor = (typeof builtInEditorOptions)[number]['value'];

const builtInEditors = new Set<string>(builtInEditorOptions.map((option) => option.value));

export function isBuiltInEditor(editor: Editor): editor is BuiltInEditor {
	return builtInEditors.has(editor);
}

export function isAbsoluteExecutablePath(value: string): boolean {
	if (value.includes('\0') || /[\r\n]/.test(value)) return false;
	return (
		value.startsWith('/') ||
		/^[A-Za-z]:[\\/][^\\/]/.test(value) ||
		/^\\\\[^\\]+\\[^\\]+/.test(value)
	);
}

export function validateCustomEditorPath(value: string): string {
	if (value.length === 0) return 'Enter the absolute path to the editor executable.';
	if (value.trim() !== value) return 'Remove spaces before or after the executable path.';
	if (!isAbsoluteExecutablePath(value)) {
		return 'Use an absolute executable path, such as /usr/local/bin/editor or C:\\Apps\\Editor.exe.';
	}
	if (/^[A-Za-z]:[\\/]/.test(value) && !/\.(?:exe|com)$/i.test(value)) {
		return 'Windows custom editors must point to an .exe or .com executable.';
	}
	return '';
}
