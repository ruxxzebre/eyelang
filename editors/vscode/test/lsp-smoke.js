// Runs `eye lsp` and talks to it with the JSON-RPC library the extension's
// client uses: initialize, open a file with an error, fix it, then ask for
// formatting, symbols and semantic tokens. EYE names the binary (default:
// bin/eye at the repository root).
'use strict';

const cp = require('child_process');
const path = require('path');
const rpc = require('vscode-jsonrpc/node');

const eye = process.env.EYE || path.join(__dirname, '..', '..', '..', 'bin', 'eye');

function fail(msg) {
  console.error(`lsp-smoke: ${msg}`);
  process.exit(1);
}

async function main() {
  const child = cp.spawn(eye, ['lsp'], { stdio: ['pipe', 'pipe', 'inherit'] });
  child.on('error', (err) => fail(`can't run ${eye}: ${err.message} (run task build first)`));
  const conn = rpc.createMessageConnection(
    new rpc.StreamMessageReader(child.stdout),
    new rpc.StreamMessageWriter(child.stdin),
  );
  const diagnostics = [];
  conn.onNotification('textDocument/publishDiagnostics', (p) => diagnostics.push(p));
  conn.listen();

  const init = await conn.sendRequest('initialize', { processId: process.pid, rootUri: null, capabilities: {} });
  if (!init.capabilities.semanticTokensProvider) fail('no semantic tokens in the capabilities');
  await conn.sendNotification('initialized', {});

  const uri = 'file:///smoke.eye';
  await conn.sendNotification('textDocument/didOpen', {
    textDocument: { uri, languageId: 'eyelang', version: 1, text: 'job j() {\n  sleep!(1 min\n}\n' },
  });
  await conn.sendNotification('textDocument/didChange', {
    textDocument: { uri, version: 2 },
    contentChanges: [{ text: 'job  j() {\n  sleep!(1 min)\n}\n' }],
  });
  const edits = await conn.sendRequest('textDocument/formatting', {
    textDocument: { uri }, options: { tabSize: 2, insertSpaces: true },
  });
  const symbols = await conn.sendRequest('textDocument/documentSymbol', { textDocument: { uri } });
  const tokens = await conn.sendRequest('textDocument/semanticTokens/full', { textDocument: { uri } });

  if (diagnostics.length !== 2) fail(`got ${diagnostics.length} diagnostics notifications, want 2`);
  if (diagnostics[0].diagnostics.length === 0) fail('no error for the unclosed call');
  if (diagnostics[1].diagnostics.length !== 0) fail('the error stayed after the fix');
  if (edits.length !== 1 || !edits[0].newText.startsWith('job j() {')) fail(`formatting gave ${JSON.stringify(edits)}`);
  if (symbols.length !== 1 || symbols[0].name !== 'j') fail(`symbols: ${JSON.stringify(symbols)}`);
  const waiting = init.capabilities.semanticTokensProvider.legend.tokenTypes.indexOf('waitingCall');
  const types = tokens.data.filter((_, i) => i % 5 === 3);
  if (!types.includes(waiting)) fail(`no waiting call among the semantic tokens ${JSON.stringify(tokens.data)}`);

  await conn.sendRequest('shutdown');
  await conn.sendNotification('exit');
  const code = await new Promise((resolve) => child.on('exit', resolve));
  if (code !== 0) fail(`eye lsp exited with ${code}`);
  console.log('✓ eye lsp answers the JSON-RPC client.');
  conn.dispose();
}

main().catch((err) => fail(err.stack || String(err)));
