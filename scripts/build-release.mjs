import {readFileSync, readdirSync, unlinkSync, writeFileSync} from 'node:fs';
import {dirname, join, relative, resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
import {execFileSync, spawnSync} from 'node:child_process';

const projectRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const buildArgs = process.argv.slice(2);
const buildRoot = resolve(projectRoot, 'build');

const listFiles = (directory) => readdirSync(directory, {withFileTypes: true}).flatMap((entry) => {
    const path = join(directory, entry.name);
    return entry.isDirectory() ? listFiles(path) : entry.isFile() ? [path] : [];
});

const snapshotBuildAssets = () => new Map(listFiles(buildRoot).map((path) => [relative(buildRoot, path), readFileSync(path)]));

const restoreBuildAssets = (snapshot) => {
    for (const path of listFiles(buildRoot)) {
        if (!snapshot.has(relative(buildRoot, path))) unlinkSync(path);
    }
    for (const [path, content] of snapshot) writeFileSync(join(buildRoot, path), content);
};
let gitTag;
try {
    gitTag = execFileSync('git', ['describe', '--tags', '--abbrev=0', '--match', 'v[0-9]*', 'HEAD'], {
        cwd: projectRoot,
        encoding: 'utf8',
    }).trim();
} catch {
    console.error('Release builds require a reachable vMAJOR.MINOR.PATCH Git tag.');
    process.exit(2);
}

const versionMatch = /^v(\d+\.\d+\.\d+)$/.exec(gitTag);
if (!versionMatch) {
    console.error(`Unsupported release tag: ${gitTag}. Expected vMAJOR.MINOR.PATCH.`);
    process.exit(2);
}
const version = versionMatch[1];
const packagePath = 'github.com/yearnfar/keynest/internal/updater';
const targetOS = buildArgs.find((arg) => arg.startsWith('GOOS='))?.slice('GOOS='.length)
    || ({darwin: 'darwin', win32: 'windows', linux: 'linux'})[process.platform];
const linkerFlags = `-X ${packagePath}.Version=${version} -X ${packagePath}.BuildMode=production`;
const wailsCommand = process.env.WAILS || 'wails3';
const buildAssetSnapshot = snapshotBuildAssets();
let result;

try {
    console.log(`Building KeyNest ${gitTag}`);
    const metadataResult = spawnSync(wailsCommand, [
        'update', 'build-assets',
        '-name', 'keynest',
        '-binaryname', 'keynest',
        '-config', 'build/config.yml',
        '-dir', 'build',
        '-productversion', version,
        '-silent',
    ], {cwd: projectRoot, stdio: 'inherit'});
    if (metadataResult.status !== 0 || metadataResult.error) {
        result = metadataResult;
    } else {
        const task = targetOS === 'darwin' ? 'package' : 'build';
        result = spawnSync(wailsCommand, ['task', task, ...buildArgs, `EXTRA_LDFLAGS=${linkerFlags}`], {
            cwd: projectRoot,
            env: {...process.env, VITE_APP_VERSION: version},
            stdio: 'inherit',
        });
    }
} finally {
    restoreBuildAssets(buildAssetSnapshot);
}

if (result?.error) {
    console.error(result.error.message);
    process.exit(1);
}
if (result?.signal) {
    console.error(`Wails build terminated by ${result.signal}`);
    process.exit(1);
}
process.exit(result?.status ?? 1);
