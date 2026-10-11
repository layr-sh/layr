
# ListEventsResponse

ListEventsResponse schema

## Properties

Name | Type
------------ | -------------
`count` | number
`data` | [Array&lt;Event&gt;](Event.md)
`limit` | number
`offset` | number
`total` | number

## Example

```typescript
import type { ListEventsResponse } from ''

// TODO: Update the object below with actual values
const example = {
  "count": null,
  "data": null,
  "limit": null,
  "offset": null,
  "total": null,
} satisfies ListEventsResponse

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as ListEventsResponse
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


