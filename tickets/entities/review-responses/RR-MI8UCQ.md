---
id: RR-MI8UCQ
type: review-response
title: Create candidate has no face
finding: 'write_handler.go create and dry-run build the candidate without a Face, so CreateFieldVerdicts asks create on the unnamed face and the narrowing returns on faced types: the form says writable and POST answers 403.'
severity: significant
resolution: 'POST /api/v1/<type> now builds the create candidate with Face: createOpts.Face; the dry-run candidate already carried the face via ValidateCreate. TestCreateFacedRow_JudgesFieldsOnTheCreatedFace posts a draft policy to both and fails (403) without the face.'
status: addressed
---
