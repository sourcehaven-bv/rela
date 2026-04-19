---
id: RR-OKJUF
type: review-response
title: Mode arg duplicates source of truth, invites drift
finding: 'Authoritative signal ''is this repo encrypted?'' is os.Stat(.rela/encryption.yaml) (factory.go:loadCrypto). Threading a Mode enum into the verifier means two declarations of the same fact. They can disagree: a future bug constructs encrypted decorator but passes Cleartext to verifier, half-migrated-repo guard silently disarmed. Current isCleartextMode(c Crypto) cannot drift because it asks the actual object that performs the I/O.'
severity: significant
resolution: Factory computes wantSealed := cfgExists once and uses it in the same if branch that installs EncryptedFS. Single source of truth enforced in one place. AC#9 asserts via test that no code path constructs EncryptedFS without passing wantSealed=true to the verifier (and vice versa).
status: addressed
---
