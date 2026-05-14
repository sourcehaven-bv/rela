---
id: RR-EBGN
type: review-response
title: Helper test mock does not exercise CodeMirror replaceSelection semantics — false confidence
finding: |-
    `frontend/src/components/forms/insertEntityRef.test.ts` is 28 cases of duck-typed `makeEditor` calls. The 'replaces the current selection (CodeMirror replaceSelection semantic)' test is misleading — the mock has no notion of selection. The test names a behavior it does not actually verify:

    ```
    it('replaces the current selection (CodeMirror replaceSelection semantic)', () => {
      // replaceSelection is the same call whether the selection is empty or
      // not, so the same assertion holds — verify the contract isn't bypassed.
      const id = 'FEAT-010'
      const { editor, replaceSelection } = makeEditor()
      insertEntityRef(editor, id)
      expect(replaceSelection).toHaveBeenCalledExactlyOnceWith(`\`${id}\``, 'end')
    })
    ```

    This is identical to the 'happy path' test — same assertion, no selection state, just a comment saying 'verify the contract isn't bypassed'. It buys zero coverage over the previous case. Either delete it or rewrite it to seed a non-empty selection and assert adjacency padding is computed from the selection BOUNDS, which is the actual contract being claimed and the bug surface flagged in RR-A4RR.

    The broader issue: real EasyMDE/CodeMirror has methods this test doesn't stub (`getCursor('from'/'to')`, `listSelections`, `somethingSelected`, `getRange` across lines, the `pos.outside` flag returned at buffer boundaries). The duck-typed mock will let any helper implementation that satisfies the same four-method shape pass — including a buggy one. Recommend at least ONE integration-style test that mounts a real EasyMDE in JSDOM and asserts insertion at a multi-line selection, so adjacency and replaceSelection semantics are exercised end-to-end.

    Low severity because the helper logic is genuinely simple; high informational value because the test count is misleadingly high for the assurance delivered.
severity: minor
resolution: Removed the redundant 'replaces the current selection' test case. Selection-bound behavior is now covered by the three new RR-A4RR-driven tests under 'adjacency padding' that pass actual from/to bounds, asserting against the real contract.
status: addressed
---
