import * as fs from "fs";
import * as path from "path";
import type { BrowserContext, TestInfo } from "@playwright/test";

// SPA-to-backend call tracing (tools/seqtrace/README.md, "Tracing the SPA").
// When RELA_E2E_SEQTRACE names a directory, serverBinary is a seqtrace build:
// each spawned server writes its call trace there, and each test's browser
// records the stack of every /api/ request it sends. `seqtrace stitch` joins
// the two by the span id the recorder puts in a request header.
const DIR = process.env.RELA_E2E_SEQTRACE
  ? path.resolve(process.env.RELA_E2E_SEQTRACE)
  : "";

const RECORDER = path.resolve(
  __dirname,
  "../../tools/seqtrace/browser/recorder.js",
);

let servers = 0;

/** The environment that makes a seqtrace build write its trace, or {} when
 *  tracing is off. Each server gets its own file, with a sidecar recording
 *  whether projectDir has an acl.yaml: a trace from a server without one says
 *  nothing about authorization, and `seqtrace stitch` marks it so. */
export function seqtraceServerEnv(projectDir: string): Record<string, string> {
  if (!DIR) return {};
  fs.mkdirSync(DIR, { recursive: true });
  servers++;
  const base = path.join(DIR, `server-${process.pid}-${servers}`);
  // A snapshot at spawn: a spec that removes acl.yaml later is not seen.
  const acl = fs.existsSync(path.join(projectDir, "acl.yaml"));
  fs.writeFileSync(`${base}.json`, JSON.stringify({ acl }));
  return {
    SEQTRACE_OUT: `${base}.jsonl`,
    SEQTRACE_ROOT: "internal/dataentry\\.requestStats\\.",
  };
}

/** The requests a test's browser sent, in the order it sent them. */
export type SeqtraceRecords = unknown[];

/** Injects the browser recorder into every page of context and returns the
 *  list its records arrive in. The recorder pushes each record through a
 *  binding as it is made, so records survive navigations and cover every
 *  page of the context. */
export async function installSeqtraceRecorder(
  context: BrowserContext,
): Promise<SeqtraceRecords> {
  const records: SeqtraceRecords = [];
  if (!DIR) return records;
  await context.exposeBinding("__relaSeqtracePush", (_source, record: unknown) => {
    records.push(record);
  });
  await context.addInitScript({ path: RECORDER });
  return records;
}

/** Writes the records of one test under the test's title. */
export function saveSeqtraceRecords(
  records: SeqtraceRecords,
  testInfo: TestInfo,
): void {
  if (!DIR || records.length === 0) return;
  const title = testInfo.titlePath.slice(1).join(" › ");
  const file = path.join(
    DIR,
    `client-${testInfo.testId}-${testInfo.retry}.json`,
  );
  fs.writeFileSync(file, JSON.stringify({ title, records }));
}
