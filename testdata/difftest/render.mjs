// Reads djot markup from stdin, writes HTML to stdout.
import { createRequire } from "module";
const require = createRequire("/djot-js/");
const { parse, renderHTML } = require("@djot/djot");

let input = "";
for await (const chunk of process.stdin) {
  input += chunk;
}
const doc = parse(input);
process.stdout.write(renderHTML(doc));
