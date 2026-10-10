// Runs the extension inside a real VS Code (downloaded into .vscode-test/ on
// first use) against a scratch workspace; test/integration/suite.js holds the
// checks. EYE names the eye binary (default: bin/eye at the repository root),
// and VSCODE_EXECUTABLE a VS Code to use instead of downloading one. On Linux
// without a display, run it under xvfb-run.
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
  // Nowhere the extension looks may already hold an eye binary.
  const empty = fs.mkdtempSync(path.join(os.tmpdir(), 'eyelang-empty-'));
  // An empty HOME hides ~/.local/bin/eye. On macOS VS Code keeps a key in
  // the login keychain under HOME, and without one asks for it in a dialog
  // that outlives the test, so there HOME stays, and ~/.local/bin/eye must
  // not exist.
  let home = { HOME: empty };
  if (process.platform === 'darwin') {
    const local = path.join(os.homedir(), '.local', 'bin', 'eye');
    if (fs.existsSync(local)) {
      throw new Error(`${local} exists, and the test needs a machine where the extension finds no eye`);
    }
    home = {};
  }
  try {
    await runTests({
      vscodeExecutablePath: process.env.VSCODE_EXECUTABLE || undefined,
      extensionDevelopmentPath: root,
      extensionTestsPath: path.join(__dirname, 'suite.js'),
      launchArgs: [workspace, '--disable-extensions', '--disable-workspace-trust', '--no-sandbox'],
      extensionTestsEnv: { EYE_BINARY: eye, GOPATH: empty, GOBIN: empty, ...home },
    });
  } finally {
    fs.rmSync(workspace, { recursive: true, force: true });
    fs.rmSync(empty, { recursive: true, force: true });
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
