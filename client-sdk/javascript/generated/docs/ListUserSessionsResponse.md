
# ListUserSessionsResponse

ListUserSessionsResponse schema

## Properties

Name | Type
------------ | -------------
`count` | number
`sessions` | [Array&lt;Session&gt;](Session.md)

## Example

```typescript
import type { ListUserSessionsResponse } from ''

// TODO: Update the object below with actual values
const example = {
  "count": null,
  "sessions": null,
} satisfies ListUserSessionsResponse

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as ListUserSessionsResponse
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


