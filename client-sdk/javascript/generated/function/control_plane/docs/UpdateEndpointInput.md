
# UpdateEndpointInput

UpdateEndpointInput schema

## Properties

Name | Type
------------ | -------------
`description` | string
`entrypoint` | string
`isPublic` | boolean
`memoryLimitMb` | number
`runtime` | string
`timeoutSeconds` | number

## Example

```typescript
import type { UpdateEndpointInput } from ''

// TODO: Update the object below with actual values
const example = {
  "description": null,
  "entrypoint": null,
  "isPublic": null,
  "memoryLimitMb": null,
  "runtime": null,
  "timeoutSeconds": null,
} satisfies UpdateEndpointInput

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as UpdateEndpointInput
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


