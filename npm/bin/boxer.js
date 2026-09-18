#!/usr/bin/env node
// Runs the boxer binary that postinstall.js downloaded into vendor/, forwarding arguments,
// standard streams and the exit code unchanged. Signals are inherited with stdio: 'inherit'.
'use strict';
const path = require('path');
const fs = require('fs');
const { spawnSync } = require('child_process');

const binary = path.join(__dirname, '..', 'vendor', 'boxer');
if (!fs.existsSync(binary)) {
  // npm is moving towards refusing install scripts by default, and a package whose binary only
  // arrives through postinstall is broken the moment that happens. Fetch it on first use instead
  // of telling the user to reinstall: same script, same checksum check, one line of output.
  const postinstall = path.join(__dirname, '..', 'postinstall.js');
  const fetch = spawnSync(process.execPath, [postinstall], { stdio: 'inherit' });
  if (fetch.status !== 0 || !fs.existsSync(binary)) {
    console.error('boxer-cli: could not download the boxer binary. Install it directly with');
    console.error('  curl -fsSL https://raw.githubusercontent.com/BarakChamo/boxer/main/install.sh | sh');
    process.exit(1);
  }
}

const res = spawnSync(binary, process.argv.slice(2), { stdio: 'inherit' });
if (res.error) {
  console.error(`boxer-cli: ${res.error.message}`);
  process.exit(1);
}
// A binary killed by a signal has no exit code; report it the way a shell does.
process.exit(res.status === null ? 128 + (require('os').constants.signals[res.signal] || 0) : res.status);
