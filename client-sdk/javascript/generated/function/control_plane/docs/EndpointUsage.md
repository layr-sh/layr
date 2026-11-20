
# EndpointUsage


## Properties

Name | Type
------------ | -------------
`averageDurationMs` | number
`executions` | number
`failedExecutions` | number
`isDeployed` | boolean
`isPublic` | boolean
`lastExecutedAt` | Date
`name` | string
`p95DurationMs` | number
`runtime` | string

## Example

```typescript
import type { EndpointUsage } from ''

// TODO: Update the object below with actual values
const example = {
  "averageDurationMs": null,
  "executions": null,
  "failedExecutions": null,
  "isDeployed": null,
  "isPublic": null,
  "lastExecutedAt": null,
  "name": null,
  "p95DurationMs": null,
  "runtime": null,
} satisfies EndpointUsage

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as EndpointUsage
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


