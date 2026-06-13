#!/usr/bin/env node

var child_process = require('child_process');
var crypto = require('crypto');
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

function sha256(filePath) {
  return crypto.createHash('sha256').update(fs.readFileSync(filePath)).digest('hex');
}

function downloadFile(url, dest) {
  if (PLATFORM === 'Windows') {
    child_process.execSync('powershell -NoProfile -Command "[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; Invoke-WebRequest -Uri \'' + url + '\' -OutFile \'' + dest + '\'"', { stdio: 'pipe' });
  } else {
    try {
      child_process.execSync('curl -fsSL -o "' + dest + '" "' + url + '"', { stdio: 'pipe' });
    } catch (e) {
      child_process.execSync('wget -q -O "' + dest + '" "' + url + '"', { stdio: 'pipe' });
    }
  }
}

function install() {
  if (fs.existsSync(BIN)) {
    try { child_process.execSync('"' + BIN + '" --version', { stdio: 'pipe' }); return; }
    catch (e) {}
  }

  var version = getVersion();
  var archiveName = 'querylex_' + PLATFORM + '_' + ARCH + '.' + EXT;
  var baseUrl = 'https://github.com/' + REPO + '/releases/download/v' + version;
  var archive = path.join(BIN_DIR, archiveName);
  var checksumFile = path.join(BIN_DIR, 'checksums.txt');

  if (!fs.existsSync(BIN_DIR)) fs.mkdirSync(BIN_DIR, { recursive: true });

  try {
    process.stderr.write('Installing querylex v' + version + ' (one-time setup)...\n');

    // Download checksums
    downloadFile(baseUrl + '/checksums.txt', checksumFile);
    var checksums = fs.readFileSync(checksumFile, 'utf8');

    // Parse expected SHA256 for this archive
    var expected = null;
    var lines = checksums.split(/\r?\n/);
    for (var i = 0; i < lines.length; i++) {
      var parts = lines[i].trim().split(/\s+/);
      if (parts.length >= 2 && parts[1] === archiveName) {
        expected = parts[0];
        break;
      }
    }
    if (!expected) throw new Error('No checksum entry for ' + archiveName);

    // Download archive
    downloadFile(baseUrl + '/' + archiveName, archive);

    // Verify checksum
    var actual = sha256(archive);
    if (actual !== expected) {
      fs.unlinkSync(archive);
      fs.unlinkSync(checksumFile);
      throw new Error('Checksum mismatch: expected ' + expected + ', got ' + actual);
    }

    // Extract
    if (PLATFORM === 'Windows') {
      child_process.execSync('powershell -NoProfile -Command "Expand-Archive -Path \'' + archive + '\' -DestinationPath \'' + BIN_DIR + '\' -Force"', { stdio: 'inherit' });
    } else {
      child_process.execSync('tar -xzf "' + archive + '" -C "' + BIN_DIR + '"', { stdio: 'inherit' });
    }

    fs.unlinkSync(archive);
    fs.unlinkSync(checksumFile);
    if (PLATFORM !== 'Windows') fs.chmodSync(BIN, 0o755);
    return;
  } catch (e) {
    try { if (fs.existsSync(archive)) fs.unlinkSync(archive); } catch (e2) {}
    try { if (fs.existsSync(checksumFile)) fs.unlinkSync(checksumFile); } catch (e2) {}
    console.error('Failed to install querylex: ' + (e.message || e));
    process.exit(1);
  }
}

install();

var args = process.argv.slice(2);
try {
  child_process.execSync('"' + BIN + '" ' + args.map(function (a) { return '"' + a.replace(/"/g, '\\"') + '"'; }).join(' '), { stdio: 'inherit' });
} catch (err) {
  process.exit(err.status || 1);
}
