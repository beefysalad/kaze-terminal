// Export production platform icons from the canonical SVGs. No font dependency.
const fs = require('node:fs');
const path = require('node:path');
const { Resvg } = require('@resvg/resvg-js');
const root = path.resolve(__dirname, '../..');
const branding = path.join(root, 'assets/branding');
const render = (svg, width) => new Resvg(svg, { fitTo: { mode: 'width', value: width } }).render().asPng();
const icon = fs.readFileSync(path.join(branding, 'kaze-icon.svg'));
const mac = fs.readFileSync(path.join(branding, 'kaze-app-icon-macos.svg'));
fs.writeFileSync(path.join(branding, 'kaze-icon-512.png'), render(icon, 512));
const entries = { icp4:16, icp5:32, icp6:64, ic07:128, ic08:256, ic09:512, ic10:1024,
  ic11:32, ic12:64, ic13:256, ic14:512 };
const chunks = Object.entries(entries).map(([type, width]) => {
  const data = render(mac, width);
  const header = Buffer.alloc(8);
  header.write(type); header.writeUInt32BE(data.length + 8, 4);
  return Buffer.concat([header, data]);
});
const header = Buffer.alloc(8);
header.write('icns'); header.writeUInt32BE(8 + chunks.reduce((n, c) => n + c.length, 0), 4);
fs.writeFileSync(path.join(branding, 'kaze.icns'), Buffer.concat([header, ...chunks]));
for (const file of ['kaze-icon.svg', 'kaze-lockup.svg']) {
  fs.copyFileSync(path.join(branding, file), path.join(root, 'docs/site-assets', file));
}
console.log('Exported Windows PNG, macOS ICNS and website SVG copies.');
