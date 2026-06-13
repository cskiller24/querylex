#!/usr/bin/env node

var child_process = require('child_process');
var fs = require('fs');
var path = require('path');

var PLATFORM = process.platform === 'win32' ? 'Windows' : process.platform === 'darwin' ? 'Darwin' : 'Linux';
var ARCH = process.arch === 'x64' ? 'x86_64' : process.arch === 'arm64' ? 'arm64' : process.arch === 'ia32' ? 'i386' : process.arch;
var EXT = PLATFORM === 'Windows' ? 'zip' : 'tar.gz';
var BIN_DIR = path.join(__dirname, '..', 'bin');
var BIN_NAME = PLATFORM === 'Windows' ? 'querylex.exe' : 'querylex';
var BIN = path.join(BIN_DIR, BIN_NAME);
var REPO = 'cskiller24/querylex';

function getVersion() {
  try {
    return JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'package.json'), 'utf8')).version;
  } catch (e) { return '0.0.0'; }
}

function install() {
  if (fs.existsSync(BIN)) {
    try { child_process.execSync('"' + BIN + '" --version', { stdio: 'pipe' }); return; }
    catch (e) {}
  }

  var version = getVersion();
  var name = 'querylex_' + PLATFORM + '_' + ARCH + '.' + EXT;
  var urls = [
    'https://github.com/' + REPO + '/releases/download/v' + version + '/' + name,
    'https://github.com/' + REPO + '/releases/latest/download/' + name,
  ];
  var archive = path.join(BIN_DIR, name);

  if (!fs.existsSync(BIN_DIR)) fs.mkdirSync(BIN_DIR, { recursive: true });

  for (var i = 0; i < urls.length; i++) {
    try {
      process.stderr.write('Installing querylex v' + version + ' (one-time setup)...\n');

      if (PLATFORM === 'Windows') {
        child_process.execSync('powershell -NoProfile -Command "[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; Invoke-WebRequest -Uri \'' + urls[i] + '\' -OutFile \'' + archive + '\'"', { stdio: 'pipe' });
      } else {
        try {
          child_process.execSync('curl -fsSL -o "' + archive + '" "' + urls[i] + '"', { stdio: 'pipe' });
        } catch (e) {
          child_process.execSync('wget -q -O "' + archive + '" "' + urls[i] + '"', { stdio: 'pipe' });
        }
      }

      if (PLATFORM === 'Windows') {
        child_process.execSync('powershell -NoProfile -Command "Expand-Archive -Path \'' + archive + '\' -DestinationPath \'' + BIN_DIR + '\' -Force"', { stdio: 'inherit' });
      } else {
        child_process.execSync('tar -xzf "' + archive + '" -C "' + BIN_DIR + '"', { stdio: 'inherit' });
      }

      fs.unlinkSync(archive);
      if (PLATFORM !== 'Windows') fs.chmodSync(BIN, 0o755);
      return;
    } catch (e) {
      try { if (fs.existsSync(archive)) fs.unlinkSync(archive); } catch (e2) {}
    }
  }

  console.error('Failed to install querylex. Build from source: go build -o /usr/local/bin/querylex ./cmd/querylex');
  process.exit(1);
}

install();

var args = process.argv.slice(2);
try {
  child_process.execSync('"' + BIN + '" ' + args.map(function (a) { return '"' + a.replace(/"/g, '\\"') + '"'; }).join(' '), { stdio: 'inherit' });
} catch (err) {
  process.exit(err.status || 1);
}
