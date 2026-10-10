// The Eyelang extension. Highlighting comes from the TextMate grammar in
// syntaxes/; errors as you type, formatting, the outline, folding and the
// finer highlighting come from the language server, `eye lsp`.
'use strict';

const vscode = require('vscode');
const { LanguageClient } = require('vscode-languageclient/node');

/** @type {LanguageClient | undefined} */
let client;

async function start() {
  const config = vscode.workspace.getConfiguration('eyelang');
  if (!config.get('server.enabled', true)) {
    return;
  }
  const command = config.get('server.path', 'eye') || 'eye';
  client = new LanguageClient(
    'eyelang',
    'Eyelang',
    { command, args: ['lsp'] },
    { documentSelector: [{ language: 'eyelang' }] },
  );
  try {
    await client.start();
  } catch (err) {
    client = undefined;
    vscode.window.showWarningMessage(
      `Eyelang: couldn't start "${command} lsp": ${err.message}. ` +
        'Highlighting still works. Set eyelang.server.path to the eye binary for errors and formatting.',
    );
  }
}

async function stop() {
  if (client) {
    const c = client;
    client = undefined;
    await c.stop();
  }
}

async function restart() {
  await stop();
  await start();
}

/** @param {vscode.ExtensionContext} context */
async function activate(context) {
  context.subscriptions.push(
    vscode.commands.registerCommand('eyelang.restartServer', restart),
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration('eyelang.server')) {
        restart();
      }
    }),
  );
  await start();
}

function deactivate() {
  return stop();
}

module.exports = { activate, deactivate };
