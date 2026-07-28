import assert from 'node:assert/strict';
import { access, readdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const frontendDirectory = fileURLToPath(new URL('../', import.meta.url));
const routesDirectory = path.join(frontendDirectory, 'src', 'routes');
const buildDirectory = path.join(frontendDirectory, 'build');

/**
 * @param {string} [directory]
 * @param {string[]} [segments]
 * @returns {Promise<string[]>}
 */
async function discoverPageRoutes(directory = routesDirectory, segments = []) {
	const entries = await readdir(directory, { withFileTypes: true });
	const routes = entries.some((entry) => entry.isFile() && entry.name === '+page.svelte')
		? [segments.join('/')]
		: [];

	for (const entry of entries) {
		if (!entry.isDirectory() || entry.name.startsWith('(') || entry.name.startsWith('@')) {
			continue;
		}

		routes.push(
			...(await discoverPageRoutes(path.join(directory, entry.name), [...segments, entry.name]))
		);
	}

	return routes;
}

/**
 * @param {string} filename
 * @param {string} message
 */
async function assertMissing(filename, message) {
	try {
		await access(filename);
		assert.fail(message);
	} catch (error) {
		if (error instanceof Error && 'code' in error && error.code === 'ENOENT') {
			return;
		}

		throw error;
	}
}

export async function assertWailsStaticBuild() {
	const routes = await discoverPageRoutes();
	assert.ok(routes.includes(''), 'The SvelteKit route inventory must include the root page');

	const indexFile = path.join(buildDirectory, 'index.html');
	const indexHtml = await readFile(indexFile, 'utf8');

	assert.match(
		indexHtml,
		/(?:href=|import\()(["'])(?:\.\/|\/)_app\/immutable\/entry\/start\.[^"']+\.js\1/,
		'build/index.html must load the SvelteKit client entry'
	);

	const appEntryPath = indexHtml.match(
		/import\((["'])((?:\.\/|\/)_app\/immutable\/entry\/app\.[^"']+\.js)\1\)/
	)?.[2];
	assert.ok(appEntryPath, 'build/index.html must import the SvelteKit app entry');

	const appEntry = await readFile(
		path.join(buildDirectory, appEntryPath.replace(/^\.?\//, '')),
		'utf8'
	);
	const truthyIdentifiers = [...appEntry.matchAll(/(?:^|[,;])([A-Za-z_$][\w$]*)=!0/g)].map(
		(match) => match[1]
	);
	assert.ok(
		truthyIdentifiers.some((identifier) => appEntry.includes(`${identifier} as hash`)),
		'The SvelteKit app entry must export hash routing as enabled'
	);

	for (const route of routes.filter(Boolean)) {
		assert.ok(
			!route.includes('['),
			`Dynamic route "${route}" needs a focused hash-routing assertion`
		);

		const routeUrl = new URL(`#/${route}`, 'http://wails.localhost/');
		assert.equal(
			routeUrl.pathname,
			'/',
			`The hash URL for /${route} must keep the Wails asset request at /`
		);
		assert.ok(
			appEntry.includes(JSON.stringify(`/${route}`)),
			`The client route manifest must include /${route}`
		);

		await assertMissing(
			path.join(buildDirectory, `${route}.html`),
			`build/${route}.html means /${route} is still being emitted as a pathname route`
		);
		await assertMissing(
			path.join(buildDirectory, route, 'index.html'),
			`build/${route}/index.html means /${route}/ is still being prerendered instead of hash-routed`
		);
	}

	console.log(
		`Verified one Wails-loadable SPA entry and ${routes.length - 1} hash routes under ${path.relative(
			process.cwd(),
			buildDirectory
		)}.`
	);
}

if (path.resolve(process.argv[1] ?? '') === fileURLToPath(import.meta.url)) {
	await assertWailsStaticBuild();
}
