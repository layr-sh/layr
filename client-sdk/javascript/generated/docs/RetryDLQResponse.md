
# RetryDLQResponse

RetryDLQResponse schema

## Properties

Name | Type
------------ | -------------
`executionId` | any
`message` | string
`status` | string

## Example

```typescript
import type { RetryDLQResponse } from ''

// TODO: Update the object below with actual values
const example = {
  "executionId": null,
  "message": null,
  "status": null,
} satisfies RetryDLQResponse

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as RetryDLQResponse
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


