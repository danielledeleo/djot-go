// Reads djot markup from stdin, writes HTML to stdout.
import { createRequire } from "module";
const require = createRequire(import.meta.url);
const { parse, renderHTML } = require("/djot-js");

let input = "";
for await (const chunk of process.stdin) {
  input += chunk;
}
const doc = parse(input);
process.stdout.write(renderHTML(doc));
