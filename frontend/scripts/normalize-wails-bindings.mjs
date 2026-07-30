import { chmod, readdir } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const runtimeDirectory = fileURLToPath(new URL('../src/lib/wailsjs/runtime/', import.meta.url));
const entries = await readdir(runtimeDirectory, { withFileTypes: true });

for (const entry of entries) {
	if (entry.isFile()) {
		await chmod(path.join(runtimeDirectory, entry.name), 0o644);
	}
}
