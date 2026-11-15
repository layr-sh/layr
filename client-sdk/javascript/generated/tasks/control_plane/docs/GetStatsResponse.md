
# GetStatsResponse

GetStatsResponse schema

## Properties

Name | Type
------------ | -------------
`activeJobs` | number
`completedExecutions` | number
`failedExecutions` | number
`pausedJobs` | number
`pendingExecutions` | number
`runningExecutions` | number
`successRate` | number
`totalJobs` | number

## Example

```typescript
import type { GetStatsResponse } from ''

// TODO: Update the object below with actual values
const example = {
  "activeJobs": null,
  "completedExecutions": null,
  "failedExecutions": null,
  "pausedJobs": null,
  "pendingExecutions": null,
  "runningExecutions": null,
  "successRate": null,
  "totalJobs": null,
} satisfies GetStatsResponse

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as GetStatsResponse
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


