#!/usr/bin/env node

const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const binName = process.platform === 'win32' ? 'querylex.exe' : 'querylex';
const binPath = path.join(__dirname, '..', 'bin', binName);

if (!fs.existsSync(binPath)) {
  console.error(
    'querylex binary not found — reinstall with: npm install -g cskiller24/querylex',
  );
  process.exit(1);
}

try {
  const args = process.argv.slice(2);
  execSync('"' + binPath + '" ' + args.map(function (a) { return '"' + a + '"'; }).join(' '), {
    stdio: 'inherit',
  });
} catch (err) {
  process.exit(err.status || 1);
}
