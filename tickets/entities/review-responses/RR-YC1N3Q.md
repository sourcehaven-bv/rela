---
id: RR-YC1N3Q
type: review-response
title: Aliasing test in TestDescribe_In is vacuous
finding: 'The plan checks that the returned In does not alias the config by appending to the returned slice and reading again. append never writes through to an element the config''s slice header can see: with len==cap it reallocates, and with spare capacity it writes past the config''s len. The test passes with or without slices.Clone. Mutate an element instead (d.In[0] = "x"), then Describe and Read again and assert the original value, for both MessageTypeDescription and MessageTypeView.'
severity: minor
resolution: "addressed\tTestDescribe_In now sets In[0] on both the MessageTypeDescription and the MessageTypeView, then calls Describe and Read again and asserts the original value; append is no longer used."
status: addressed
---
