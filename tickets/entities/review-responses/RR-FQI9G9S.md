---
id: RR-FQI9G9S
type: review-response
title: Provider is called when nothing can be proposed
finding: When the user may write no set property and every relate output has no readable candidate the model was still called.
severity: minor
resolution: runLLM returns an empty proposal without calling the provider in that case; tested.
status: addressed
---
