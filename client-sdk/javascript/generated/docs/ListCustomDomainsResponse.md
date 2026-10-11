
# ListCustomDomainsResponse

ListCustomDomainsResponse schema

## Properties

Name | Type
------------ | -------------
`data` | [Array&lt;CustomDomain&gt;](CustomDomain.md)
`total` | number

## Example

```typescript
import type { ListCustomDomainsResponse } from ''

// TODO: Update the object below with actual values
const example = {
  "data": null,
  "total": null,
} satisfies ListCustomDomainsResponse

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as ListCustomDomainsResponse
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


