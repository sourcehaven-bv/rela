---
id: RR-ZD5M1V
type: review-response
title: Head can answer read.seq above channel.last_seq
finding: The head takes LastSeq from Head and then runs a separate Reads. An own post that lands in between moves the marker to N+1 while channel.last_seq says N. Fill channel.last_seq from the Reads LastSeq when it is higher, or document that read.seq may exceed last_seq. A client that compares the two should not have to handle it by accident.
severity: nit
resolution: Head takes the higher LastSeq from Reads, so read.seq never exceeds last_seq. TestHead_ReadNotAboveLastSeq.
status: addressed
---
