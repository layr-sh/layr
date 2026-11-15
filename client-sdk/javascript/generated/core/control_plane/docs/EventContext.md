
# EventContext


## Properties

Name | Type
------------ | -------------
`ipAddress` | string
`requestId` | string
`userAgent` | string

## Example

```typescript
import type { EventContext } from ''

// TODO: Update the object below with actual values
const example = {
  "ipAddress": null,
  "requestId": null,
  "userAgent": null,
} satisfies EventContext

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as EventContext
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


