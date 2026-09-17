---
id: RR-ZH02FY
type: review-response
title: Rename aborts on a PK collision, stranding the whole thread
finding: 'pgcomments.Rename and sqlitecomments.Rename merged into an occupied destination with a plain UPDATE of target_key. If the destination already held a comment with the same id, the UPDATE violated PRIMARY KEY (target_key, id) and aborted, moving NOTHING — while the entity rename that triggered it had already committed. Reproduced against a live PostgreSQL: rename err = duplicate key value violates unique constraint comments_pkey, leaving old=[c1 dup] new=[dup]. The pg.go doc comment asserted the collision was impossible because ids are minted per comment; that reasoning was wrong, since rela permits id reuse and a restored backup, a re-import or a filecomments migration can all produce a shared id. Severity is critical rather than proportional to the odds because entitymanager/alias_hook.go LOGS a failure from EntityRenamed rather than returning it, so the thread ends up under a key nothing resolves to with no error reaching the user and no route left to the rows.'
severity: critical
resolution: Replaced the UPDATE with INSERT ... SELECT ... ON CONFLICT (target_key, id) DO NOTHING followed by a DELETE, both inside a transaction (which is what pgcomments.DBTX.Begin had been declared for but never used; sqlitecomments.DBTX gained BeginTx to match). The destination's existing comment wins on a collision — it is the one a reader may already have seen — and the rest of the thread still moves. Pinned by a new commentstest.RunRenameTests case, verified to fail against the old UPDATE form.
status: addressed
---
