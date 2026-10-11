
# ListBucketsResponse

ListBucketsResponse schema

## Properties

Name | Type
------------ | -------------
`buckets` | [Array&lt;Bucket&gt;](Bucket.md)
`count` | number

## Example

```typescript
import type { ListBucketsResponse } from ''

// TODO: Update the object below with actual values
const example = {
  "buckets": null,
  "count": null,
} satisfies ListBucketsResponse

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as ListBucketsResponse
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


