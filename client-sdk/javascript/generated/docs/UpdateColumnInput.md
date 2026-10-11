
# UpdateColumnInput

UpdateColumnInput schema

## Properties

Name | Type
------------ | -------------
`defaultValue` | string
`isNullable` | boolean
`newName` | string
`newType` | string

## Example

```typescript
import type { UpdateColumnInput } from ''

// TODO: Update the object below with actual values
const example = {
  "defaultValue": null,
  "isNullable": null,
  "newName": null,
  "newType": null,
} satisfies UpdateColumnInput

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as UpdateColumnInput
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


