---
id: RR-GK2Z
type: review-response
title: 'F5: Missing choices[0].message silently swallowed as empty content'
finding: chatResponseWire.Choices[i].Message was a value type. If the upstream returned {choices:[{finish_reason:stop}]} with no message, Message was zero-valued, Content was a nil json.RawMessage, and decodeContent(nil) returned ('', nil). The caller got a successful *ChatResponse with Content=='' and no error at all. Scripts checking err==nil would happily use the empty string.
severity: significant
resolution: 'Made choiceWire.Message a *messageRawWire pointer so absence is detectable. decodeChatResponse now rejects parsed.Choices[0].Message == nil with &Error{Kind: ErrBadResponse, Message: ''upstream returned choice with no message''}. New TestProvider_Chat_ChoiceWithoutMessage covers the path.'
status: addressed
---
