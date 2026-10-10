// Tokenizes every valid file in testdata/syntax with the TextMate grammar and
// fails if any token gets an `invalid` scope, or if a string or interpolation
// is still open at the end of a file.
'use strict';

const fs = require('fs');
const path = require('path');
const vsctm = require('vscode-textmate');
const oniguruma = require('vscode-oniguruma');

const root = path.join(__dirname, '..');
const valid = path.join(root, '..', '..', 'testdata', 'syntax', 'valid');

async function main() {
  const wasm = fs.readFileSync(require.resolve('vscode-oniguruma/release/onig.wasm'));
  await oniguruma.loadWASM(wasm.buffer.slice(wasm.byteOffset, wasm.byteOffset + wasm.byteLength));
  const registry = new vsctm.Registry({
    onigLib: Promise.resolve({
      createOnigScanner: (patterns) => new oniguruma.OnigScanner(patterns),
      createOnigString: (s) => new oniguruma.OnigString(s),
    }),
    loadGrammar: async () => {
      const file = path.join(root, 'syntaxes', 'eyelang.tmLanguage.json');
      return vsctm.parseRawGrammar(fs.readFileSync(file, 'utf8'), file);
    },
  });
  const grammar = await registry.loadGrammar('source.eyelang');

  const failures = [];
  const files = fs.readdirSync(valid).filter((f) => f.endsWith('.eye'));
  for (const name of files) {
    const lines = fs.readFileSync(path.join(valid, name), 'utf8').split(/\r?\n/);
    let stack = vsctm.INITIAL;
    lines.forEach((line, i) => {
      const result = grammar.tokenizeLine(line, stack);
      for (const t of result.tokens) {
        if (t.scopes.some((s) => s.startsWith('invalid'))) {
          failures.push(`${name}:${i + 1}:${t.startIndex + 1}: ${JSON.stringify(line.slice(t.startIndex, t.endIndex))} is ${t.scopes.join(' ')}`);
        }
      }
      stack = result.ruleStack;
    });
    const open = grammar.tokenizeLine('', stack).tokens[0].scopes;
    if (open.length > 1) {
      failures.push(`${name}: the file ends inside ${open.slice(1).join(' ')}`);
    }
  }
  if (failures.length > 0) {
    console.error(failures.join('\n'));
    process.exit(1);
  }
  console.log(`✓ ${files.length} valid files have no invalid tokens.`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
