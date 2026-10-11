
# ListEndpointsResponse

ListEndpointsResponse schema

## Properties

Name | Type
------------ | -------------
`count` | number
`endpoints` | [Array&lt;Endpoint&gt;](Endpoint.md)

## Example

```typescript
import type { ListEndpointsResponse } from ''

// TODO: Update the object below with actual values
const example = {
  "count": null,
  "endpoints": null,
} satisfies ListEndpointsResponse

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as ListEndpointsResponse
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


