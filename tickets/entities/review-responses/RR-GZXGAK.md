---
id: RR-GZXGAK
type: review-response
title: Timestamps round-trip in the server's local zone on PostgreSQL, diverging from SQLite
finding: 'pgx returns a TIMESTAMPTZ in time.Local while sqlitecomments returns UTC, so the same comment came back in different zones from the two backends. Comment marshals to JSON carrying whatever offset it holds, so one comment serialized as "+01:00" from a postgres node and "Z" from a sqlite one, with the offset shifting under the server''s DST — a difference clients see even though both describe the same instant. The conformance suite could not catch it: it asserted CreatedAt with .Equal(), which compares instants and ignores location entirely. PostgreSQL also truncates to microseconds where SQLite keeps nanoseconds, though Service.Add stamps UTC going in so the precision loss is capped below a microsecond in practice. Latent rather than live, but exactly the class of file-vs-database divergence commentstest exists to catch.'
severity: significant
resolution: pgcomments.scanComment normalizes both CreatedAt and UpdatedAt to UTC on read. Added a require.Equal(time.UTC, ...Location()) assertion to commentstest.RunRoundTripTests so the suite can now see the difference .Equal() hides; verified it fails without the normalization.
status: addressed
---
