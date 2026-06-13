#!/usr/bin/env node

const https = require('https');
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const PLATFORM_MAP = {
  linux: 'Linux',
  darwin: 'Darwin',
  win32: 'Windows',
};

const ARCH_MAP = {
  x64: 'x86_64',
  arm64: 'arm64',
  ia32: 'i386',
};

const REPO = 'cskiller24/querylex';

function getVersion() {
  try {
    const pkg = JSON.parse(
      fs.readFileSync(path.join(__dirname, '..', 'package.json'), 'utf8'),
    );
    return pkg.version;
  } catch {
    return '0.0.0';
  }
}

const VERSION = getVersion();

function getPlatform() {
  const p = PLATFORM_MAP[process.platform];
  if (!p) throw new Error(`Unsupported platform: ${process.platform}`);
  return p;
}

function getArch() {
  const a = ARCH_MAP[process.arch];
  if (!a) throw new Error(`Unsupported architecture: ${process.arch}`);
  return a;
}

function downloadToFile(url, dest) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);
    const request = (u) => {
      https
        .get(u, (response) => {
          if (response.statusCode >= 300 && response.statusCode < 400 && response.headers.location) {
            return request(response.headers.location);
          }
          if (response.statusCode !== 200) {
            file.close();
            fs.unlinkSync(dest);
            return reject(new Error(`Download failed: HTTP ${response.statusCode}`));
          }
          response.pipe(file);
          file.on('finish', () => {
            file.close();
            resolve();
          });
        })
        .on('error', (err) => {
          file.close();
          if (fs.existsSync(dest)) fs.unlinkSync(dest);
          reject(err);
        });
    };
    request(url);
  });
}

async function main() {
  const platform = getPlatform();
  const arch = getArch();
  const ext = platform === 'Windows' ? 'zip' : 'tar.gz';
  const archiveName = `querylex_${platform}_${arch}.${ext}`;
  const binDir = path.join(__dirname, '..', 'bin');
  if (!fs.existsSync(binDir)) fs.mkdirSync(binDir, { recursive: true });

  const binaryName = platform === 'Windows' ? 'querylex.exe' : 'querylex';
  const binPath = path.join(binDir, binaryName);

  // Don't re-download if binary already exists and works
  if (fs.existsSync(binPath)) {
    try {
      execSync(`"${binPath}" --version`, { stdio: 'pipe' });
      console.log(`querylex v${VERSION} already installed.`);
      return;
    } catch {
      console.log('Existing binary is broken, re-downloading...');
    }
  }

  // Try release tag first, then latest release
  const urls = [
    `https://github.com/${REPO}/releases/download/v${VERSION}/${archiveName}`,
    `https://github.com/${REPO}/releases/latest/download/${archiveName}`,
  ];

  const archivePath = path.join(binDir, archiveName);
  let downloaded = false;

  console.log('Installing querylex v' + VERSION + ' for ' + platform + ' ' + arch + '...\n');
  for (const url of urls) {
    try {
      await downloadToFile(url, archivePath);
      downloaded = true;
      break;
    } catch (err) {
      try { if (fs.existsSync(archivePath)) fs.unlinkSync(archivePath); } catch {}
    }
  }

  if (!downloaded) {
    console.error(
      `Could not download pre-built binary for ${platform}/${arch}.`,
    );
    console.error(
      'Build from source: git clone https://github.com/cskiller24/querylex && cd querylex && make build',
    );
    process.exit(1);
  }

  console.log('Extracting...');

  if (platform === 'Windows') {
    execSync(
      `powershell -Command "Expand-Archive -Path '${archivePath}' -DestinationPath '${binDir}' -Force"`,
      { stdio: 'inherit' },
    );
  } else {
    execSync(`tar -xzf "${archivePath}" -C "${binDir}"`, {
      stdio: 'inherit',
    });
  }

  fs.unlinkSync(archivePath);

  if (platform !== 'Windows') {
    fs.chmodSync(binPath, 0o755);
  }

  console.log(`querylex v${VERSION} installed successfully.`);
  console.log('Run "querylex add-db" to get started.');
}

main().catch((err) => {
  console.error('querylex install failed:', err.message);
  process.exit(1);
});
