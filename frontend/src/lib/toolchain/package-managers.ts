import type { Tool, Toolchain } from '$lib/api';

export function isUsableTool(tool: Tool | undefined): boolean {
	return Boolean(tool?.present && tool.error === '' && tool.version.trim() !== '');
}

export function availablePackageManagers(
	required: readonly string[],
	toolchain: Toolchain
): string[] {
	const toolsByName = new Map(toolchain.tools.map((tool) => [tool.name, tool]));
	return required.filter((manager) => isUsableTool(toolsByName.get(manager)));
}

export function repairPackageManagerSelection(
	required: readonly string[],
	toolchain: Toolchain,
	selected: string
): string {
	const available = availablePackageManagers(required, toolchain);
	return available.includes(selected) ? selected : (available.at(0) ?? '');
}
