import { chmod, readFile, readdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const modelsPath = fileURLToPath(new URL('../src/lib/wailsjs/go/models.ts', import.meta.url));
const runtimeDirectory = fileURLToPath(new URL('../src/lib/wailsjs/runtime/', import.meta.url));
const entries = await readdir(runtimeDirectory, { withFileTypes: true });

// Wails emits indented blank lines and host-native line endings in models.ts.
// Normalize both so the committed binding is reproducible on every CI runner.
const models = await readFile(modelsPath, 'utf8');
const normalizedModels = models.replace(/\r\n?/g, '\n').replace(/[\t ]+$/gm, '');
if (normalizedModels !== models) {
	await writeFile(modelsPath, normalizedModels);
}

for (const entry of entries) {
	if (entry.isFile()) {
		await chmod(path.join(runtimeDirectory, entry.name), 0o644);
	}
}
