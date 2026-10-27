// Hand-written CRC-32 for JavaScript (hashbench-2026-10-27): node crc32.mjs FILE
import { readFileSync } from "node:fs";
const b = readFileSync(process.argv[2]);
const tab = new Int32Array(256);
for (let i = 0; i < 256; i++) {
  let c = i;
  for (let j = 0; j < 8; j++) c = c & 1 ? 0xEDB88320 ^ (c >>> 1) : c >>> 1;
  tab[i] = c;
}
let crc = -1;
for (let i = 0; i < b.length; i++) crc = tab[(crc ^ b[i]) & 0xff] ^ (crc >>> 8);
console.log(((~crc) >>> 0).toString(16).padStart(8, "0"));
