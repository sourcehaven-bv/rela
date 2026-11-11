---
id: RR-M0GUA3S
type: review-response
title: Conversion bypasses the rename guard on a held subject
finding: 'pr-guidance: Converter.Convert moves the subject to a new id and type without asking the rename guard, so a conversion could drop a subject out from under an open flow run on every surface.'
severity: significant
resolution: Convert asks checkRenameGuard inside its store transaction before it moves the id, and Preview asks it too. Both fail closed with ErrRenameBlocked; data-entry answers 409 rename_blocked. Pinned by TestRenameGuard_Conversion, TestOutsideEdits_ConversionRefusedWhileHeld and TestWriteConversionError_RenameBlocked.
status: addressed
---
