#!/usr/bin/env node
"use strict";

const crypto = require("crypto");
const fs = require("fs");
const os = require("os");
const path = require("path");
const { spawnSync } = require("child_process");

const args = parseArgs(process.argv.slice(2));
const repoRoot = path.resolve(__dirname, "..");
const outputRoot = path.resolve(repoRoot, args.outputRoot || ".offline");
const packageName = args.includeGoModules
  ? "new-api-offline-inputs-with-gomod"
  : "new-api-offline-inputs-dist";
const packageDir = path.join(outputRoot, packageName);
const archivePath = path.join(outputRoot, `${packageName}.tar.gz`);
const goModCache = path.join(outputRoot, "gomodcache");
const goProxyCache = path.join(goModCache, "cache", "download");

main();

function main() {
  process.chdir(repoRoot);
  step("Checking tools");
  requireCommand("bun", "Install or unpack Bun and add it to PATH.");
  requireCommand("tar", "Windows Server 2019, modern Windows, Linux, and macOS normally include tar.");
  if (args.includeGoModules) {
    requireCommand("go", "Install or unpack Go and add it to PATH.");
  }

  const version = readFile("VERSION").trim();

  step("Building web/default dist");
  if (!args.skipBunInstall) {
    run("bun", ["install", "--frozen-lockfile"], {
      cwd: path.join(repoRoot, "web", "default"),
    });
  }
  run("bun", ["run", "build"], {
    cwd: path.join(repoRoot, "web", "default"),
    env: {
      DISABLE_ESLINT_PLUGIN: "true",
      VITE_REACT_APP_VERSION: version,
    },
  });

  step("Creating web/classic placeholder dist");
  const classicDist = path.join(repoRoot, "web", "classic", "dist");
  fs.mkdirSync(classicDist, { recursive: true });
  fs.writeFileSync(
    path.join(classicDist, "index.html"),
    "<!doctype html><html><head><title>classic disabled</title></head><body>classic frontend is not bundled</body></html>\n",
    "utf8",
  );

  assertFile(path.join(repoRoot, "web", "default", "dist", "index.html"));
  assertFile(path.join(repoRoot, "web", "classic", "dist", "index.html"));

  if (args.includeGoModules) {
    step("Downloading Go modules into a portable file proxy cache");
    if (!args.skipGoModDownload) {
      removeIfExists(goModCache);
      fs.mkdirSync(goModCache, { recursive: true });
      run("go", ["mod", "download", "all"], {
        cwd: repoRoot,
        env: {
          GOMODCACHE: goModCache,
          GOTOOLCHAIN: "local",
        },
      });
    } else {
      console.log(`Skipping go mod download; existing cache will be packaged from ${goModCache}`);
    }
    assertDirectory(goProxyCache);
  } else {
    step("Skipping Go module packaging");
    console.log("Use --include-go-modules for the first transfer or whenever go.mod/go.sum changes.");
  }

  step("Preparing package directory");
  fs.mkdirSync(outputRoot, { recursive: true });
  removeIfExists(packageDir);
  removeIfExists(archivePath);
  fs.mkdirSync(packageDir, { recursive: true });

  step("Copying source tree");
  copyRepoTree(repoRoot, packageDir);

  copyDirectory(
    path.join(repoRoot, "web", "default", "dist"),
    path.join(packageDir, "web", "default", "dist"),
  );
  copyDirectory(
    path.join(repoRoot, "web", "classic", "dist"),
    path.join(packageDir, "web", "classic", "dist"),
  );
  if (args.includeGoModules) {
    copyDirectory(goProxyCache, path.join(packageDir, "offline", "gomodproxy"));
  }

  fs.writeFileSync(path.join(packageDir, "A-RUNBOOK.md"), createRunbook(), "utf8");

  step("Writing checksums");
  const checksumText = listFiles(packageDir)
    .filter((file) => path.basename(file) !== "SHA256SUMS.txt")
    .map((file) => {
      const rel = toPosix(path.relative(packageDir, file));
      return `${sha256(file)}  ${rel}`;
    })
    .join("\n");
  fs.writeFileSync(path.join(packageDir, "SHA256SUMS.txt"), `${checksumText}\n`, "ascii");

  step("Creating tar.gz");
  run("tar", ["-czf", archivePath, "-C", packageDir, "."], { cwd: repoRoot });

  console.log("");
  console.log("Offline input package created:");
  console.log(`  ${archivePath}`);
  console.log("SHA256:");
  console.log(`  ${sha256(archivePath)}`);
  console.log("");
  console.log("Copy this archive to machine A and extract it into the A-side build directory.");
}

function parseArgs(argv) {
  const parsed = {
    includeGoModules: false,
    outputRoot: "",
    skipBunInstall: false,
    skipGoModDownload: false,
  };

  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === "--include-go-modules") {
      parsed.includeGoModules = true;
    } else if (arg === "--skip-bun-install") {
      parsed.skipBunInstall = true;
    } else if (arg === "--skip-go-mod-download") {
      parsed.skipGoModDownload = true;
    } else if (arg === "--output-root") {
      i += 1;
      if (!argv[i]) throw new Error("--output-root requires a value");
      parsed.outputRoot = argv[i];
    } else if (arg === "-h" || arg === "--help") {
      printHelp();
      process.exit(0);
    } else {
      throw new Error(`Unknown argument: ${arg}`);
    }
  }

  if (parsed.skipGoModDownload && !parsed.includeGoModules) {
    throw new Error("--skip-go-mod-download only makes sense with --include-go-modules");
  }

  return parsed;
}

function printHelp() {
  console.log(`Usage:
  bun scripts/package-offline-inputs.cjs [options]
  node scripts/package-offline-inputs.cjs [options]

Options:
  --include-go-modules      Package offline/gomodproxy. Use on first transfer or when go.mod/go.sum changes.
  --skip-bun-install        Reuse existing web/default/node_modules.
  --skip-go-mod-download    Reuse .offline/gomodcache when packaging modules.
  --output-root <path>      Output directory. Default: .offline
`);
}

function step(message) {
  console.log("");
  console.log(`==> ${message}`);
}

function requireCommand(command, hint) {
  const probe = os.platform() === "win32" ? "where" : "command";
  const probeArgs = os.platform() === "win32" ? [command] : ["-v", command];
  const result = os.platform() === "win32"
    ? spawnSync(probe, probeArgs, { stdio: "ignore" })
    : spawnSync("sh", ["-c", `command -v ${shellQuote(command)}`], { stdio: "ignore" });
  if (result.status !== 0) {
    throw new Error(`Required command '${command}' was not found. ${hint}`);
  }
}

function run(command, commandArgs, options = {}) {
  console.log(`${command} ${commandArgs.join(" ")}`);
  const result = spawnSync(command, commandArgs, {
    cwd: options.cwd || repoRoot,
    env: { ...process.env, ...(options.env || {}) },
    stdio: "inherit",
    shell: false,
  });
  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    throw new Error(`Command failed with exit code ${result.status}: ${command} ${commandArgs.join(" ")}`);
  }
}

function readFile(relativePath) {
  const fullPath = path.join(repoRoot, relativePath);
  assertFile(fullPath);
  return fs.readFileSync(fullPath, "utf8");
}

function assertFile(filePath) {
  if (!fs.existsSync(filePath) || !fs.statSync(filePath).isFile()) {
    throw new Error(`Required file does not exist: ${filePath}`);
  }
}

function assertDirectory(dirPath) {
  if (!fs.existsSync(dirPath) || !fs.statSync(dirPath).isDirectory()) {
    throw new Error(`Required directory does not exist: ${dirPath}`);
  }
}

function removeIfExists(targetPath) {
  if (fs.existsSync(targetPath)) {
    fs.rmSync(targetPath, { recursive: true, force: true });
  }
}

function copyRepoTree(sourceRoot, targetRoot) {
  const skipDirs = new Set([
    ".git",
    ".offline",
    ".gocache",
    ".gomodcache",
    ".gocache-temp",
    ".gopath",
    ".cache",
    ".idea",
    ".vscode",
    ".zed",
    ".codex",
    ".claude",
    ".cursor",
    ".history",
    "node_modules",
    "offline",
    "logs",
    "data",
    "upload",
    "plans",
  ]);
  const skipFiles = new Set([".env", "new-api", "one-api"]);

  function shouldSkip(srcPath, dirent) {
    const rel = toPosix(path.relative(sourceRoot, srcPath));
    const base = path.basename(srcPath);
    if (!rel) return false;
    if (dirent.isDirectory() && skipDirs.has(base)) return true;
    if (dirent.isFile() && skipFiles.has(base)) return true;
    if (rel === "web/default/dist" || rel.startsWith("web/default/dist/")) return true;
    if (rel === "web/classic/dist" || rel.startsWith("web/classic/dist/")) return true;
    if (rel === "electron/dist" || rel.startsWith("electron/dist/")) return true;
    if (rel.endsWith(".db") || rel.endsWith(".db-journal")) return true;
    if (rel.endsWith(".tar") || rel.endsWith(".tar.gz") || rel.endsWith(".zip")) return true;
    return false;
  }

  function walk(srcDir, dstDir) {
    fs.mkdirSync(dstDir, { recursive: true });
    for (const dirent of fs.readdirSync(srcDir, { withFileTypes: true })) {
      const srcPath = path.join(srcDir, dirent.name);
      if (shouldSkip(srcPath, dirent)) continue;
      const dstPath = path.join(dstDir, dirent.name);
      if (dirent.isDirectory()) {
        walk(srcPath, dstPath);
      } else if (dirent.isFile()) {
        fs.mkdirSync(path.dirname(dstPath), { recursive: true });
        fs.copyFileSync(srcPath, dstPath);
      } else if (dirent.isSymbolicLink()) {
        const linkTarget = fs.readlinkSync(srcPath);
        fs.symlinkSync(linkTarget, dstPath);
      }
    }
  }

  walk(sourceRoot, targetRoot);
}

function copyDirectory(source, destination) {
  assertDirectory(source);
  removeIfExists(destination);
  fs.mkdirSync(destination, { recursive: true });

  function walk(srcDir, dstDir) {
    for (const dirent of fs.readdirSync(srcDir, { withFileTypes: true })) {
      const srcPath = path.join(srcDir, dirent.name);
      const dstPath = path.join(dstDir, dirent.name);
      if (dirent.isDirectory()) {
        fs.mkdirSync(dstPath, { recursive: true });
        walk(srcPath, dstPath);
      } else if (dirent.isFile()) {
        fs.copyFileSync(srcPath, dstPath);
      } else if (dirent.isSymbolicLink()) {
        fs.symlinkSync(fs.readlinkSync(srcPath), dstPath);
      }
    }
  }

  walk(source, destination);
}

function listFiles(rootDir) {
  const files = [];
  function walk(dir) {
    for (const dirent of fs.readdirSync(dir, { withFileTypes: true })) {
      const fullPath = path.join(dir, dirent.name);
      if (dirent.isDirectory()) {
        walk(fullPath);
      } else if (dirent.isFile()) {
        files.push(fullPath);
      }
    }
  }
  walk(rootDir);
  return files.sort((a, b) => a.localeCompare(b));
}

function sha256(filePath) {
  const hash = crypto.createHash("sha256");
  hash.update(fs.readFileSync(filePath));
  return hash.digest("hex");
}

function toPosix(value) {
  return value.split(path.sep).join("/");
}

function shellQuote(value) {
  return `'${value.replace(/'/g, "'\\''")}'`;
}

function createRunbook() {
  const archiveName = `${packageName}.tar.gz`;
  const moduleNote = args.includeGoModules
    ? "This package includes offline/gomodproxy."
    : "This package does not include offline/gomodproxy. Extract it over an A-side build directory that already has offline/gomodproxy from a previous --include-go-modules package.";

  return `# A-side offline build

${moduleNote}

## Linux A

\`\`\`bash
mkdir -p /tmp/new-api-offline
tar -xzf ${archiveName} -C /tmp/new-api-offline
cd /tmp/new-api-offline

test -f web/default/dist/index.html
test -f web/classic/dist/index.html
test -d offline/gomodproxy

docker build --pull=false --network=none -f Dockerfile.offline -t new-api-local:offline .
docker compose -f docker-compose.prod-offline.yml up -d
\`\`\`

## Windows Server 2019 A

Run in PowerShell from the directory containing ${archiveName}.

\`\`\`powershell
New-Item -ItemType Directory -Force C:\\new-api-offline | Out-Null
tar -xzf .\\${archiveName} -C C:\\new-api-offline
Set-Location C:\\new-api-offline

Test-Path .\\web\\default\\dist\\index.html
Test-Path .\\web\\classic\\dist\\index.html
Test-Path .\\offline\\gomodproxy

docker build --pull=false --network=none -f Dockerfile.offline -t new-api-local:offline .
docker compose -f docker-compose.prod-offline.yml up -d
\`\`\`

## Required images on A

A must already have the builder, runtime, and production Compose dependency images loaded locally:

\`\`\`text
golang:1.26.1-alpine
new-api-runtime-base:bookworm-slim-amd64
redis:7.2-alpine
postgres:15-alpine
apache/kafka:3.7.1
clickhouse/clickhouse-server:24.8
\`\`\`

If your local image tags differ, pass build args:

\`\`\`bash
docker build --pull=false --network=none \\
  --build-arg GO_BUILDER_IMAGE=your-go-builder:tag \\
  --build-arg RUNTIME_IMAGE=your-runtime:tag \\
  -f Dockerfile.offline \\
  -t new-api-local:offline .
\`\`\`

## When to include Go modules

Use the B-side script with --include-go-modules on first transfer and whenever go.mod or go.sum changes.
For normal Go source or frontend changes, use the default package and extract it over the same A-side build directory.
`;
}
