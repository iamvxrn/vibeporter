#!/usr/bin/env node
"use strict";

const { spawnSync } = require("child_process");
const {
  chmodSync,
  copyFileSync,
  createWriteStream,
  existsSync,
  mkdirSync,
  rmSync,
  unlinkSync,
} = require("fs");
const { homedir } = require("os");
const { join } = require("path");
const https = require("https");
const { pipeline } = require("stream/promises");

const VERSION = require("./package.json").version;
const REPO = "iamvxrn/vibeporter";

function cacheRoot() {
  if (process.env.XDG_CACHE_HOME) {
    return join(process.env.XDG_CACHE_HOME, "vibeporter");
  }
  if (process.platform === "win32") {
    return join(process.env.LOCALAPPDATA || join(homedir(), "AppData", "Local"), "vibeporter");
  }
  if (process.platform === "darwin") {
    return join(homedir(), "Library", "Caches", "vibeporter");
  }
  return join(homedir(), ".cache", "vibeporter");
}

function asset() {
  const osMap = { linux: "linux", darwin: "darwin", win32: "windows" };
  const archMap = { x64: "amd64", arm64: "arm64" };
  const os = osMap[process.platform];
  const goarch = archMap[process.arch];
  if (!os || !goarch) {
    throw new Error(`unsupported platform ${process.platform}/${process.arch}`);
  }
  if (os === "windows" && goarch === "arm64") {
    throw new Error("no Windows arm64 GitHub Release yet; use linux/darwin or windows amd64");
  }
  const ext = os === "windows" ? "zip" : "tar.gz";
  return {
    name: `vibeporter_${os}_${goarch}.${ext}`,
    ext,
    bin: os === "windows" ? "vibeporter.exe" : "vibeporter",
  };
}

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const go = (u, hops) => {
      if (hops > 8) {
        reject(new Error("too many redirects"));
        return;
      }
      https
        .get(u, { headers: { "User-Agent": "vibeporter-npm" } }, (res) => {
          if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
            res.resume();
            go(res.headers.location, hops + 1);
            return;
          }
          if (res.statusCode !== 200) {
            res.resume();
            reject(new Error(`download failed (${res.statusCode}): ${u}`));
            return;
          }
          const out = createWriteStream(dest);
          pipeline(res, out).then(resolve).catch(reject);
        })
        .on("error", reject);
    };
    go(url, 0);
  });
}

function psQuote(value) {
  return `'${String(value).replace(/'/g, "''")}'`;
}

function extract(archive, dest, ext) {
  mkdirSync(dest, { recursive: true });
  if (ext === "zip") {
    const tar = spawnSync("tar", ["-xf", archive, "-C", dest], { encoding: "utf8" });
    if (tar.status === 0) {
      return;
    }
    const ps = spawnSync(
      "powershell.exe",
      [
        "-NoProfile",
        "-NonInteractive",
        "-Command",
        `Expand-Archive -LiteralPath ${psQuote(archive)} -DestinationPath ${psQuote(dest)} -Force`,
      ],
      { encoding: "utf8" },
    );
    if (ps.status !== 0) {
      throw new Error((ps.stderr || tar.stderr || "failed to extract zip").trim());
    }
    return;
  }
  const r = spawnSync("tar", ["-xzf", archive, "-C", dest], { encoding: "utf8" });
  if (r.status !== 0) {
    throw new Error((r.stderr || r.stdout || "failed to extract archive").trim());
  }
}

async function ensureBinary() {
  const { name, ext, bin } = asset();
  const dir = join(cacheRoot(), VERSION);
  mkdirSync(dir, { recursive: true });
  const binPath = join(dir, bin);
  if (existsSync(binPath)) {
    return binPath;
  }

  const url = `https://github.com/${REPO}/releases/download/v${VERSION}/${name}`;
  const archive = join(dir, name);
  process.stderr.write(`Downloading vibeporter v${VERSION} from GitHub Releases...\n`);
  await download(url, archive);

  const extractDir = join(dir, "extract");
  rmSync(extractDir, { recursive: true, force: true });
  extract(archive, extractDir, ext);

  const extracted = join(extractDir, bin);
  if (!existsSync(extracted)) {
    throw new Error(`extracted binary missing: ${extracted}`);
  }
  copyFileSync(extracted, binPath);
  chmodSync(binPath, 0o755);
  unlinkSync(archive);
  rmSync(extractDir, { recursive: true, force: true });
  return binPath;
}

(async () => {
  const bin = await ensureBinary();
  const result = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
  if (result.error) {
    throw result.error;
  }
  process.exit(result.status === null ? 1 : result.status);
})().catch((err) => {
  console.error(err.message || err);
  process.exit(1);
});
