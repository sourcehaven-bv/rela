---
id: RR-6ZDIPKU
type: review-response
title: Faced subjects get two different ids
finding: A hidden subject's stub used the address while a read one used the bare id; firstTasks compared Keep with the address; {id} was filled with the bare id, so a declared link could open another face.
severity: minor
resolution: The stub and firstTasks use the parsed id; FlowTask.Address fills {id}, so the link keeps the run's face.
status: addressed
---
