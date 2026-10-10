// The Eyelang extension. Highlighting comes from the TextMate grammar in
// syntaxes/; errors as you type, formatting, the outline, folding and the
// finer highlighting come from the language server, `eye lsp`.
'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');
const vscode = require('vscode');
const { LanguageClient } = require('vscode-languageclient/node');

const exe = process.platform === 'win32' ? 'eye.exe' : 'eye';

/** @type {LanguageClient | undefined} */
let client;
/** The binary the running server came from, and its modification time. */
let serverPath;
let serverTime;
/** A binary that failed to start; it isn't retried until it changes. */
let failed;
let starts = 0;
let restartTimer;
let pollTimer;

function isFile(p) {
  try {
    return fs.statSync(p).isFile();
  } catch {
    return false;
  }
}

function modified(p) {
  try {
    return fs.statSync(p).mtimeMs;
  } catch {
    return undefined;
  }
}

function folders() {
  return (vscode.workspace.workspaceFolders || []).map((f) => f.uri.fsPath);
}

/**
 * Finds the eye binary. A path in eyelang.server.path wins; `~` and
 * ${workspaceFolder} are expanded, and a relative path is looked up in each
 * workspace folder. Otherwise: bin/eye in a workspace folder (what `task build`
 * writes in the eyelang repository), then PATH, then GOBIN and ~/go/bin, which
 * an editor started from the Dock or a desktop menu often doesn't have on its
 * PATH.
 * @returns {string | undefined}
 */
function findEye() {
  const setting = (vscode.workspace.getConfiguration('eyelang').get('server.path') || '').trim();
  const home = os.homedir();
  const pathDirs = (process.env.PATH || '').split(path.delimiter).filter(Boolean);
  if (setting && setting !== 'eye') {
    const expanded = setting.replace(/^~(?=$|[\\/])/, home);
    const candidates = [];
    if (expanded.includes('${workspaceFolder}')) {
      candidates.push(...folders().map((f) => expanded.split('${workspaceFolder}').join(f)));
    } else if (path.isAbsolute(expanded)) {
      candidates.push(expanded);
    } else if (!expanded.includes('/') && !expanded.includes('\\')) {
      candidates.push(...pathDirs.map((d) => path.join(d, expanded)));
    } else {
      candidates.push(...folders().map((f) => path.join(f, expanded)));
    }
    return candidates.find(isFile);
  }
  const gopath = (process.env.GOPATH || path.join(home, 'go')).split(path.delimiter)[0];
  const candidates = [
    ...folders().map((f) => path.join(f, 'bin', exe)),
    ...pathDirs.map((d) => path.join(d, exe)),
    process.env.GOBIN && path.join(process.env.GOBIN, exe),
    path.join(gopath, 'bin', exe),
    path.join(home, '.local', 'bin', exe),
    '/usr/local/bin/eye',
    '/opt/homebrew/bin/eye',
  ];
  return candidates.find((c) => c && isFile(c));
}

/** The workspace folder that holds the eyelang repository, if one does. */
function eyelangRepo() {
  return (vscode.workspace.workspaceFolders || []).find((f) =>
    isFile(path.join(f.uri.fsPath, 'cmd', 'eye', 'main.go')),
  );
}

async function start() {
  if (!vscode.workspace.getConfiguration('eyelang').get('server.enabled', true)) {
    return;
  }
  const eye = findEye();
  if (!eye) {
    missing();
    return;
  }
  const time = modified(eye);
  const c = new LanguageClient(
    'eyelang',
    'Eyelang',
    { command: eye, args: ['lsp'] },
    { documentSelector: [{ language: 'eyelang' }] },
  );
  try {
    await c.start();
    client = c;
    serverPath = eye;
    serverTime = time;
    failed = undefined;
    starts++;
  } catch (err) {
    failed = { path: eye, time };
    const why = err instanceof Error ? err.message : String(err ?? 'unknown error');
    vscode.window.showWarningMessage(`Eyelang: "${eye} lsp" didn't start: ${why}. Highlighting still works.`);
  }
}

// Every two seconds: start the server once an eye binary appears (after a
// build, or `go install` into ~/go/bin), and restart it when its binary is
// rebuilt. A stat or two is cheaper and more reliable than file watchers,
// which miss files outside the workspace.
function poll() {
  if (restartTimer || !vscode.workspace.getConfiguration('eyelang').get('server.enabled', true)) {
    return;
  }
  if (client) {
    const time = modified(serverPath);
    if (time !== undefined && time !== serverTime) {
      scheduleRestart();
    }
    return;
  }
  const eye = findEye();
  if (eye && !(failed && failed.path === eye && failed.time === modified(eye))) {
    scheduleRestart();
  }
}

async function missing() {
  const repo = eyelangRepo();
  const actions = repo ? ['Build eye', 'Set Path'] : ['Set Path'];
  const where = repo
    ? 'Build it here, or set eyelang.server.path.'
    : 'Build it in the eyelang repository with "go build -o bin/eye ./cmd/eye" and set eyelang.server.path to it.';
  const choice = await vscode.window.showWarningMessage(
    `Eyelang: can't find the eye binary, so there are no errors or formatting yet; highlighting works. ${where}`,
    ...actions,
  );
  if (choice === 'Build eye' && repo) {
    build(repo);
  } else if (choice === 'Set Path') {
    vscode.commands.executeCommand('workbench.action.openSettings', 'eyelang.server.path');
  }
}

/**
 * Builds bin/eye in the eyelang repository in a terminal task, then starts
 * the server.
 * @param {vscode.WorkspaceFolder} folder
 */
async function build(folder) {
  const task = new vscode.Task(
    { type: 'eyelang', task: 'build' },
    folder,
    'build eye',
    'eyelang',
    new vscode.ShellExecution('go build -o bin/eye ./cmd/eye'),
  );
  const execution = await vscode.tasks.executeTask(task);
  const sub = vscode.tasks.onDidEndTaskProcess((e) => {
    if (e.execution !== execution) {
      return;
    }
    sub.dispose();
    if (e.exitCode === 0) {
      scheduleRestart();
    } else {
      vscode.window.showErrorMessage('Eyelang: building eye failed; the terminal shows why. Is Go installed?');
    }
  });
}

async function stop() {
  if (client) {
    const c = client;
    client = undefined;
    serverPath = undefined;
    await c.stop().catch(() => {});
  }
}

async function restart() {
  await stop();
  await start();
}

// A build may write the binary more than once; restart once, after the last
// write.
function scheduleRestart() {
  clearTimeout(restartTimer);
  restartTimer = setTimeout(() => {
    restartTimer = undefined;
    restart();
  }, 500);
}

/** @param {vscode.ExtensionContext} context */
async function activate(context) {
  pollTimer = setInterval(poll, 2000);
  context.subscriptions.push(
    { dispose: () => clearInterval(pollTimer) },
    vscode.commands.registerCommand('eyelang.restartServer', restart),
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration('eyelang.server')) {
        scheduleRestart();
      }
    }),
  );
  await start();
  // For the integration tests.
  return { serverPath: () => serverPath, running: () => client !== undefined, starts: () => starts };
}

function deactivate() {
  clearTimeout(restartTimer);
  clearInterval(pollTimer);
  return stop();
}

module.exports = { activate, deactivate };
