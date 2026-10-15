
# UpdateCustomDomainInput

UpdateCustomDomainInput schema

## Properties

Name | Type
------------ | -------------
`tlsAcmeEmail` | string
`tlsEnabled` | boolean

## Example

```typescript
import type { UpdateCustomDomainInput } from ''

// TODO: Update the object below with actual values
const example = {
  "tlsAcmeEmail": null,
  "tlsEnabled": null,
} satisfies UpdateCustomDomainInput

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as UpdateCustomDomainInput
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


