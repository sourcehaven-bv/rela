---
id: RR-OW7CUZ
type: review-response
title: Startup warning misses attachment transforms and document commands, and fires when unconfined
finding: warnIfNoSandboxReadPaths counts only HasConfiguredScan and meta.Transforms; it ignores attachment transform steps with cmd (walked by probeAttachmentCommands) and document commands, and still warns under RELA_UNCONFINED_COMMANDS=1 where nothing restricts reads.
severity: minor
resolution: warnIfNoSandboxReadPaths now counts attachment scans a property uses, attachment transform steps, export transforms and command-rendered documents (cfg.Documents), and stays quiet when RELA_UNCONFINED_COMMANDS is set (new cmdexec.UnconfinedByDefault). probeAttachmentCommands shares the attachmentCommands walk. TestWarnIfNoSandboxReadPaths covers each source plus the unconfined case.
status: addressed
---
