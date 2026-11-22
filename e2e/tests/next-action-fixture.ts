import fs from "node:fs";
import path from "node:path";
import { test } from "./fixtures";

/**
 * A test whose project adds `block` to data-entry.yaml, just above
 * `navigation:`. Specs use it to configure next actions.
 */
export function withDataEntryBlock(block: string) {
  return test.extend({
    testProject: async ({ testProject }, use) => {
      const file = path.join(testProject, "data-entry.yaml");
      const yaml = fs.readFileSync(file, "utf8");
      if (!yaml.includes("\nnavigation:\n"))
        throw new Error("fixture YAML has no navigation:");
      fs.writeFileSync(
        file,
        yaml.replace("\nnavigation:\n", `\n${block}\nnavigation:\n`),
      );
      await use(testProject);
    },
  });
}

/**
 * A test whose project's data-entry.yaml has `from` replaced by `to`. The
 * fixture text must contain `from`, so a fixture change cannot silently
 * leave the test on the default config.
 */
export function withDataEntryEdit(from: string, to: string) {
  return test.extend({
    testProject: async ({ testProject }, use) => {
      const file = path.join(testProject, "data-entry.yaml");
      const yaml = fs.readFileSync(file, "utf8");
      if (!yaml.includes(from))
        throw new Error(`fixture YAML has no ${JSON.stringify(from)}`);
      fs.writeFileSync(file, yaml.replace(from, to));
      await use(testProject);
    },
  });
}
