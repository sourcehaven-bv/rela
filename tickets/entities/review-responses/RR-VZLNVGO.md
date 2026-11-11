---
id: RR-VZLNVGO
type: review-response
title: Reopen on Remove cannot be config-free and is wrong with two answers
finding: Remove erases the type and runs no session step, so the store cannot tell an answer without config; with two answers, removing one reopens a still-answered question.
severity: significant
resolution: 'Plan: store asked as map[questionSeq]{AnswerType; Answers int} on the channel. Realtime passes NewMessage.Asks (answer type) per question. An append whose reply_to is in asked with matching type adds one; Remove subtracts one using the type read before erasing. open_questions counts entries at zero. No Close hook, no generation check.'
status: addressed
---
