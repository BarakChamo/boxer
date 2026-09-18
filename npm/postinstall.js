// Downloads the boxer release matching this package's version into vendor/ and verifies its
// sha256 against the release's checksums.txt. No dependencies; tar is expected on PATH.
// BOXER_BASE_URL overrides the asset directory (tests point it at a local server).
'use strict';
const fs = require('fs');
const path = require('path');
const https = require('https');
const http = require('http');
const crypto = require('crypto');
const { execFileSync } = require('child_process');

const pkg = require('./package.json');
const version = pkg.version;
const os = process.platform; // darwin | linux
const arch = { x64: 'amd64', arm64: 'arm64' }[process.arch];
const supported = ['darwin/arm64', 'darwin/amd64', 'linux/amd64', 'linux/arm64'];
if (!arch || !supported.includes(`${os}/${arch}`)) {
  console.error(`boxer-cli: no release binary for ${os}/${process.arch}; use: go install github.com/BarakChamo/boxer/cmd/boxer@v${version}`);
  process.exit(1);
}
if (version === '0.0.0') {
  console.error('boxer-cli: unreleased package version 0.0.0; nothing to download');
  process.exit(0);
}

const file = `boxer_${version}_${os}_${arch}.tar.gz`;
const base = process.env.BOXER_BASE_URL || `https://github.com/BarakChamo/boxer/releases/download/v${version}`;
const vendor = path.join(__dirname, 'vendor');

function get(url, redirects = 0) {
  return new Promise((resolve, reject) => {
    const mod = url.startsWith('http:') ? http : https;
    mod.get(url, { headers: { 'user-agent': 'boxer-cli' } }, (res) => {
      if ([301, 302, 303, 307, 308].includes(res.statusCode) && res.headers.location && redirects < 5) {
        res.resume();
        return resolve(get(new URL(res.headers.location, url).toString(), redirects + 1));
      }
      if (res.statusCode !== 200) {
        res.resume();
        return reject(new Error(`${url}: HTTP ${res.statusCode}`));
      }
      const chunks = [];
      res.on('data', (c) => chunks.push(c));
      res.on('end', () => resolve(Buffer.concat(chunks)));
      res.on('error', reject);
    }).on('error', reject);
  });
}

(async () => {
  const [archive, sums] = await Promise.all([get(`${base}/${file}`), get(`${base}/checksums.txt`)]);
  const line = sums.toString().split('\n').find((l) => l.trim().endsWith(` ${file}`) || l.trim().endsWith(`*${file}`));
  const expected = line && line.trim().split(/\s+/)[0];
  const actual = crypto.createHash('sha256').update(archive).digest('hex');
  if (!expected || expected !== actual) throw new Error(`checksum mismatch for ${file}`);
  fs.mkdirSync(vendor, { recursive: true });
  const tgz = path.join(vendor, file);
  fs.writeFileSync(tgz, archive);
  execFileSync('tar', ['-xzf', tgz, '-C', vendor, 'boxer']);
  fs.unlinkSync(tgz);
  fs.chmodSync(path.join(vendor, 'boxer'), 0o755);
  console.log(`boxer-cli: installed boxer ${version} (${os}/${arch})`);
})().catch((err) => {
  console.error(`boxer-cli: ${err.message}`);
  process.exit(1);
});
