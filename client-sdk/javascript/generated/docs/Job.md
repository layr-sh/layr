
# Job

Job schema

## Properties

Name | Type
------------ | -------------
`createdAt` | Date
`cronExpression` | string
`id` | any
`isEnabled` | boolean
`lastRunAt` | Date
`lastUpdatedAt` | Date
`name` | string
`nextRunAt` | Date
`targetPayload` | object
`targetType` | string
`timezone` | string

## Example

```typescript
import type { Job } from ''

// TODO: Update the object below with actual values
const example = {
  "createdAt": null,
  "cronExpression": null,
  "id": null,
  "isEnabled": null,
  "lastRunAt": null,
  "lastUpdatedAt": null,
  "name": null,
  "nextRunAt": null,
  "targetPayload": null,
  "targetType": null,
  "timezone": null,
} satisfies Job

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as Job
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


