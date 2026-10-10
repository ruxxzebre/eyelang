// Runs the extension inside a real VS Code (downloaded into .vscode-test/ on
// first use) against a scratch workspace; test/integration/suite.js holds the
// checks. EYE names the eye binary (default: bin/eye at the repository root),
// and VSCODE_EXECUTABLE a VS Code to use instead of downloading one. On Linux
// without a display, run it under xvfb-run.
//
// The test VS Code gets its own short user data directory, so it runs beside
// a VS Code you have open, and its socket path stays under macOS's 103
// characters even from a deep worktree.
'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');
const { runTests } = require('@vscode/test-electron');

async function main() {
  const root = path.join(__dirname, '..', '..');
  const eye = process.env.EYE || path.join(root, '..', '..', 'bin', 'eye');
  if (!fs.existsSync(eye)) {
    throw new Error(`${eye} doesn't exist; run task build first`);
  }
  const workspace = fs.mkdtempSync(path.join(os.tmpdir(), 'eyelang-vscode-'));
  fs.mkdirSync(path.join(workspace, 'bin'));
  fs.writeFileSync(path.join(workspace, 'a.eye'), 'let a = (1\n');
  // A program of two modules, for go-to-definition.
  fs.writeFileSync(path.join(workspace, 'eye.toml'), '');
  fs.mkdirSync(path.join(workspace, 'util'));
  fs.writeFileSync(path.join(workspace, 'util', 'util.eye'), 'pub fn twice(n: Int) -> Int = n * 2\n');
  fs.writeFileSync(path.join(workspace, 'b.eye'), 'import util\n\nfn main() {\n  print(util.twice(2))\n}\n');
  // Nowhere the extension looks may already hold an eye binary, so PATH loses
  // every directory that has one, such as scripts/, which direnv adds.
  const empty = fs.mkdtempSync(path.join(os.tmpdir(), 'eyelang-empty-'));
  const PATH = (process.env.PATH || '')
    .split(path.delimiter)
    .filter((d) => d && !fs.existsSync(path.join(d, 'eye')))
    .join(path.delimiter);
  // An empty HOME hides ~/.local/bin/eye. On macOS VS Code keeps a key in
  // the login keychain under HOME, and without one asks for it in a dialog
  // that outlives the test, so there HOME stays, and ~/.local/bin/eye must
  // not exist. Nor may the fixed places the extension looks in.
  let home = { HOME: empty };
  const fixed = ['/usr/local/bin/eye', '/opt/homebrew/bin/eye'];
  if (process.platform === 'darwin') {
    fixed.push(path.join(os.homedir(), '.local', 'bin', 'eye'));
    home = {};
  }
  for (const f of fixed) {
    if (fs.existsSync(f)) {
      throw new Error(`${f} exists, and the test needs a machine where the extension finds no eye`);
    }
  }
  const userData = fs.mkdtempSync(path.join(os.tmpdir(), 'evt-'));
  try {
    await runTests({
      vscodeExecutablePath: process.env.VSCODE_EXECUTABLE || undefined,
      extensionDevelopmentPath: root,
      extensionTestsPath: path.join(__dirname, 'suite.js'),
      launchArgs: [
        workspace,
        '--disable-extensions',
        '--disable-workspace-trust',
        '--no-sandbox',
        `--user-data-dir=${userData}`,
        // No keyring prompt on Linux; macOS ignores it and keeps HOME instead.
        '--password-store=basic',
      ],
      extensionTestsEnv: { EYE_BINARY: eye, GOPATH: empty, GOBIN: empty, PATH, ...home },
    });
  } finally {
    fs.rmSync(workspace, { recursive: true, force: true });
    fs.rmSync(empty, { recursive: true, force: true });
    fs.rmSync(userData, { recursive: true, force: true });
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
