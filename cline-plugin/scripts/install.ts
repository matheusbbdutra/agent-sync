#!/usr/bin/env node
// scripts/install.ts — wirer do plugin Cline (A-90, passo 8).
//
// Substitui `internal/hooks/apply_cline.go` (que será deletado no passo 9).
// Faz:
//   1. resolve baseDir (do agent-sync-config.json ou argv)
//   2. hash sha256(baseDir)[:12] para nome do diretório
//   3. mkdir -p ~/.cline/plugins/_installed/local/agent-sync-hooks-<hash>/package/
//   4. copia dist/{index,types}.js + dist/src/** + package.json (raiz)
//   5. grava agent-sync-config.json com baseDir + bin
//   6. poda plugins órfãos de outros baseDirs
//   7. recusa baseDir não-canônico (escape hatch AGENT_SYNC_ALLOW_BASEDIR=1)

import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync, statSync } from "node:fs";
import { join, dirname } from "node:path";
import { homedir } from "node:os";
import { fileURLToPath } from "node:url";

const PLUGIN_NAME = "agent-sync-hooks";
const HASH_LEN = 12;
const PLUGIN_ROOT_RE = /^agent-sync-hooks(-[a-f0-9]{12})?$/;

// ---- helpers ----

function die(msg: string): never {
  process.stderr.write(`install-cline: ${msg}\n`);
  process.exit(1);
}

function baseDirShort(baseDir: string): string {
  return createHash("sha256").update(baseDir).digest("hex").slice(0, HASH_LEN);
}

function pluginInstallDir(hooksDir: string, baseDir: string): string {
  const h = baseDirShort(baseDir);
  return join(hooksDir, `${PLUGIN_NAME}-${h}`, "package");
}

function isOwnDir(name: string): boolean {
  return PLUGIN_ROOT_RE.test(name);
}

function copyDir(src: string, dst: string): void {
  if (!existsSync(src)) return;
  mkdirSync(dst, { recursive: true });
  for (const entry of readdirSync(src)) {
    const sp = join(src, entry);
    const dp = join(dst, entry);
    const st = statSync(sp);
    if (st.isDirectory()) copyDir(sp, dp);
    else writeFileSync(dp, readFileSync(sp));
  }
}

// ---- main ----

const argv = process.argv.slice(2);
const repoRoot = dirname(fileURLToPath(import.meta.url));
const clinePluginDir = join(repoRoot, "..");

const baseDir = (argv[0] && !argv[0].startsWith("--")) ? argv[0] : process.cwd();
if (!existsSync(baseDir)) die(`baseDir não existe: ${baseDir}`);

// Lê config canônico (registrado em ~/.config/agent-sync/config.json).
const homeConfig = join(homedir(), ".config", "agent-sync", "config.json");
let canonicalRepo: string | undefined;
if (existsSync(homeConfig)) {
  try {
    const cfg = JSON.parse(readFileSync(homeConfig, "utf8")) as { repo?: string };
    canonicalRepo = cfg.repo;
  } catch { /* ignore */ }
}

const allowBasedir = process.env["AGENT_SYNC_ALLOW_BASEDIR"] === "1";
if (canonicalRepo && canonicalRepo !== baseDir && !allowBasedir) {
  die(
    `baseDir ${baseDir} difere do repo canônico ${canonicalRepo}. ` +
    `Use AGENT_SYNC_ALLOW_BASEDIR=1 para ignorar (escopo A-84).`,
  );
}

const clineConfigDir = process.env["CLINE_CONFIG_DIR"] ?? join(homedir(), ".cline");
const localPluginsDir = join(clineConfigDir, "plugins", "_installed", "local");
mkdirSync(localPluginsDir, { recursive: true });

// Poda plugins órfãos de outros baseDirs.
let pruned: string[] = [];
for (const name of readdirSync(localPluginsDir)) {
  const full = join(localPluginsDir, name);
  if (!statSync(full).isDirectory()) continue;
  if (!isOwnDir(name)) continue;
  if (name === `${PLUGIN_NAME}-${baseDirShort(baseDir)}`) continue;
  try {
    rmSync(full, { recursive: true, force: true });
    pruned.push(name);
  } catch { /* ignore */ }
}

// Destino.
const installDir = pluginInstallDir(localPluginsDir, baseDir);
mkdirSync(installDir, { recursive: true });

// Copia artefatos TS wirados.
const distDir = join(clinePluginDir, "dist");
if (!existsSync(distDir)) die(`dist/ não encontrado. Rode \`make cline-plugin-build\` antes.`);
copyDir(join(distDir, "src"), join(installDir, "dist", "src"));
writeFileSync(join(installDir, "dist", "index.js"), readFileSync(join(distDir, "index.js")));
writeFileSync(join(installDir, "dist", "types.js"), readFileSync(join(distDir, "types.js")));

// package.json agregador (Cline carrega via `extensions[name]`).
const coreBinPath = process.env["AGENT_SYNC_BIN"] ?? join(homedir(), ".local", "bin", "agent-sync");
const pkg = {
  name: "cline-plugin-agent-sync",
  version: "0.2.0",
  private: true,
  main: "dist/index.js",
  cline: {
    agentPlugin: PLUGIN_NAME,
  },
};
writeFileSync(join(installDir, "package.json"), JSON.stringify(pkg, null, 2) + "\n");

// agent-sync-config.json (para hooks TS resolverem baseDir + bin).
writeFileSync(
  join(installDir, "agent-sync-config.json"),
  JSON.stringify({ baseDir, bin: coreBinPath }, null, 2) + "\n",
);

process.stdout.write(
  `[install-cline] wirado em ${installDir}` +
  (pruned.length > 0 ? ` (podados: ${pruned.join(", ")})` : "") +
  "\n",
);