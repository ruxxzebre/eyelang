// Runs inside VS Code. The workspace starts with a.eye, which has a syntax
// error, an empty bin/, and b.eye, which imports util/: the extension must
// start without a server, start one when bin/eye appears, restart it when
// bin/eye changes, and answer through it, go-to-definition included.
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const vscode = require('vscode');

async function waitFor(what, check, ms = 15000) {
  const end = Date.now() + ms;
  while (Date.now() < end) {
    if (await check()) {
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error(`timed out waiting for ${what}`);
}

exports.run = async function run() {
  const ws = vscode.workspace.workspaceFolders[0].uri.fsPath;
  const doc = await vscode.workspace.openTextDocument(path.join(ws, 'a.eye'));
  await vscode.window.showTextDocument(doc);
  assert.strictEqual(doc.languageId, 'eyelang');

  const ext = vscode.extensions.getExtension('eyelang.eyelang');
  const api = await ext.activate();
  assert.strictEqual(api.running(), false, 'a server is running though there is no eye binary');

  const eye = path.join(ws, 'bin', process.platform === 'win32' ? 'eye.exe' : 'eye');
  fs.copyFileSync(process.env.EYE_BINARY, eye);
  fs.chmodSync(eye, 0o755);
  await waitFor('the server to start once bin/eye exists', () => api.running());
  assert.strictEqual(api.serverPath(), eye);

  // Rebuilding eye restarts the server.
  const starts = api.starts();
  const later = new Date(Date.now() + 60000);
  fs.utimesSync(eye, later, later);
  await waitFor('a restart after bin/eye changed', () => api.starts() > starts && api.running());

  await waitFor('the syntax error', () => vscode.languages.getDiagnostics(doc.uri).length > 0);
  const [diag] = vscode.languages.getDiagnostics(doc.uri);
  assert.strictEqual(diag.source, 'eye');

  const edit = new vscode.WorkspaceEdit();
  edit.replace(doc.uri, new vscode.Range(0, 0, doc.lineCount, 0), 'let  a = 1\njob j() {\n  sleep!(1 min)\n}\n');
  assert.ok(await vscode.workspace.applyEdit(edit));
  await waitFor('the error to clear', () => vscode.languages.getDiagnostics(doc.uri).length === 0);

  const edits = await vscode.commands.executeCommand('vscode.executeFormatDocumentProvider', doc.uri, {
    tabSize: 2,
    insertSpaces: true,
  });
  // VS Code shrinks the server's whole-document edit to the changed part.
  const format = new vscode.WorkspaceEdit();
  format.set(doc.uri, edits);
  assert.ok(await vscode.workspace.applyEdit(format));
  assert.strictEqual(doc.getText(), 'let a = 1\njob j() {\n  sleep!(1 min)\n}\n');

  const symbols = await vscode.commands.executeCommand('vscode.executeDocumentSymbolProvider', doc.uri);
  assert.deepStrictEqual(
    symbols.map((s) => s.name),
    ['a', 'j'],
  );

  const legend = await vscode.commands.executeCommand('vscode.provideDocumentSemanticTokensLegend', doc.uri);
  const tokens = await vscode.commands.executeCommand('vscode.provideDocumentSemanticTokens', doc.uri);
  const waiting = legend.tokenTypes.indexOf('waitingCall');
  assert.ok(waiting >= 0, 'no waitingCall token type');
  assert.ok(
    tokens.data.some((v, i) => i % 5 === 3 && v === waiting),
    'sleep! is not marked as a waiting call',
  );

  // Go to definition: into another module, and into the Go behind a built-in.
  const b = await vscode.workspace.openTextDocument(path.join(ws, 'b.eye'));
  const definition = async (line, character) => {
    const locs = await vscode.commands.executeCommand('vscode.executeDefinitionProvider', b.uri, new vscode.Position(line, character));
    return locs.map((l) => l.uri || l.targetUri);
  };
  const [twice] = await definition(3, 14);
  assert.ok(twice && twice.fsPath.endsWith(path.join('util', 'util.eye')), `util.twice goes to ${twice}`);
  const [print] = await definition(3, 3);
  assert.ok(print && print.fsPath.endsWith(path.join('interp', 'builtins.go')), `print goes to ${print}`);
};
