import fs from "node:fs/promises";
import { fileURLToPath } from "node:url";
import sharp from "sharp";

const iconSource = fileURLToPath(new URL("../public/komari-retro-mark.svg", import.meta.url));
const publicDir = fileURLToPath(new URL("../public/", import.meta.url));
const assetsDir = fileURLToPath(new URL("../public/assets/", import.meta.url));

await fs.mkdir(assetsDir, { recursive: true });

const renderPng = (size) =>
  sharp(iconSource).resize(size, size).png().toBuffer();

await Promise.all([
  renderPng(192).then((png) => fs.writeFile(`${assetsDir}/komari-retro-192.png`, png)),
  renderPng(512).then((png) => fs.writeFile(`${assetsDir}/komari-retro-512.png`, png)),
]);

const faviconPng = await renderPng(48);
const icoHeader = Buffer.alloc(22);
icoHeader.writeUInt16LE(0, 0); // Reserved.
icoHeader.writeUInt16LE(1, 2); // ICO image type.
icoHeader.writeUInt16LE(1, 4); // One PNG-backed image.
icoHeader.writeUInt8(48, 6);
icoHeader.writeUInt8(48, 7);
icoHeader.writeUInt8(0, 8); // No palette.
icoHeader.writeUInt8(0, 9); // Reserved.
icoHeader.writeUInt16LE(1, 10);
icoHeader.writeUInt16LE(32, 12);
icoHeader.writeUInt32LE(faviconPng.length, 14);
icoHeader.writeUInt32LE(icoHeader.length, 18);

await fs.writeFile(`${publicDir}/favicon.ico`, Buffer.concat([icoHeader, faviconPng]));
