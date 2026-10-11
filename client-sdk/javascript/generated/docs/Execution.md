
# Execution


## Properties

Name | Type
------------ | -------------
`createdAt` | Date
`deploymentId` | any
`durationMs` | number
`endpointId` | any
`errorMessage` | string
`id` | any
`method` | string
`path` | string
`statusCode` | number
`stderr` | string
`stdout` | string

## Example

```typescript
import type { Execution } from ''

// TODO: Update the object below with actual values
const example = {
  "createdAt": null,
  "deploymentId": null,
  "durationMs": null,
  "endpointId": null,
  "errorMessage": null,
  "id": null,
  "method": null,
  "path": null,
  "statusCode": null,
  "stderr": null,
  "stdout": null,
} satisfies Execution

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as Execution
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


