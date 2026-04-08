---
id: RR-RA9I
type: review-response
title: 'F4: Response body size cap returns ErrNetwork instead of ErrBadResponse'
finding: readLimitedBody returned a plain error when the body exceeded maxResponseBytes. That flowed through wrapNetworkError and got classified as ErrNetwork. A 12 MiB upstream response is an upstream protocol violation, not a network failure; classifying it as network misleads automated retry logic (retrying a misbehaving upstream is pointless) and the test even accepted *either* classification, which was a smell.
severity: significant
resolution: 'Introduced an errBodyTooLarge sentinel in openai.go. Chat() now branches on errors.Is(readErr, errBodyTooLarge) and constructs a typed *Error{Kind: ErrBadResponse, Status: resp.StatusCode, Message: ...} directly. TestProvider_Chat_ResponseTooLarge tightened to assert exactly ErrBadResponse with the size-limit message in the error string.'
status: addressed
---
