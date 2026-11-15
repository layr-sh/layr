
# ExecutionLog


## Properties

Name | Type
------------ | -------------
`errorMessage` | string
`executedAt` | Date
`executionDurationMs` | number
`executionId` | any
`id` | any
`jobId` | any
`responseBody` | string
`responseStatusCode` | number
`status` | string

## Example

```typescript
import type { ExecutionLog } from ''

// TODO: Update the object below with actual values
const example = {
  "errorMessage": null,
  "executedAt": null,
  "executionDurationMs": null,
  "executionId": null,
  "id": null,
  "jobId": null,
  "responseBody": null,
  "responseStatusCode": null,
  "status": null,
} satisfies ExecutionLog

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as ExecutionLog
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


