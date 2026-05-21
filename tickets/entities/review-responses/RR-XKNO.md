---
id: RR-XKNO
type: review-response
title: Walker stack-overflow defence missing for deep recursion
finding: 'An adversarial 10k-deep `((((((x))))))` or `a and a and a and ...` will parse via gopher-lua and walk through the AST visitor recursively. Go stack grows to ~8KB per frame; 10k deep is ~80MB of stack and risks runtime.SIGSEGV before eval ever runs. Step budget covers eval only. Two fixes: (a) rewrite walker iteratively with a work-stack, or (b) add a depth counter that returns CompileError past 256. Pick (b) — simpler, easier to reason about. Also acts as compile-time complexity bound complementing R5. Pin with a fuzz target plus a `TestCompile_RejectsDeeplyNestedExpression` that constructs 1024-paren input and expects a CompileError, not a panic.'
severity: significant
resolution: Compile-time depth budget added (default 256, configurable via CompileOption). Walker returns *CompileError on overflow, not a panic. AC5 + TestCompile_RejectsDeeplyNestedExpression (1024 nested parens) pin this. FuzzCompile also catches the failure mode if it regresses.
status: addressed
---
