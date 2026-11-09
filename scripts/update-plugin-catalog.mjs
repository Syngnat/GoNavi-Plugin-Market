#!/usr/bin/env node
// Update one public/catalog.json item from a built plugin archive.
//
// The market manifest cannot be hand-maintained: version, installSpec, sha256
// and sizeBytes must all describe the *same* archive, and the archive is built
// by CI. This script is the single writer for those fields.
//
// Usage:
//   node scripts/update-plugin-catalog.mjs --manifest plugins/<name>/gonavi.plugin.json --zip <archive.zip>
//
// Exit codes: 0 updated or already current, 1 bad input.

import { createHash } from 'node:crypto'
import { readFileSync, writeFileSync } from 'node:fs'
import { basename, dirname, resolve } from 'node:path'

const repoUrl = 'https://github.com/Syngnat/GoNavi-Plugin-Market'

function fail(message) {
  console.error('update-plugin-catalog: ' + message)
  process.exit(1)
}

function parseArgs(argv) {
  const options = { manifest: '', zip: '', catalog: 'public/catalog.json', updatedAt: '' }
  for (let i = 0; i < argv.length; i += 1) {
    const flag = argv[i]
    const value = argv[i + 1]
    if (!flag.startsWith('--') || value === undefined) fail('bad argument ' + flag)
    i += 1
    switch (flag) {
      case '--manifest':
        options.manifest = value
        break
      case '--zip':
        options.zip = value
        break
      case '--catalog':
        options.catalog = value
        break
      case '--updated-at':
        options.updatedAt = value
        break
      default:
        fail('unknown flag ' + flag)
    }
  }
  if (!options.manifest || !options.zip) fail('--manifest and --zip are required')
  return options
}

const options = parseArgs(process.argv.slice(2))
const manifestPath = resolve(options.manifest)
const pluginDir = dirname(manifestPath)
const pluginName = basename(pluginDir)
const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'))
if (!manifest.id) fail('manifest has no id: ' + manifestPath)
if (!/^\d+\.\d+\.\d+$/.test(String(manifest.version || ''))) {
  fail('manifest version is not semver: ' + manifest.version)
}

const archive = readFileSync(resolve(options.zip))
const sha256 = createHash('sha256').update(archive).digest('hex')
const tag = pluginName + '-v' + manifest.version

const catalogPath = resolve(options.catalog)
const catalog = JSON.parse(readFileSync(catalogPath, 'utf8'))
const item = (catalog.items || []).find((candidate) => candidate.id === manifest.id)
if (!item) fail('catalog has no item with id ' + manifest.id)

const updatedAt = options.updatedAt || new Date().toISOString().replace(/\.\d{3}Z$/, 'Z')
item.version = manifest.version
item.minHostVersion = manifest.minHostVersion || item.minHostVersion
item.installSpec = 'github:Syngnat/GoNavi-Plugin-Market@' + tag
item.updatedAt = updatedAt
item.repoUrl = item.repoUrl || repoUrl
// contentDigest is a host-side canonical digest, not the archive hash. It
// cannot be derived here, so a stale value must not survive a new version.
delete item.contentDigest
item.artifact = {
  ...item.artifact,
  url: repoUrl + '/releases/download/' + tag + '/gonavi-plugin-' + pluginName + '-' + manifest.version + '.zip',
  sha256,
  sizeBytes: archive.length,
  platform: 'all',
  version: manifest.version,
}

const serialized = JSON.stringify(catalog, null, 2) + '\n'
const current = readFileSync(catalogPath, 'utf8')
if (current === serialized) {
  console.log('catalog already current for ' + manifest.id + ' ' + manifest.version)
  process.exit(0)
}
writeFileSync(catalogPath, serialized)
console.log('catalog updated: ' + manifest.id + ' ' + manifest.version + ' sha256=' + sha256)
