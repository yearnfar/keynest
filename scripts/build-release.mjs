import {readFileSync, writeFileSync} from 'node:fs';
import {dirname, resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
import {execFileSync, spawnSync} from 'node:child_process';

const projectRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const buildArgs = process.argv.slice(2);
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
const configPath = resolve(projectRoot, 'wails.json');
const originalConfig = readFileSync(configPath, 'utf8');
const config = JSON.parse(originalConfig);
config.info = {...config.info, productVersion: version};

const packagePath = 'github.com/yearnfar/keynest/internal/updater';
const linkerFlags = `-X ${packagePath}.Version=${version} -X ${packagePath}.BuildMode=production`;
const wailsCommand = process.env.WAILS || 'wails';
let result;

try {
    console.log(`Building KeyNest ${gitTag}`);
    writeFileSync(configPath, `${JSON.stringify(config, null, 2)}\n`);
    result = spawnSync(wailsCommand, ['build', ...buildArgs, '-ldflags', linkerFlags], {
        cwd: projectRoot,
        env: {...process.env, VITE_APP_VERSION: version},
        stdio: 'inherit',
    });
} finally {
    writeFileSync(configPath, originalConfig);
}

if (result.error) {
    console.error(result.error.message);
    process.exit(1);
}
if (result.signal) {
    console.error(`Wails build terminated by ${result.signal}`);
    process.exit(1);
}
process.exit(result.status ?? 1);
